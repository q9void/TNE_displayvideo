// Package middleware provides HTTP middleware for PBS
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// PublisherAuthConfig holds publisher authentication configuration
type PublisherAuthConfig struct {
	Enabled           bool              // Enable publisher validation
	AllowUnregistered bool              // Allow requests without publisher ID (for testing)
	RegisteredPubs    map[string]string // publisher_id -> allowed domains (comma-separated, empty = any)
	ValidateDomain    bool              // Validate request domain matches registered domains
	RateLimitPerPub   int               // Requests per second per publisher (0 = unlimited)
	UseRedis          bool              // Use Redis for publisher validation
}

// DefaultPublisherAuthConfig returns default config
// SECURITY: Publisher auth is ENABLED by default in production mode
// Set PUBLISHER_AUTH_ENABLED=false explicitly to disable (development only)
func DefaultPublisherAuthConfig() *PublisherAuthConfig {
	// Production-secure default: enabled unless explicitly disabled
	enabled := os.Getenv("PUBLISHER_AUTH_ENABLED") != "false"

	allowUnregistered := os.Getenv("PUBLISHER_ALLOW_UNREGISTERED") == "true"

	return &PublisherAuthConfig{
		Enabled:           enabled,
		AllowUnregistered: allowUnregistered,
		RegisteredPubs:    parsePublishers(os.Getenv("REGISTERED_PUBLISHERS")),
		ValidateDomain:    os.Getenv("PUBLISHER_VALIDATE_DOMAIN") == "true",
		RateLimitPerPub:   100, // Default 100 RPS per publisher
		UseRedis:          os.Getenv("PUBLISHER_AUTH_USE_REDIS") != "false",
	}
}

// parsePublishers parses "pub1:domain1.com,pub2:domain2.com" format
func parsePublishers(envValue string) map[string]string {
	pubs := make(map[string]string)
	if envValue == "" {
		return pubs
	}

	pairs := strings.Split(envValue, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			pubs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 && parts[0] != "" {
			// Publisher without domain restriction
			pubs[strings.TrimSpace(parts[0])] = ""
		}
	}
	return pubs
}

// minimalBidRequest is a minimal struct for extracting publisher info
type minimalBidRequest struct {
	Site *struct {
		Domain    string `json:"domain"`
		Publisher *struct {
			ID string `json:"id"`
		} `json:"publisher"`
	} `json:"site"`
	App *struct {
		Bundle    string `json:"bundle"`
		Publisher *struct {
			ID string `json:"id"`
		} `json:"publisher"`
	} `json:"app"`
}

// RedisClient interface for Redis operations
type RedisClient interface {
	HGet(ctx context.Context, key, field string) (string, error)
	Ping(ctx context.Context) error
}

// PublisherStore interface for database operations
type PublisherStore interface {
	GetByPublisherID(ctx context.Context, publisherID string) (publisher interface{}, err error)
}

// PublisherAuth provides publisher authentication for auction endpoints
//
// LOCK ORDERING: To prevent deadlocks, locks MUST be acquired in this order:
//   1. mu (config lock) - protects config, redisClient, publisherStore
//   2. publisherCacheMu - protects publisherCache
//   3. rateLimitsMu - protects rateLimits
//
// RULES:
//   - Never acquire locks in reverse order
//   - Release locks as soon as possible (use RLock when possible)
//   - Never hold multiple locks across I/O operations (Redis, PostgreSQL)
//   - Document any method that acquires multiple locks
//
// Example correct ordering:
//   mu.RLock()
//   config := p.config
//   mu.RUnlock()
//   // Now safe to take other locks without holding mu
//   publisherCacheMu.Lock()
//   // ... work ...
//   publisherCacheMu.Unlock()
type PublisherAuth struct {
	config         *PublisherAuthConfig
	redisClient    RedisClient
	publisherStore PublisherStore
	mu             sync.RWMutex // Level 1: Config/client access

	// Rate limiting per publisher
	rateLimits   map[string]*rateLimitEntry
	rateLimitsMu sync.RWMutex // Level 3: Rate limit state

	// In-memory fallback cache (for Redis/PostgreSQL failures)
	publisherCache   map[string]*publisherCacheEntry
	publisherCacheMu sync.RWMutex // Level 2: Publisher cache

	// IVT detection
	ivtDetector *IVTDetector
}

