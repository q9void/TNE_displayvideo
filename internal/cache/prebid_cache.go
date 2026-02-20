// Package cache provides Prebid cache service integration
package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// CacheConfig configures Prebid cache client
type CacheConfig struct {
	// Enabled turns on cache integration
	Enabled bool

	// Endpoint is the Prebid cache service URL
	Endpoint string

	// Timeout for cache requests
	Timeout time.Duration

	// DefaultTTL is the default cache TTL in seconds
	DefaultTTL int

	// CacheBanner enables caching of banner creatives
	CacheBanner bool

	// CacheVideo enables caching of video creatives
	CacheVideo bool

	// CacheNative enables caching of native creatives
	CacheNative bool
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:     false, // Disabled by default, requires configuration
		Endpoint:    "https://prebid-cache.example.com/cache",
		Timeout:     500 * time.Millisecond,
		DefaultTTL:  300, // 5 minutes
		CacheBanner: true,
		CacheVideo:  true,
		CacheNative: true,
	}
}

// PrebidCache client for caching bids
type PrebidCache struct {
	config     *CacheConfig
	httpClient *http.Client
}

// NewPrebidCache creates a new Prebid cache client
func NewPrebidCache(config *CacheConfig) *PrebidCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	return &PrebidCache{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// CacheRequest represents a cache request
type CacheRequest struct {
	Puts []CachePut `json:"puts"`
}

// CachePut represents an item to cache
type CachePut struct {
	Type  string      `json:"type"`           // "json" or "xml"
	Value interface{} `json:"value"`          // The content to cache
	TTL   int         `json:"ttlseconds,omitempty"` // TTL in seconds
}

// CacheResponse represents a cache response
type CacheResponse struct {
	Responses []CacheResponseItem `json:"responses"`
}

// CacheResponseItem represents a cached item response
type CacheResponseItem struct {
	UUID string `json:"uuid"`
}

// CacheBid caches a bid creative and returns cache UUID
func (pc *PrebidCache) CacheBid(
	ctx context.Context,
	bid *openrtb.Bid,
	mediaType string,
) (string, error) {
	if !pc.config.Enabled {
		return "", fmt.Errorf("cache is disabled")
	}

	// Check if this media type should be cached
	if !pc.shouldCache(mediaType) {
		return "", nil
	}

	// Prepare cache request
	cacheReq := CacheRequest{
		Puts: []CachePut{
			{
				Type:  pc.getCacheType(mediaType),
				Value: pc.getCacheValue(bid, mediaType),
				TTL:   pc.config.DefaultTTL,
			},
		},
	}

	// Marshal request
	reqBody, err := json.Marshal(cacheReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cache request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		pc.config.Endpoint,
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create cache request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := pc.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("cache request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Log.Warn().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("Cache request returned non-200 status")
		return "", fmt.Errorf("cache returned status %d", resp.StatusCode)
	}

	// Parse response
	var cacheResp CacheResponse
	if err := json.NewDecoder(resp.Body).Decode(&cacheResp); err != nil {
		return "", fmt.Errorf("failed to decode cache response: %w", err)
	}

	if len(cacheResp.Responses) == 0 {
		return "", fmt.Errorf("cache returned no responses")
	}

	return cacheResp.Responses[0].UUID, nil
}

// CacheBids caches multiple bids and returns map of bid ID to cache UUID
func (pc *PrebidCache) CacheBids(
	ctx context.Context,
	bids map[string]*openrtb.Bid,
	mediaTypes map[string]string,
) (map[string]string, error) {
	if !pc.config.Enabled || len(bids) == 0 {
		return make(map[string]string), nil
	}

	// Prepare cache puts
	puts := make([]CachePut, 0, len(bids))
	bidIDs := make([]string, 0, len(bids))

	for bidID, bid := range bids {
		mediaType := mediaTypes[bidID]
		if !pc.shouldCache(mediaType) {
			continue
		}

		puts = append(puts, CachePut{
			Type:  pc.getCacheType(mediaType),
			Value: pc.getCacheValue(bid, mediaType),
			TTL:   pc.config.DefaultTTL,
		})
		bidIDs = append(bidIDs, bidID)
	}

	if len(puts) == 0 {
		return make(map[string]string), nil
	}

	// Send cache request
	cacheReq := CacheRequest{Puts: puts}
	reqBody, err := json.Marshal(cacheReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cache request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		pc.config.Endpoint,
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := pc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cache request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cache returned status %d", resp.StatusCode)
	}

	var cacheResp CacheResponse
	if err := json.NewDecoder(resp.Body).Decode(&cacheResp); err != nil {
		return nil, fmt.Errorf("failed to decode cache response: %w", err)
	}

	// Map bid IDs to cache UUIDs
	result := make(map[string]string)
	for i, uuid := range cacheResp.Responses {
		if i < len(bidIDs) {
			result[bidIDs[i]] = uuid.UUID
		}
	}

	return result, nil
}

// shouldCache checks if media type should be cached
func (pc *PrebidCache) shouldCache(mediaType string) bool {
	switch mediaType {
	case "banner":
		return pc.config.CacheBanner
	case "video":
		return pc.config.CacheVideo
	case "native":
		return pc.config.CacheNative
	default:
		return false
	}
}

// getCacheType returns cache type for media type
func (pc *PrebidCache) getCacheType(mediaType string) string {
	if mediaType == "video" {
		return "xml" // VAST is XML
	}
	return "json"
}

// getCacheValue extracts cacheable value from bid
func (pc *PrebidCache) getCacheValue(bid *openrtb.Bid, mediaType string) interface{} {
	// For video, prefer NURL (VAST URL) over AdM (VAST XML)
	if mediaType == "video" {
		if bid.NURL != "" {
			return bid.NURL
		}
		return bid.AdM
	}

	// For banner/native, use AdM (creative markup)
	return bid.AdM
}

// GetCacheURL returns the URL for a cached creative
func (pc *PrebidCache) GetCacheURL(uuid string) string {
	if uuid == "" {
		return ""
	}
	// Standard Prebid cache URL format
	return fmt.Sprintf("%s?uuid=%s", pc.config.Endpoint, uuid)
}