type rateLimitEntry struct {
	tokens    float64
	lastCheck time.Time
}

// publisherCacheEntry represents a cached publisher record
type publisherCacheEntry struct {
	allowedDomains string
	expiresAt      time.Time
}

// Redis key for registered publishers
const RedisPublishersHash = "tne_catalyst:publishers" // hash: publisher_id -> allowed_domains

// maxRequestBodySize limits request body reads to prevent OOM attacks (1MB)
const maxRequestBodySize = 1024 * 1024

// Context key for storing publisher objects
const publisherContextKey = "publisher"

// Context key for storing publisher ID
const publisherIDKey = "publisher_id"

// NewPublisherAuth creates a new publisher auth middleware
func NewPublisherAuth(config *PublisherAuthConfig) *PublisherAuth {
	if config == nil {
		config = DefaultPublisherAuthConfig()
	}
	return &PublisherAuth{
		config:      config,
		rateLimits:  make(map[string]*rateLimitEntry),
		ivtDetector: NewIVTDetector(DefaultIVTConfig()),
	}
}

// SetRedisClient sets the Redis client for publisher validation
func (p *PublisherAuth) SetRedisClient(client RedisClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.redisClient = client
}

// SetPublisherStore sets the PostgreSQL publisher store
func (p *PublisherAuth) SetPublisherStore(store PublisherStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publisherStore = store
}

// Middleware returns the publisher authentication middleware handler
func (p *PublisherAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.RLock()
		enabled := p.config.Enabled
		p.mu.RUnlock()

		// Skip if disabled
		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Only apply to POST requests to auction endpoints
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/openrtb2/auction") {
			next.ServeHTTP(w, r)
			return
		}

		// Read and buffer the body so it can be re-read by the handler
		// Use LimitReader to prevent OOM from oversized requests
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize))
		r.Body.Close()
		if err != nil {
			http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
			return
		}

		// Parse minimal request to extract publisher info
		var minReq minimalBidRequest
		if err := json.Unmarshal(body, &minReq); err != nil {
			// Let the main handler deal with invalid JSON
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Extract publisher ID
		publisherID, domain := p.extractPublisherInfo(&minReq)

		// Validate publisher
		if err := p.validatePublisher(r.Context(), publisherID, domain); err != nil {
			log.Warn().
				Str("publisher_id", publisherID).
				Str("domain", domain).
				Str("error", err.Error()).
				Msg("Publisher validation failed")
			// Use json.NewEncoder to prevent JSON injection
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// IVT detection (Invalid Traffic)
		if p.ivtDetector != nil {
			ivtResult := p.ivtDetector.Validate(r.Context(), r, publisherID, domain)

			// Log IVT detection
			if !ivtResult.IsValid {
				// GDPR FIX: Anonymize IP and truncate UA before logging to prevent PII leakage
				log.Warn().
					Str("publisher_id", publisherID).
					Str("domain", domain).
					Str("ip", AnonymizeIPForLogging(ivtResult.IPAddress)).
					Str("ua", AnonymizeUserAgentForLogging(ivtResult.UserAgent)).
					Int("ivt_score", ivtResult.Score).
					Int("signal_count", len(ivtResult.Signals)).
					Bool("blocked", ivtResult.ShouldBlock).
					Msg("IVT detected")
			}

			// Block if IVT score is high and blocking is enabled
			if ivtResult.ShouldBlock {
				log.Warn().
					Str("publisher_id", publisherID).
					Str("reason", ivtResult.BlockReason).
					Int("score", ivtResult.Score).
					Msg("Request blocked - IVT detected")
				http.Error(w, `{"error":"invalid traffic detected"}`, http.StatusForbidden)
				return
			}

			// Add IVT score to headers for monitoring (even if not blocking)
			r.Header.Set("X-IVT-Score", strconv.Itoa(ivtResult.Score))
			if len(ivtResult.Signals) > 0 {
				r.Header.Set("X-IVT-Signals", strconv.Itoa(len(ivtResult.Signals)))
			}
		}

		// Apply rate limiting per publisher
		if publisherID != "" && !p.checkRateLimit(publisherID) {
			log.Warn().
				Str("publisher_id", publisherID).
				Msg("Publisher rate limit exceeded")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		// Add publisher ID to request context (secure - can't be spoofed by client)
		ctx := r.Context()
		ctx = context.WithValue(ctx, publisherIDKey, publisherID)

		// Retrieve and store full publisher object in context for downstream use
		if publisherID != "" && p.publisherStore != nil {
			pub, err := p.publisherStore.GetByPublisherID(ctx, publisherID)
			if err == nil && pub != nil {
				// Store publisher in context for exchange to access bid_multiplier
				ctx = context.WithValue(ctx, publisherContextKey, pub)
			}
		}

		r = r.WithContext(ctx)

		// Restore body for handler
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// extractPublisherInfo extracts publisher ID and domain from request
func (p *PublisherAuth) extractPublisherInfo(req *minimalBidRequest) (publisherID, domain string) {
	if req.Site != nil {
		domain = req.Site.Domain
		if req.Site.Publisher != nil {
			publisherID = req.Site.Publisher.ID
		}
	} else if req.App != nil {
		domain = req.App.Bundle
		if req.App.Publisher != nil {
			publisherID = req.App.Publisher.ID
		}
	}
	return
}

// validatePublisher validates the publisher ID and domain
// Fallback chain: Redis → PostgreSQL → Memory cache → RegisteredPubs
//
// LOCK ORDERING: mu only (Level 1)
// Does not hold locks across I/O operations (Redis, PostgreSQL)
// Calls getCachedPublisher/cachePublisher which handle their own locking
func (p *PublisherAuth) validatePublisher(ctx context.Context, publisherID, domain string) error {
	// Lock ordering: Level 1 (mu) - read config atomically
	p.mu.RLock()
	allowUnregistered := p.config.AllowUnregistered
	validateDomain := p.config.ValidateDomain
	publisherStore := p.publisherStore
	useRedis := p.config.UseRedis
	redisClient := p.redisClient
	// Make a copy of the map to avoid race conditions when map is modified concurrently
	var registeredPubs map[string]string
	if p.config.RegisteredPubs != nil {
		registeredPubs = make(map[string]string, len(p.config.RegisteredPubs))
		for k, v := range p.config.RegisteredPubs {
			registeredPubs[k] = v
		}
	}
	p.mu.RUnlock()
	// Release mu before any I/O or other lock acquisitions

	// No publisher ID
	if publisherID == "" {
		if allowUnregistered {
			return nil
		}
		return &PublisherAuthError{Code: "missing_publisher", Message: "publisher ID required"}
	}

	// 1. Try Redis FIRST (fastest if configured)
	if useRedis && redisClient != nil {
		allowedDomains, err := redisClient.HGet(ctx, RedisPublishersHash, publisherID)
		if err == nil && allowedDomains != "" {
			// Publisher found in Redis - validate domain and return
			if validateDomain && allowedDomains != "" && allowedDomains != "*" {
				if !p.domainMatches(domain, allowedDomains) {
					return &PublisherAuthError{Code: "domain_mismatch", Message: "domain not allowed for publisher"}
				}
			}
			return nil
		}
		// Redis error or not found - log and fall through to PostgreSQL
		if err != nil {
			p.logRedisFallback(err, publisherID)
		}
		// Continue to PostgreSQL fallback
	}

	// 2. Fall back to PostgreSQL database
	if publisherStore != nil {
		pub, err := publisherStore.GetByPublisherID(ctx, publisherID)
		if err == nil && pub != nil {
			// Publisher found in PostgreSQL - extract allowed domains
			type domainProvider interface {
				GetAllowedDomains() string
			}

			var allowedDomains string
			if dp, ok := pub.(domainProvider); ok {
				allowedDomains = dp.GetAllowedDomains()
			} else {
				// Try type assertion to map for flexibility
				if pubMap, ok := pub.(map[string]interface{}); ok {
					if ad, ok := pubMap["allowed_domains"].(string); ok {
						allowedDomains = ad
					}
				}
			}

			// Cache result in memory for 30s
			p.cachePublisher(publisherID, allowedDomains, 30*time.Second)

			// Validate domain if required
			if validateDomain && allowedDomains != "" && allowedDomains != "*" {
				if !p.domainMatches(domain, allowedDomains) {
					return &PublisherAuthError{Code: "domain_mismatch", Message: "domain not allowed for publisher"}
				}
			}
			return nil
		}
		// PostgreSQL error or not found - log and fall through to memory cache
		if err != nil {
			p.logDatabaseFallback(err, publisherID)
		}
		// Continue to memory cache fallback
	}

	// 3. Fall back to in-memory cache (30s TTL from previous PostgreSQL success)
	if allowedDomains := p.getCachedPublisher(publisherID); allowedDomains != "" {
		// Publisher found in cache - validate domain and return
		if validateDomain && allowedDomains != "" && allowedDomains != "*" {
			if !p.domainMatches(domain, allowedDomains) {
				return &PublisherAuthError{Code: "domain_mismatch", Message: "domain not allowed for publisher"}
			}
		}
		return nil
	}

	// 4. Check in-memory RegisteredPubs (for development/testing)
	if len(registeredPubs) > 0 {
		allowedDomains, exists := registeredPubs[publisherID]
		if exists {
			// Publisher found in RegisteredPubs - validate domain and return
			if validateDomain && allowedDomains != "" && allowedDomains != "*" {
				if !p.domainMatches(domain, allowedDomains) {
					return &PublisherAuthError{Code: "domain_mismatch", Message: "domain not allowed for publisher"}
				}
			}
			return nil
		}
		// Publisher not in RegisteredPubs - reject if we have publishers defined
		if !allowUnregistered {
			return &PublisherAuthError{Code: "unknown_publisher", Message: "publisher not registered"}
		}
	}

	// 5. All fallbacks exhausted - check if unregistered publishers are allowed
	if allowUnregistered {
		return nil
	}

	return &PublisherAuthError{
		Code:    "unknown_publisher",
		Message: "publisher not registered in any data source",
	}
}

// domainMatches checks if domain matches allowed domains (comma-separated)
func (p *PublisherAuth) domainMatches(domain, allowedDomains string) bool {
	if domain == "" {
		return false
	}

	for _, allowed := range strings.Split(allowedDomains, "|") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // ".example.com"
			if strings.HasSuffix(domain, suffix) || domain == allowed[2:] {
				return true
			}
		} else if domain == allowed {
			return true
		}
	}
	return false
}

// checkRateLimit implements token bucket rate limiting per publisher
//
// LOCK ORDERING: mu → rateLimitsMu
// This method acquires mu.RLock() first, then rateLimitsMu.Lock()
// Releases mu before acquiring rateLimitsMu to minimize lock holding time
func (p *PublisherAuth) checkRateLimit(publisherID string) bool {
	// Lock ordering: Level 1 (mu) first
	p.mu.RLock()
	rateLimit := p.config.RateLimitPerPub
	p.mu.RUnlock()
	// Release mu before acquiring other locks

	if rateLimit <= 0 {
		return true // Unlimited
	}

	// Lock ordering: Level 3 (rateLimitsMu)
	p.rateLimitsMu.Lock()
	defer p.rateLimitsMu.Unlock()

	entry, exists := p.rateLimits[publisherID]
	now := time.Now()

	if !exists {
		p.rateLimits[publisherID] = &rateLimitEntry{
			tokens:    float64(rateLimit) - 1,
			lastCheck: now,
		}
		return true
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(entry.lastCheck).Seconds()
	entry.tokens += elapsed * float64(rateLimit)
	if entry.tokens > float64(rateLimit) {
		entry.tokens = float64(rateLimit)
	}
	entry.lastCheck = now

	// Try to consume a token
	if entry.tokens >= 1 {
		entry.tokens--
		// Opportunistic cleanup to prevent unbounded memory growth
		// Remove stale entries if map is getting too large (>1000 entries)
		if len(p.rateLimits) > 1000 {
			p.cleanupStaleRateLimits(now)
		}
		return true
	}

	return false
}

// cleanupStaleRateLimits removes rate limit entries that haven't been accessed recently
// This prevents unbounded memory growth from unique publisher IDs (DoS vector)
// CALLER MUST HOLD rateLimitsMu.Lock()
func (p *PublisherAuth) cleanupStaleRateLimits(now time.Time) {
	// Remove entries not accessed in the last hour
	staleThreshold := now.Add(-1 * time.Hour)
	for pubID, entry := range p.rateLimits {
		if entry.lastCheck.Before(staleThreshold) {
			delete(p.rateLimits, pubID)
		}
	}
}

// Rate-limited logging state (shared across all PublisherAuth instances)
var (
	lastRedisWarning sync.Map // string -> time.Time
	lastDBWarning    sync.Map // string -> time.Time
)

// logRedisFallback logs Redis fallback with rate limiting (max 1 log per minute)
func (p *PublisherAuth) logRedisFallback(err error, pubID string) {
	key := "redis"
	if last, ok := lastRedisWarning.Load(key); ok {
		if time.Since(last.(time.Time)) < 1*time.Minute {
			return // Skip duplicate log
		}
	}
	lastRedisWarning.Store(key, time.Now())

	log.Warn().
		Err(err).
		Str("publisher_id", pubID).
		Msg("Redis unavailable, falling back to PostgreSQL")
}

// logDatabaseFallback logs database fallback with rate limiting (max 1 log per minute)
func (p *PublisherAuth) logDatabaseFallback(err error, pubID string) {
	key := "database"
	if last, ok := lastDBWarning.Load(key); ok {
		if time.Since(last.(time.Time)) < 1*time.Minute {
			return // Skip duplicate log
		}
	}
	lastDBWarning.Store(key, time.Now())

	log.Warn().
		Err(err).
		Str("publisher_id", pubID).
		Msg("PostgreSQL unavailable, falling back to memory cache")
}

// cachePublisher caches a publisher in memory with TTL
//
// LOCK ORDERING: publisherCacheMu only (Level 2)
// Safe to call from any context - does not acquire other locks
func (p *PublisherAuth) cachePublisher(publisherID, allowedDomains string, ttl time.Duration) {
	p.publisherCacheMu.Lock()
	defer p.publisherCacheMu.Unlock()

	if p.publisherCache == nil {
		p.publisherCache = make(map[string]*publisherCacheEntry)
	}

	p.publisherCache[publisherID] = &publisherCacheEntry{
		allowedDomains: allowedDomains,
		expiresAt:      time.Now().Add(ttl),
	}

	// Cleanup old entries (keep cache bounded to prevent memory issues)
	if len(p.publisherCache) > 1000 {
		p.cleanupExpiredCache()
	}
}

// getCachedPublisher retrieves a cached publisher if it exists and hasn't expired
//
// LOCK ORDERING: publisherCacheMu only (Level 2)
// Safe to call from any context - does not acquire other locks
func (p *PublisherAuth) getCachedPublisher(publisherID string) string {
	p.publisherCacheMu.RLock()
	defer p.publisherCacheMu.RUnlock()

	entry, ok := p.publisherCache[publisherID]
	if !ok {
		return ""
	}

	if time.Now().After(entry.expiresAt) {
		return "" // Expired
	}

	return entry.allowedDomains
}

// cleanupExpiredCache removes expired cache entries
// CALLER MUST HOLD publisherCacheMu.Lock()
func (p *PublisherAuth) cleanupExpiredCache() {
	now := time.Now()
	for pubID, entry := range p.publisherCache {
		if now.After(entry.expiresAt) {
			delete(p.publisherCache, pubID)
		}
	}
}

// IsEnabled returns whether publisher auth is enabled
func (p *PublisherAuth) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.Enabled
}

// RegisterPublisher adds a publisher at runtime
func (p *PublisherAuth) RegisterPublisher(publisherID, allowedDomains string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.config.RegisteredPubs == nil {
		p.config.RegisteredPubs = make(map[string]string)
	}
	p.config.RegisteredPubs[publisherID] = allowedDomains
}

// UnregisterPublisher removes a publisher at runtime
func (p *PublisherAuth) UnregisterPublisher(publisherID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.config.RegisteredPubs, publisherID)
}

// SetEnabled enables or disables publisher authentication
func (p *PublisherAuth) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Enabled = enabled
}

// SetIVTConfig updates IVT detection configuration at runtime
func (p *PublisherAuth) SetIVTConfig(config *IVTConfig) {
	if p.ivtDetector != nil {
		p.ivtDetector.SetConfig(config)
	}
}

// GetIVTConfig returns current IVT configuration
func (p *PublisherAuth) GetIVTConfig() *IVTConfig {
	if p.ivtDetector != nil {
		return p.ivtDetector.GetConfig()
	}
	return nil
}

// GetIVTMetrics returns current IVT detection metrics
func (p *PublisherAuth) GetIVTMetrics() IVTMetrics {
	if p.ivtDetector != nil {
		return p.ivtDetector.GetMetrics()
	}
	return IVTMetrics{}
}

// EnableIVTMonitoring enables/disables IVT monitoring (detection, logging, metrics)
func (p *PublisherAuth) EnableIVTMonitoring(enabled bool) {
	if p.ivtDetector != nil {
		config := p.ivtDetector.GetConfig()
		config.MonitoringEnabled = enabled
		p.ivtDetector.SetConfig(config)
	}
}

// EnableIVTBlocking enables/disables IVT blocking (requires monitoring to be enabled)
func (p *PublisherAuth) EnableIVTBlocking(enabled bool) {
	if p.ivtDetector != nil {
		config := p.ivtDetector.GetConfig()
		config.BlockingEnabled = enabled
		// If blocking is enabled but monitoring is not, enable monitoring automatically
		if enabled && !config.MonitoringEnabled {
			config.MonitoringEnabled = true
			log.Warn().Msg("IVT blocking requires monitoring - enabling monitoring automatically")
		}
		p.ivtDetector.SetConfig(config)
	}
}

// EnableIVT enables IVT monitoring (backward compatibility - use EnableIVTMonitoring instead)
// Deprecated: Use EnableIVTMonitoring for clarity
func (p *PublisherAuth) EnableIVT(enabled bool) {
	p.EnableIVTMonitoring(enabled)
}

// SetIVTBlockMode sets IVT blocking mode (backward compatibility - use EnableIVTBlocking instead)
// Deprecated: Use EnableIVTBlocking for clarity
func (p *PublisherAuth) SetIVTBlockMode(block bool) {
	p.EnableIVTBlocking(block)
}

// PublisherAuthError represents a publisher auth error
type PublisherAuthError struct {
	Code    string
	Message string
	Cause   error // Optional underlying cause
}

// Error returns a formatted error message including the error code
func (e *PublisherAuthError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

// Unwrap returns the underlying cause for error chain support
func (e *PublisherAuthError) Unwrap() error {
	return e.Cause
}

// PublisherFromContext retrieves the publisher object from the request context
// Returns nil if no publisher was set (e.g., unregistered publisher allowed)
func PublisherFromContext(ctx context.Context) interface{} {
	if pub := ctx.Value(publisherContextKey); pub != nil {
		return pub
	}
	return nil
}

// PublisherIDFromContext retrieves the publisher ID string from context
// Returns empty string if not set
func PublisherIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(publisherIDKey).(string); ok {
		return id
	}
	return ""
}

// NewContextWithPublisher creates a new context with the publisher set (for testing)
func NewContextWithPublisher(ctx context.Context, publisher interface{}) context.Context {
	return context.WithValue(ctx, publisherContextKey, publisher)
}

// NewContextWithPublisherID creates a new context with the publisher ID set (for testing)
func NewContextWithPublisherID(ctx context.Context, publisherID string) context.Context {
	return context.WithValue(ctx, publisherIDKey, publisherID)
}
