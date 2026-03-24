// Package exchange implements the auction exchange
package exchange

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/internal/fpd"
	"github.com/thenexusengine/tne_springwire/internal/hooks"
	"github.com/thenexusengine/tne_springwire/internal/middleware"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/currency"
	"github.com/thenexusengine/tne_springwire/pkg/idr"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// flattenHeaders converts http.Header to a flat map for structured logging.
// Multi-value headers are joined with ", " (RFC 7230). Keys are kept canonical.
func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		out[k] = strings.Join(vals, ", ")
	}
	return out
}

// ValidationError represents a client validation error (results in 4xx response)
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(format string, args ...interface{}) *ValidationError {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// MetricsRecorder interface for recording revenue/margin metrics and circuit breaker metrics
type MetricsRecorder interface {
	// Auction and bid metrics
	RecordAuction(status, mediaType string, duration time.Duration, biddersSelected, biddersExcluded int)
	RecordBid(bidder, mediaType string, cpm float64)
	RecordBidderRequest(bidder string, latency time.Duration, hasError, timedOut bool)

	// Revenue/margin metrics
	RecordMargin(publisher, bidder, mediaType string, originalPrice, adjustedPrice, platformCut float64)
	RecordFloorAdjustment(publisher string)

	// Circuit breaker metrics
	SetBidderCircuitState(bidder, state string)
	RecordBidderCircuitRequest(bidder string)
	RecordBidderCircuitFailure(bidder string)
	RecordBidderCircuitSuccess(bidder string)
	RecordBidderCircuitRejected(bidder string)
	RecordBidderCircuitStateChange(bidder, fromState, toState string)
}

// Exchange orchestrates the auction process
type Exchange struct {
	registry          *adapters.Registry
	httpClient        adapters.HTTPClient
	idrClient         *idr.Client
	eventRecorder     *idr.EventRecorder
	config            *Config
	fpdProcessor      *fpd.Processor
	eidFilter         *fpd.EIDFilter
	metrics           MetricsRecorder
	currencyConverter *currency.Converter
	analytics         analytics.Module       // NEW: Analytics module for rich auction transaction data
	mfProcessor       *MultiformatProcessor  // Multiformat bid selection
	multibidProcessor *MultibidProcessor     // Task #52: Multibid processing

	// Per-bidder circuit breakers to prevent cascade failures
	bidderBreakers   map[string]*idr.CircuitBreaker
	bidderBreakersMu sync.RWMutex

	// configMu protects fpdProcessor, eidFilter, and config.FPD
	// for safe concurrent access during runtime config updates
	configMu sync.RWMutex
}

// AuctionType defines the type of auction to run
type AuctionType int

const (
	// FirstPriceAuction - winner pays their bid price
	FirstPriceAuction AuctionType = 1
	// SecondPriceAuction - winner pays second highest bid + increment
	SecondPriceAuction AuctionType = 2
)

// Default clone allocation limits (P1-3: prevent OOM from malicious requests)
// P3-1: These are now defaults; can be overridden via CloneLimits config
const (
	defaultMaxImpressionsPerRequest = 100 // Maximum impressions to clone
	defaultMaxEIDsPerUser           = 50  // Maximum EIDs to clone
	defaultMaxDataPerUser           = 20  // Maximum Data segments to clone
	defaultMaxDealsPerImp           = 50  // Maximum deals per impression
	defaultMaxSChainNodes           = 20  // Maximum supply chain nodes
)

// maxAllowedTMax caps TMax at a reasonable maximum to prevent resource exhaustion (10 seconds)
const maxAllowedTMax = 10000

// P2-7: NBR codes consolidated in openrtb/response.go
// Use openrtb.NoBidXxx constants for all no-bid reasons

// CloneLimits holds configurable limits for request cloning (P3-1)
type CloneLimits struct {
	MaxImpressionsPerRequest int // Maximum impressions to clone (default: 100)
	MaxEIDsPerUser           int // Maximum EIDs to clone (default: 50)
	MaxDataPerUser           int // Maximum Data segments to clone (default: 20)
	MaxDealsPerImp           int // Maximum deals per impression (default: 50)
	MaxSChainNodes           int // Maximum supply chain nodes (default: 20)
}

// DefaultCloneLimits returns default clone limits
func DefaultCloneLimits() *CloneLimits {
	return &CloneLimits{
		MaxImpressionsPerRequest: defaultMaxImpressionsPerRequest,
		MaxEIDsPerUser:           defaultMaxEIDsPerUser,
		MaxDataPerUser:           defaultMaxDataPerUser,
		MaxDealsPerImp:           defaultMaxDealsPerImp,
		MaxSChainNodes:           defaultMaxSChainNodes,
	}
}

// Config holds exchange configuration
type Config struct {
	DefaultTimeout       time.Duration
	MaxBidders           int
	MaxConcurrentBidders int // P0-4: Limit concurrent bidder goroutines (0 = unlimited)
	IDREnabled           bool
	IDRServiceURL        string
	IDRAPIKey            string // Internal API key for IDR service-to-service calls
	EventRecordEnabled   bool
	EventBufferSize      int
	CurrencyConv         bool
	DefaultCurrency      string
	CurrencyConverter    *currency.Converter // Currency conversion support
	Analytics            analytics.Module     // NEW: Analytics module for rich auction transaction data
	FPD                  *fpd.Config
	CloneLimits          *CloneLimits       // P3-1: Configurable clone limits
	MultiformatConfig    *MultiformatConfig // Multiformat bid selection config
	MultibidConfig       *MultibidConfig    // Task #52: Multibid processing config
	// Auction configuration
	AuctionType    AuctionType
	PriceIncrement float64 // For second-price auctions (typically 0.01)
	MinBidPrice    float64 // Minimum valid bid price
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:        1000 * time.Millisecond,
		MaxBidders:            50,
		MaxConcurrentBidders:  10, // P0-4: Limit concurrent HTTP requests per auction
		IDREnabled:            true,
		IDRServiceURL:         "http://localhost:5050",
		EventRecordEnabled:    true,
		EventBufferSize:       100,
		CurrencyConv:          false,
		DefaultCurrency:       "USD",
		FPD:                   fpd.DefaultConfig(),
		CloneLimits:           DefaultCloneLimits(), // P3-1: Configurable clone limits
		AuctionType:           FirstPriceAuction,
		PriceIncrement:        0.01,
		MinBidPrice:           0.0,
	}
}

// validateConfig validates config values and applies sensible defaults for invalid values
// P1-2: Prevent runtime panics or silent failures from bad configuration
func validateConfig(config *Config) *Config {
	defaults := DefaultConfig()

	// Timeout must be positive
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = defaults.DefaultTimeout
	}

	// MaxBidders must be positive
	if config.MaxBidders <= 0 {
		config.MaxBidders = defaults.MaxBidders
	}

	// MaxConcurrentBidders must be non-negative (0 means unlimited)
	if config.MaxConcurrentBidders < 0 {
		config.MaxConcurrentBidders = defaults.MaxConcurrentBidders
	}

	// AuctionType must be valid
	if config.AuctionType != FirstPriceAuction && config.AuctionType != SecondPriceAuction {
		config.AuctionType = FirstPriceAuction
	}

	// PriceIncrement must be positive for second-price auctions
	if config.AuctionType == SecondPriceAuction && config.PriceIncrement <= 0 {
		config.PriceIncrement = defaults.PriceIncrement
	}

	// MinBidPrice should not be negative
	if config.MinBidPrice < 0 {
		config.MinBidPrice = 0
	}

	// EventBufferSize must be positive if event recording is enabled
	if config.EventRecordEnabled && config.EventBufferSize <= 0 {
		config.EventBufferSize = defaults.EventBufferSize
	}

	// P3-1: Initialize CloneLimits if nil and validate values
	if config.CloneLimits == nil {
		config.CloneLimits = DefaultCloneLimits()
	} else {
		defaultLimits := DefaultCloneLimits()
		if config.CloneLimits.MaxImpressionsPerRequest <= 0 {
			config.CloneLimits.MaxImpressionsPerRequest = defaultLimits.MaxImpressionsPerRequest
		}
		if config.CloneLimits.MaxEIDsPerUser <= 0 {
			config.CloneLimits.MaxEIDsPerUser = defaultLimits.MaxEIDsPerUser
		}
		if config.CloneLimits.MaxDataPerUser <= 0 {
			config.CloneLimits.MaxDataPerUser = defaultLimits.MaxDataPerUser
		}
		if config.CloneLimits.MaxDealsPerImp <= 0 {
			config.CloneLimits.MaxDealsPerImp = defaultLimits.MaxDealsPerImp
		}
		if config.CloneLimits.MaxSChainNodes <= 0 {
			config.CloneLimits.MaxSChainNodes = defaultLimits.MaxSChainNodes
		}
	}

	return config
}

// New creates a new exchange
func New(registry *adapters.Registry, config *Config) *Exchange {
	if config == nil {
		config = DefaultConfig()
	}

	// P1-2: Validate and apply defaults for critical config fields
	config = validateConfig(config)

	// Initialize FPD config if not provided
	fpdConfig := config.FPD
	if fpdConfig == nil {
		fpdConfig = fpd.DefaultConfig()
	}

	// Initialize multiformat config if not provided
	mfConfig := config.MultiformatConfig
	if mfConfig == nil {
		mfConfig = DefaultMultiformatConfig()
	}

	// Task #52: Initialize multibid config if not provided
	multibidConfig := config.MultibidConfig
	if multibidConfig == nil {
		multibidConfig = DefaultMultibidConfig()
	}

	ex := &Exchange{
		registry:          registry,
		httpClient:        adapters.NewHTTPClient(config.DefaultTimeout),
		config:            config,
		fpdProcessor:      fpd.NewProcessor(fpdConfig),
		eidFilter:         fpd.NewEIDFilter(fpdConfig),
		bidderBreakers:    make(map[string]*idr.CircuitBreaker),
		currencyConverter: config.CurrencyConverter,
		analytics:         config.Analytics,         // NEW: Set analytics module from config
		mfProcessor:       NewMultiformatProcessor(mfConfig),
		multibidProcessor: NewMultibidProcessor(multibidConfig), // Task #52: Initialize multibid processor
	}

	// Initialize circuit breaker for each registered bidder
	for _, bidderCode := range registry.ListEnabledBidders() {
		ex.initBidderCircuitBreaker(bidderCode)
	}

	if config.IDREnabled && config.IDRServiceURL != "" {
		ex.idrClient = idr.NewClient(config.IDRServiceURL, 50*time.Millisecond, config.IDRAPIKey)
	}

	if config.EventRecordEnabled && config.IDRServiceURL != "" {
		ex.eventRecorder = idr.NewEventRecorder(config.IDRServiceURL, config.EventBufferSize)
	}

	return ex
}

// SetMetrics sets the metrics recorder for tracking revenue/margins
func (e *Exchange) SetMetrics(m MetricsRecorder) {
	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.metrics = m
}

// Close shuts down the exchange and flushes pending events
func (e *Exchange) Close() error {
	// Close circuit breakers (wait for pending callbacks)
	e.bidderBreakersMu.RLock()
	for _, breaker := range e.bidderBreakers {
		breaker.Close()
	}
	e.bidderBreakersMu.RUnlock()

	// Shutdown analytics module (flush buffers)
	if e.analytics != nil {
		if err := e.analytics.Shutdown(); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to shutdown analytics module")
		}
	}

	// Flush event recorder (legacy - will be removed after migration)
	if e.eventRecorder != nil {
		return e.eventRecorder.Close()
	}
	return nil
}

// initBidderCircuitBreaker initializes a circuit breaker for a specific bidder
func (e *Exchange) initBidderCircuitBreaker(bidderCode string) {
	config := &idr.CircuitBreakerConfig{
		FailureThreshold: 5,               // Open after 5 consecutive failures
		SuccessThreshold: 2,              // Close after 2 successes in half-open
		Timeout:          30 * time.Second, // Wait 30s before testing recovery
		MaxConcurrent:    100,            // Max concurrent requests per bidder
		OnStateChange: func(from, to string) {
			logger.Log.Warn().
				Str("bidder_code", bidderCode).
				Str("from_state", from).
				Str("to_state", to).
				Msg("Bidder circuit breaker state changed")

			// Record state change metrics
			if e.metrics != nil {
				e.metrics.SetBidderCircuitState(bidderCode, to)
				e.metrics.RecordBidderCircuitStateChange(bidderCode, from, to)
			}
		},
	}

	e.bidderBreakersMu.Lock()
	e.bidderBreakers[bidderCode] = idr.NewCircuitBreaker(config)
	e.bidderBreakersMu.Unlock()

	// Initialize state metric to closed
	if e.metrics != nil {
		e.metrics.SetBidderCircuitState(bidderCode, "closed")
	}
}

// getBidderCircuitBreaker retrieves the circuit breaker for a specific bidder
func (e *Exchange) getBidderCircuitBreaker(bidderCode string) *idr.CircuitBreaker {
	e.bidderBreakersMu.RLock()
	defer e.bidderBreakersMu.RUnlock()
	return e.bidderBreakers[bidderCode]
}

// GetBidderCircuitBreakerStats returns circuit breaker stats for all bidders
func (e *Exchange) GetBidderCircuitBreakerStats() map[string]idr.CircuitBreakerStats {
	e.bidderBreakersMu.RLock()
	defer e.bidderBreakersMu.RUnlock()

	stats := make(map[string]idr.CircuitBreakerStats)
	for bidderCode, breaker := range e.bidderBreakers {
		stats[bidderCode] = breaker.Stats()
	}
	return stats
}

// AuctionRequest contains auction parameters
type AuctionRequest struct {
	BidRequest *openrtb.BidRequest
	Timeout    time.Duration
	Account    string
	Debug      bool
}

// AuctionResponse contains auction results
type AuctionResponse struct {
	BidResponse   *openrtb.BidResponse
	BidderResults map[string]*BidderResult
	IDRResult     *idr.SelectPartnersResponse
	DebugInfo     *DebugInfo
}

// BidderResult contains results from a single bidder
type BidderResult struct {
	BidderCode string
	Bids       []*adapters.TypedBid
	Currency   string // Currency of the bids (after conversion)
	Errors     []error
	Latency    time.Duration
	Selected   bool
	Score      float64
	TimedOut        bool   // P2-2: indicates if the bidder request timed out
	LastStatusCode  int    // CP-5: HTTP status from the last SSP response (0 if no response)
	RejectionReason string // CP-5: Value of X-Rejection-Reason response header if present
}

// DebugInfo contains debug information
type DebugInfo struct {
	RequestTime     time.Time
	TotalLatency    time.Duration
	IDRLatency      time.Duration
	BidderLatencies map[string]time.Duration
	SelectedBidders []string
	ExcludedBidders []string
	Errors          map[string][]string
	errorsMu        sync.Mutex // Protects concurrent access to Errors map
}

// AddError safely adds errors to the Errors map with mutex protection
func (d *DebugInfo) AddError(key string, errors []string) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()
	d.Errors[key] = errors
}

// AppendError safely appends an error to the Errors map with mutex protection
func (d *DebugInfo) AppendError(key string, errMsg string) {
	d.errorsMu.Lock()
	defer d.errorsMu.Unlock()
	d.Errors[key] = append(d.Errors[key], errMsg)
}

// RequestValidationError represents a bid request validation failure
type RequestValidationError struct {
	Field  string
	Reason string
}

func (e *RequestValidationError) Error() string {
	return fmt.Sprintf("invalid request: %s - %s", e.Field, e.Reason)
}

// ValidateRequest performs OpenRTB 2.x request validation
// Returns nil if valid, or a RequestValidationError describing the issue
func ValidateRequest(req *openrtb.BidRequest) *RequestValidationError {
	if req == nil {
		return &RequestValidationError{Field: "request", Reason: "nil request"}
	}

	// Validate required field: ID
	if req.ID == "" {
		return &RequestValidationError{Field: "id", Reason: "missing required field"}
	}

	// Validate required field: at least one impression
	if len(req.Imp) == 0 {
		return &RequestValidationError{Field: "imp", Reason: "at least one impression is required"}
	}

	// Validate impression IDs are unique and non-empty
	impIDs := make(map[string]struct{}, len(req.Imp))
	for i, imp := range req.Imp {
		if imp.ID == "" {
			return &RequestValidationError{
				Field:  fmt.Sprintf("imp[%d].id", i),
				Reason: "impression ID is required",
			}
		}
		if _, exists := impIDs[imp.ID]; exists {
			return &RequestValidationError{
				Field:  fmt.Sprintf("imp[%d].id", i),
				Reason: fmt.Sprintf("duplicate impression ID: %s", imp.ID),
			}
		}
		impIDs[imp.ID] = struct{}{}
	}

	// Validate exactly one distribution channel: Site XOR App XOR DOOH (OpenRTB 2.6)
	hasSite := req.Site != nil
	hasApp := req.App != nil
	hasDOOH := req.DOOH != nil
	channelCount := 0
	if hasSite {
		channelCount++
	}
	if hasApp {
		channelCount++
	}
	if hasDOOH {
		channelCount++
	}
	if channelCount > 1 {
		return &RequestValidationError{
			Field:  "site/app/dooh",
			Reason: "request must contain only one of site, app, or dooh objects",
		}
	}
	if channelCount == 0 {
		return &RequestValidationError{
			Field:  "site/app/dooh",
			Reason: "request must contain one of site, app, or dooh objects",
		}
	}

	// Validate TMax if present (reasonable bounds: 0 means no limit, otherwise 10ms-30000ms)
	if req.TMax < 0 {
		return &RequestValidationError{
			Field:  "tmax",
			Reason: fmt.Sprintf("tmax cannot be negative: %d", req.TMax),
		}
	}
	if req.TMax > 0 && req.TMax < 10 {
		return &RequestValidationError{
			Field:  "tmax",
			Reason: fmt.Sprintf("tmax too small (minimum 10ms): %d", req.TMax),
		}
	}
	if req.TMax > 30000 {
		return &RequestValidationError{
			Field:  "tmax",
			Reason: fmt.Sprintf("tmax too large (maximum 30000ms): %d", req.TMax),
		}
	}

	return nil
}

// BidValidationError represents a bid validation failure
type BidValidationError struct {
	BidID      string
	ImpID      string
	Reason     string
	BidderCode string
}

func (e *BidValidationError) Error() string {
	return fmt.Sprintf("invalid bid from %s (bid=%s, imp=%s): %s", e.BidderCode, e.BidID, e.ImpID, e.Reason)
}

// validateURL validates that a URL string is properly formatted and uses HTTPS
func validateURL(urlStr string, requireHTTPS bool) error {
	if urlStr == "" {
		return fmt.Errorf("empty URL")
	}

	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	// Check scheme
	if u.Scheme == "" {
		return fmt.Errorf("missing URL scheme")
	}

	if requireHTTPS && u.Scheme != "https" {
		return fmt.Errorf("URL must use HTTPS, got %s", u.Scheme)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (must be http or https)", u.Scheme)
	}

	// Check host
	if u.Host == "" {
		return fmt.Errorf("missing URL host")
	}

	return nil
}

// validateBidMediaType validates that the bid matches an available media type in the impression
// OpenRTB 2.5 Section 3.2.4: While impressions can offer multiple media types,
// the bid must clearly indicate which type it's for
func validateBidMediaType(bid *openrtb.Bid, imp *openrtb.Imp) error {
	// Determine which media type the bid is for based on its properties
	// Priority: Video (has protocol) > Native (no dimensions) > Banner (default)

	// Check if this is a video bid (has protocol field set)
	isVideoBid := bid.Protocol > 0

	// Check if this is a native bid (typically has no dimensions)
	// Issue #10 fix: Removed imp.Banner == nil check to support multiformat (banner + native)
	// Native bids are identified by lack of dimensions, not by exclusion of other formats
	isNativeBid := bid.W == 0 && bid.H == 0 && bid.Protocol == 0 && imp.Native != nil

	// Video bid validation
	if isVideoBid {
		if imp.Video == nil {
			return fmt.Errorf("video bid for impression without video object")
		}
		return nil
	}

	// Native bid validation
	if isNativeBid {
		if imp.Native == nil {
			return fmt.Errorf("native bid for impression without native object")
		}
		return nil
	}

	// Banner bid validation (default case)
	// Banner bids should have dimensions
	if bid.W > 0 || bid.H > 0 {
		if imp.Banner == nil {
			return fmt.Errorf("banner bid for impression without banner object")
		}
	}

	return nil
}

// validateBannerDimensions validates that banner bid dimensions match allowed formats
// OpenRTB 2.5: Bid dimensions must match banner.format[] or banner.w/h
func validateBannerDimensions(bid *openrtb.Bid, banner *openrtb.Banner) error {
	// If bid has no dimensions, we can't validate (some exchanges allow this)
	if bid.W == 0 && bid.H == 0 {
		return nil
	}

	// Check against explicit banner.w/h
	if banner.W > 0 && banner.H > 0 {
		if bid.W == banner.W && bid.H == banner.H {
			return nil
		}
	}

	// Check against banner.format[] array
	if len(banner.Format) > 0 {
		for _, format := range banner.Format {
			if format.W > 0 && format.H > 0 {
				if bid.W == format.W && bid.H == format.H {
					return nil
				}
			}
		}
		// If we have formats but no match, that's an error
		return fmt.Errorf("bid dimensions %dx%d do not match any allowed banner formats", bid.W, bid.H)
	}

	// If no explicit dimensions or formats specified, allow any dimensions
	return nil
}

// validateBid checks if a bid meets OpenRTB requirements and exchange rules
func (e *Exchange) validateBid(bid *openrtb.Bid, bidderCode string, req *openrtb.BidRequest, impMap map[string]*openrtb.Imp, impFloors map[string]float64) *BidValidationError {
	if bid == nil {
		return &BidValidationError{BidderCode: bidderCode, Reason: "nil bid"}
	}

	// Check required field: Bid.ID
	if bid.ID == "" {
		return &BidValidationError{
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     "missing required field: id",
		}
	}

	// Check required field: Bid.ImpID
	if bid.ImpID == "" {
		return &BidValidationError{
			BidID:      bid.ID,
			BidderCode: bidderCode,
			Reason:     "missing required field: impid",
		}
	}

	// Validate ImpID exists in request
	imp, validImp := impMap[bid.ImpID]
	if !validImp {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("impid %q not found in request", bid.ImpID),
		}
	}

	// Check price is non-negative
	if bid.Price < 0 {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("negative price: %.4f", bid.Price),
		}
	}

	// Check for NaN or Inf in bid price
	if math.IsNaN(bid.Price) || math.IsInf(bid.Price, 0) {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("invalid price (NaN/Inf): %.4f", bid.Price),
		}
	}

	// Validate bid price is reasonable (not > $1000 CPM)
	if bid.Price > maxReasonableCPM {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("price %.4f exceeds maximum reasonable CPM %.4f", bid.Price, maxReasonableCPM),
		}
	}

	// Check price meets minimum
	if bid.Price < e.config.MinBidPrice {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("price %.4f below minimum %.4f", bid.Price, e.config.MinBidPrice),
		}
	}

	// Check price meets floor
	floor := impFloors[bid.ImpID]
	if floor > 0 && bid.Price < floor {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     fmt.Sprintf("price %.4f below floor %.4f", bid.Price, floor),
		}
	}

	// P2-1: Validate that bid has creative content (AdM or NURL required)
	// OpenRTB 2.x requires either inline markup (adm) or a URL to fetch it (nurl)
	if bid.AdM == "" && bid.NURL == "" {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     "bid must have either adm or nurl",
		}
	}

	// CRITICAL FIX #2: Validate NURL format if present
	// Task #51: Allow HTTP for nurl (not just HTTPS) - some bidders use HTTP endpoints
	if bid.NURL != "" {
		if err := validateURL(bid.NURL, false); err != nil {
			return &BidValidationError{
				BidID:      bid.ID,
				ImpID:      bid.ImpID,
				BidderCode: bidderCode,
				Reason:     fmt.Sprintf("invalid nurl format: %v", err),
			}
		}
	}

	// CRITICAL FIX #1: Validate ADomain against blocked advertisers
	// OpenRTB 2.5: Check bid.ADomain doesn't contain any domains from request.BAdv
	if len(bid.ADomain) > 0 && len(req.BAdv) > 0 {
		for _, adomain := range bid.ADomain {
			for _, blocked := range req.BAdv {
				if strings.EqualFold(adomain, blocked) {
					return &BidValidationError{
						BidID:      bid.ID,
						ImpID:      bid.ImpID,
						BidderCode: bidderCode,
						Reason:     fmt.Sprintf("blocked advertiser domain: %s", adomain),
					}
				}
			}
		}
	}

	// CRITICAL FIX #3: Validate bid type matches impression media types
	// OpenRTB 2.5 Section 3.2.4: Bid must match an available media type in the impression
	if err := validateBidMediaType(bid, imp); err != nil {
		return &BidValidationError{
			BidID:      bid.ID,
			ImpID:      bid.ImpID,
			BidderCode: bidderCode,
			Reason:     err.Error(),
		}
	}

	// Banner dimension validation intentionally removed — SSPs (e.g. Kargo) return valid
	// creatives with bid.w/h that differ from the requested formats. PBS does not enforce
	// dimension matching; we trust the placement ID / ad slot to ensure correct rendering.

	return nil
}

// buildImpFloorMap creates a map of impression IDs to their floor prices
// If publisher has a bid_multiplier, floors are MULTIPLIED to ensure platform gets its cut
// Example: floor=$1, multiplier=1.05 → adjusted_floor=$1.05 (DSPs must bid at least $1.05)
func (e *Exchange) buildImpFloorMap(ctx context.Context, req *openrtb.BidRequest) map[string]float64 {
	impFloors := make(map[string]float64, len(req.Imp))

	// Get target currency for floor validation (issue #15)
	// Handle nil config gracefully for tests
	var targetCurrency string
	if e.config != nil {
		targetCurrency = e.extractTargetCurrency(req)
	} else {
		// Default to USD if no config (test scenario)
		if len(req.Cur) > 0 && req.Cur[0] != "" {
			targetCurrency = req.Cur[0]
		} else {
			targetCurrency = "USD"
		}
	}

	// Extract custom currency rates from ext.prebid.currency (issue #17)
	customRates, useExternalRates := extractCustomRates(req)

	// Get publisher's bid multiplier
	var multiplier float64 = 1.0
	var publisherID string
	if pub := middleware.PublisherFromContext(ctx); pub != nil {
		if v, ok := extractBidMultiplier(pub); ok && v >= 1.0 && v <= 10.0 {
			multiplier = v
		}
		// Extract publisher ID for metrics
		if pid, ok := extractPublisherID(pub); ok {
			publisherID = pid
		}
	}

	// Build floor map with currency conversion and multiplier applied
	floorsAdjusted := 0
	floorsConverted := 0
	for _, imp := range req.Imp {
		baseFloor := imp.BidFloor
		floorCurrency := imp.BidFloorCur
		if floorCurrency == "" {
			floorCurrency = targetCurrency // Default to target currency
		}

		// Normalize currency code (issue #16: usd → USD)
		floorCurrency = normalizeIsoCurrency(floorCurrency)

		// Convert floor to target currency if needed (issue #15)
		if baseFloor > 0 && floorCurrency != targetCurrency {
			convertedFloor, err := e.convertBidCurrency(
				baseFloor,
				floorCurrency,
				targetCurrency,
				customRates,
				useExternalRates,
			)
			if err != nil {
				logger.Log.Warn().
					Str("impID", imp.ID).
					Str("floorCurrency", floorCurrency).
					Str("targetCurrency", targetCurrency).
					Float64("baseFloor", baseFloor).
					Err(err).
					Msg("Failed to convert floor currency, using original floor")
				// Keep original floor on conversion failure
			} else {
				baseFloor = convertedFloor
				floorsConverted++
				logger.Log.Debug().
					Str("impID", imp.ID).
					Str("from", floorCurrency).
					Str("to", targetCurrency).
					Float64("originalFloor", imp.BidFloor).
					Float64("convertedFloor", baseFloor).
					Msg("Converted floor currency")
			}
		}

		// Validate base floor is non-negative and reasonable
		if baseFloor < 0 {
			logger.Log.Warn().
				Str("impID", imp.ID).
				Float64("base_floor", baseFloor).
				Msg("Negative floor price detected, setting to 0")
			baseFloor = 0
		}

		// Check for NaN or Inf in base floor
		if math.IsNaN(baseFloor) || math.IsInf(baseFloor, 0) {
			logger.Log.Warn().
				Str("impID", imp.ID).
				Float64("base_floor", baseFloor).
				Msg("Invalid floor price (NaN/Inf), setting to 0")
			baseFloor = 0
		}

		if multiplier != 1.0 && baseFloor > 0 {
			// Multiply floor so DSPs must bid higher to cover platform's cut
			adjustedFloor := baseFloor * multiplier

			// Check for overflow in multiplication
			if math.IsInf(adjustedFloor, 1) {
				logger.Log.Error().
					Str("impID", imp.ID).
					Float64("base_floor", baseFloor).
					Float64("multiplier", multiplier).
					Msg("Floor price multiplication overflow, using base floor")
				impFloors[imp.ID] = baseFloor
				continue
			}

			// Validate adjusted floor is reasonable (not > $1000 CPM)
			if adjustedFloor > maxReasonableCPM {
				logger.Log.Warn().
					Str("impID", imp.ID).
					Float64("base_floor", baseFloor).
					Float64("multiplier", multiplier).
					Float64("adjusted_floor", adjustedFloor).
					Float64("max_cpm", maxReasonableCPM).
					Msg("Adjusted floor exceeds maximum reasonable CPM, capping")
				adjustedFloor = maxReasonableCPM
			}

			impFloors[imp.ID] = roundToCents(adjustedFloor)
			floorsAdjusted++

			logger.Log.Debug().
				Str("impID", imp.ID).
				Float64("base_floor", baseFloor).
				Float64("multiplier", multiplier).
				Float64("adjusted_floor", impFloors[imp.ID]).
				Msg("Applied multiplier to floor price")
		} else {
			impFloors[imp.ID] = baseFloor
		}
	}

	// Record floor adjustments metric
	if floorsAdjusted > 0 && publisherID != "" {
		e.configMu.RLock()
		if e.metrics != nil {
			for i := 0; i < floorsAdjusted; i++ {
				e.metrics.RecordFloorAdjustment(publisherID)
			}
		}
		e.configMu.RUnlock()
	}

	// Log summary of floor processing (issues #15, #16, #17)
	if floorsConverted > 0 || floorsAdjusted > 0 {
		logger.Log.Info().
			Int("floors_converted", floorsConverted).
			Int("floors_adjusted", floorsAdjusted).
			Int("total_impressions", len(req.Imp)).
			Str("target_currency", targetCurrency).
			Msg("Processed floor prices with currency conversion and multipliers")
	}

	return impFloors
}

// ValidatedBid wraps a bid with validation status
type ValidatedBid struct {
	Bid        *adapters.TypedBid
	BidderCode string
	DemandType adapters.DemandType // platform (obfuscated) or publisher (transparent)
}

// runAuctionLogic applies auction rules (first-price or second-price) to validated bids
// Returns bids grouped by impression with prices adjusted according to auction type
func (e *Exchange) runAuctionLogic(validBids []ValidatedBid, impFloors map[string]float64) map[string][]ValidatedBid {
	// Group bids by impression
	bidsByImp := make(map[string][]ValidatedBid)
	for _, vb := range validBids {
		impID := vb.Bid.Bid.ImpID
		bidsByImp[impID] = append(bidsByImp[impID], vb)
	}

	// Apply auction logic per impression
	for impID, bids := range bidsByImp {
		if len(bids) == 0 {
			continue
		}

		// Sort by price descending
		sortBidsByPrice(bids)

		if e.config.AuctionType == SecondPriceAuction {
			var winningPrice float64
			originalBidPrice := bids[0].Bid.Bid.Price

			// Validate original bid price before calculations
			if originalBidPrice < 0 || math.IsNaN(originalBidPrice) || math.IsInf(originalBidPrice, 0) {
				logger.Log.Warn().
					Str("impID", impID).
					Str("bidder", bids[0].BidderCode).
					Float64("bidPrice", originalBidPrice).
					Msg("Invalid bid price in auction logic, rejecting")
				bidsByImp[impID] = nil
				continue
			}

			if len(bids) > 1 {
				// Multiple bids: winner pays second highest + increment
				// Use integer arithmetic to avoid floating-point precision errors (P0-2)
				secondPrice := bids[1].Bid.Bid.Price

				// Validate second price before addition
				if secondPrice < 0 || math.IsNaN(secondPrice) || math.IsInf(secondPrice, 0) {
					logger.Log.Warn().
						Str("impID", impID).
						Float64("secondPrice", secondPrice).
						Msg("Invalid second price, using floor instead")
					secondPrice = impFloors[impID]
				}

				// Check for overflow in addition
				if secondPrice > maxReasonableCPM-e.config.PriceIncrement {
					logger.Log.Warn().
						Str("impID", impID).
						Float64("secondPrice", secondPrice).
						Float64("increment", e.config.PriceIncrement).
						Msg("Second price + increment would overflow, capping")
					winningPrice = maxReasonableCPM
				} else {
					winningPrice = roundToCents(secondPrice + e.config.PriceIncrement)
				}
			} else if len(bids) == 1 {
				// Single bid: winner pays their bid or floor, whichever is higher
				floor := impFloors[impID]

				// Validate floor is reasonable
				if floor < 0 || math.IsNaN(floor) || math.IsInf(floor, 0) {
					floor = 0
				}

				// Winner pays max(floor, bidPrice)
				if floor > originalBidPrice {
					winningPrice = roundToCents(floor)
				} else {
					winningPrice = originalBidPrice
				}
			}

			// Validate winning price is non-negative and reasonable
			if winningPrice < 0 {
				winningPrice = 0
			}
			if winningPrice > maxReasonableCPM {
				logger.Log.Warn().
					Str("impID", impID).
					Float64("winningPrice", winningPrice).
					Float64("maxCPM", maxReasonableCPM).
					Msg("Winning price exceeds maximum, capping")
				winningPrice = maxReasonableCPM
			}

			// P2-2: For multiple bids, reject if winning price exceeds bid
			// For single bids, this check was already handled above
			if len(bids) > 1 && winningPrice > originalBidPrice {
				// P2-3: Log bid rejection for debugging auction behavior
				logger.Log.Debug().
					Str("impID", impID).
					Str("bidder", bids[0].BidderCode).
					Float64("bidPrice", originalBidPrice).
					Float64("clearingPrice", winningPrice).
					Float64("floor", impFloors[impID]).
					Float64("increment", e.config.PriceIncrement).
					Msg("bid rejected: clearing price exceeds bid in second-price auction")
				bidsByImp[impID] = nil
				continue
			}
			bids[0].Bid.Bid.Price = winningPrice
		}
		// First-price: winner pays their bid (no adjustment needed)

		bidsByImp[impID] = bids
	}

	return bidsByImp
}

// sortBidsByPrice sorts bids in descending order by price (highest first)
// Includes defensive nil checks to prevent panics
func sortBidsByPrice(bids []ValidatedBid) {
	// Simple insertion sort - typically small number of bids per impression
	for i := 1; i < len(bids); i++ {
		j := i
		for j > 0 {
			// Defensive nil checks (P1-5)
			if bids[j].Bid == nil || bids[j].Bid.Bid == nil ||
				bids[j-1].Bid == nil || bids[j-1].Bid.Bid == nil {
				break
			}
			if bids[j].Bid.Bid.Price > bids[j-1].Bid.Bid.Price {
				bids[j], bids[j-1] = bids[j-1], bids[j]
				j--
			} else {
				break
			}
		}
	}
}

// Price validation constants - ensure bid prices are reasonable
const (
	maxReasonableCPM = 1000.0 // Maximum reasonable CPM in dollars ($1000)
	minBidPrice      = 0.0    // Minimum bid price (non-negative)
)

// roundToCents rounds a price to 2 decimal places
// P2-NEW-3: Use math.Round for correct rounding of all values including edge cases
// Note: This function allows negative values for intermediate calculations (e.g., platform cut)
// but bid prices themselves are validated separately to ensure they're non-negative
func roundToCents(price float64) float64 {
	// Validate input to prevent NaN and Inf propagation
	if math.IsNaN(price) || math.IsInf(price, 0) {
		return 0.0
	}

	// math.Round correctly handles all cases including negative numbers and .5 values
	return math.Round(price*100) / 100.0
}

// applyBidMultiplier applies the publisher's bid multiplier to all bids
// This allows the platform to take a revenue share before returning bids to the publisher
// Bid prices are DIVIDED by the multiplier
// For example: multiplier = 1.05 means publisher gets ~95%, platform keeps ~5% of bid price
func (e *Exchange) applyBidMultiplier(ctx context.Context, bidsByImp map[string][]ValidatedBid) map[string][]ValidatedBid {
	// Get publisher from context (set by publisher_auth middleware)
	pub := middleware.PublisherFromContext(ctx)
	if pub == nil {
		return bidsByImp // No publisher configured, no multiplier to apply
	}

	// Extract bid multiplier and publisher ID from publisher
	var multiplier float64 = 1.0 // Default: no adjustment
	var publisherID string

	// Try to extract via struct field access
	type publisherWithMultiplier struct {
		BidMultiplier float64
	}

	// Use type switch to handle different publisher types
	switch p := pub.(type) {
	case *publisherWithMultiplier:
		multiplier = p.BidMultiplier
	default:
		// Try to extract via reflection for any struct with BidMultiplier field
		// This handles the actual storage.Publisher type
		if v, ok := extractBidMultiplier(pub); ok {
			multiplier = v
		}
	}

	// Extract publisher ID for metrics
	if pid, ok := extractPublisherID(pub); ok {
		publisherID = pid
	}

	// If multiplier is 1.0 (or 0, meaning default), no adjustment needed
	if multiplier == 0 || multiplier == 1.0 {
		return bidsByImp
	}

	// Validate multiplier is in reasonable range (1.0 to 10.0)
	if multiplier < 1.0 || multiplier > 10.0 {
		logger.Log.Warn().
			Float64("multiplier", multiplier).
			Msg("Invalid bid multiplier, ignoring")
		return bidsByImp
	}

	// Additional validation: check for NaN or Inf in multiplier
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		logger.Log.Error().
			Float64("multiplier", multiplier).
			Msg("Invalid bid multiplier (NaN/Inf), ignoring")
		return bidsByImp
	}

	// Apply multiplier to all bid prices (DIVIDE to reduce what publisher sees)
	for impID, bids := range bidsByImp {
		for i := range bids {
			if bids[i].Bid != nil && bids[i].Bid.Bid != nil {
				originalPrice := bids[i].Bid.Bid.Price

				// Validate original price before division
				if originalPrice < 0 {
					logger.Log.Warn().
						Str("impID", impID).
						Str("bidder", bids[i].BidderCode).
						Float64("price", originalPrice).
						Msg("Negative bid price detected in multiplier application, skipping")
					continue
				}

				// Check for NaN or Inf in original price
				if math.IsNaN(originalPrice) || math.IsInf(originalPrice, 0) {
					logger.Log.Warn().
						Str("impID", impID).
						Str("bidder", bids[i].BidderCode).
						Float64("price", originalPrice).
						Msg("Invalid bid price (NaN/Inf) in multiplier application, skipping")
					continue
				}

				// Perform division with bounds checking
				adjustedPrice := originalPrice / multiplier

				// Check for underflow (price becomes too small)
				if adjustedPrice < 0.01 && originalPrice > 0 {
					logger.Log.Warn().
						Str("impID", impID).
						Str("bidder", bids[i].BidderCode).
						Float64("original_price", originalPrice).
						Float64("multiplier", multiplier).
						Float64("adjusted_price", adjustedPrice).
						Msg("Multiplier division resulted in very small price, setting minimum")
					adjustedPrice = 0.01
				}

				// Round and validate result
				adjustedPrice = roundToCents(adjustedPrice)

				// Ensure adjusted price is non-negative
				if adjustedPrice < 0 {
					adjustedPrice = 0
				}

				// Validate adjusted price is reasonable
				if adjustedPrice > maxReasonableCPM {
					logger.Log.Warn().
						Str("impID", impID).
						Str("bidder", bids[i].BidderCode).
						Float64("adjusted_price", adjustedPrice).
						Float64("max_cpm", maxReasonableCPM).
						Msg("Adjusted price exceeds maximum reasonable CPM, capping")
					adjustedPrice = maxReasonableCPM
				}

				platformCut := originalPrice - adjustedPrice

				// Validate platform cut is non-negative
				if platformCut < 0 {
					logger.Log.Warn().
						Str("impID", impID).
						Str("bidder", bids[i].BidderCode).
						Float64("original_price", originalPrice).
						Float64("adjusted_price", adjustedPrice).
						Float64("platform_cut", platformCut).
						Msg("Negative platform cut detected, adjusting")
					platformCut = 0
					adjustedPrice = originalPrice
				}

				// Determine media type from bid
				mediaType := "banner" // default
				if bids[i].Bid.BidType == adapters.BidTypeVideo {
					mediaType = "video"
				} else if bids[i].Bid.BidType == adapters.BidTypeNative {
					mediaType = "native"
				} else if bids[i].Bid.BidType == adapters.BidTypeAudio {
					mediaType = "audio"
				}

				// Log the adjustment for transparency (debug level)
				logger.Log.Debug().
					Str("impID", impID).
					Str("bidder", bids[i].BidderCode).
					Float64("original_price", originalPrice).
					Float64("multiplier", multiplier).
					Float64("adjusted_price", adjustedPrice).
					Float64("platform_cut", platformCut).
					Msg("Applied bid multiplier")

				// Record margin metrics
				if publisherID != "" {
					e.configMu.RLock()
					if e.metrics != nil {
						e.metrics.RecordMargin(publisherID, bids[i].BidderCode, mediaType, originalPrice, adjustedPrice, platformCut)
					}
					e.configMu.RUnlock()
				}

				bids[i].Bid.Bid.Price = adjustedPrice
			}
		}
	}

	return bidsByImp
}

// extractBidMultiplier safely extracts BidMultiplier field from any struct
func extractBidMultiplier(v interface{}) (float64, bool) {
	// Type assert to common publisher interface patterns
	type bidMultiplierGetter interface {
		GetBidMultiplier() float64
	}

	if getter, ok := v.(bidMultiplierGetter); ok {
		return getter.GetBidMultiplier(), true
	}

	// Direct type assertion for storage.Publisher (avoids expensive JSON round-trip)
	type publisherWithMultiplier interface {
		GetBidMultiplier() float64
	}
	if p, ok := v.(publisherWithMultiplier); ok {
		return p.GetBidMultiplier(), true
	}

	// Try concrete struct with BidMultiplier field via reflection-free approach
	type hasBidMultiplier struct {
		BidMultiplier float64
	}
	if p, ok := v.(*hasBidMultiplier); ok {
		return p.BidMultiplier, true
	}

	return 0, false
}

// extractPublisherID safely extracts PublisherID field from publisher struct
func extractPublisherID(v interface{}) (string, bool) {
	// Type assert to interface with GetPublisherID method (avoids expensive JSON round-trip)
	type publisherIDGetter interface {
		GetPublisherID() string
	}
	if getter, ok := v.(publisherIDGetter); ok {
		id := getter.GetPublisherID()
		return id, id != ""
	}

	// Try concrete struct with PublisherID field
	type hasPublisherID struct {
		PublisherID string
	}
	if p, ok := v.(*hasPublisherID); ok {
		return p.PublisherID, p.PublisherID != ""
	}

	return "", false
}

// RunAuction executes the auction
func (e *Exchange) RunAuction(ctx context.Context, req *AuctionRequest) (*AuctionResponse, error) {
	startTime := time.Now()

	// P0-7: Validate required BidRequest fields per OpenRTB 2.x spec
	if req.BidRequest == nil {
		return nil, NewValidationError("invalid auction request: missing bid request")
	}
	if req.BidRequest.ID == "" {
		return nil, NewValidationError("invalid bid request: missing required field 'id'")
	}
	if len(req.BidRequest.Imp) == 0 {
		return nil, NewValidationError("invalid bid request: must have at least one impression")
	}

	// P1-2: Validate impression count early to prevent OOM from malicious requests
	// This check must happen BEFORE allocating maps based on impression count
	// Use configured limit instead of hardcoded constant to allow higher limits when needed
	maxImpressions := e.config.CloneLimits.MaxImpressionsPerRequest
	if len(req.BidRequest.Imp) > maxImpressions {
		return nil, NewValidationError("invalid bid request: too many impressions (max %d, got %d)",
			maxImpressions, len(req.BidRequest.Imp))
	}

	// P2-3: Validate Site/App/DOOH mutual exclusivity per OpenRTB 2.6 section 3.2.1
	hasSite := req.BidRequest.Site != nil
	hasApp := req.BidRequest.App != nil
	hasDOOH := req.BidRequest.DOOH != nil
	distChannels := 0
	if hasSite {
		distChannels++
	}
	if hasApp {
		distChannels++
	}
	if hasDOOH {
		distChannels++
	}
	if distChannels == 0 {
		return nil, NewValidationError("invalid bid request: must have one of 'site', 'app', or 'dooh' object (OpenRTB 2.6)")
	}
	if distChannels > 1 {
		return nil, NewValidationError("invalid bid request: must have only one of 'site', 'app', or 'dooh' objects (OpenRTB 2.6)")
	}

	// P1-NEW-2: Validate impression IDs are unique and non-empty per OpenRTB 2.5 section 3.2.4
	seenImpIDs := make(map[string]bool, len(req.BidRequest.Imp))
	for i, imp := range req.BidRequest.Imp {
		if imp.ID == "" {
			return nil, NewValidationError("invalid bid request: impression[%d] has empty id (required by OpenRTB 2.5)", i)
		}
		if seenImpIDs[imp.ID] {
			return nil, NewValidationError("invalid bid request: duplicate impression id %q (must be unique per OpenRTB 2.5)", imp.ID)
		}
		seenImpIDs[imp.ID] = true

		// P2-1: Validate impression has at least one media type per OpenRTB 2.5 section 3.2.4
		if imp.Banner == nil && imp.Video == nil && imp.Audio == nil && imp.Native == nil {
			return nil, NewValidationError("invalid bid request: impression[%d] has no media type (banner/video/audio/native required)", i)
		}

		// P1-NEW-4: Validate banner dimensions per OpenRTB 2.5 section 3.2.6
		if imp.Banner != nil {
			hasExplicitSize := imp.Banner.W > 0 && imp.Banner.H > 0
			hasFormat := len(imp.Banner.Format) > 0
			if !hasExplicitSize && !hasFormat {
				return nil, NewValidationError("invalid bid request: impression[%d] banner must have either w/h or format array (OpenRTB 2.5)", i)
			}
		}

		// Normalize and validate currency codes (uppercase per ISO 4217)
		if imp.BidFloorCur != "" {
			// Normalize to uppercase
			normalized := strings.ToUpper(imp.BidFloorCur)
			req.BidRequest.Imp[i].BidFloorCur = normalized

			// Validate against common currencies (optional - could be configurable)
			// For now, accept any 3-letter uppercase code
			if len(normalized) != 3 {
				return nil, NewValidationError("invalid bid request: impression[%d] has invalid currency code %q (must be 3-letter ISO 4217)", i, imp.BidFloorCur)
			}
		}
	}

	response := &AuctionResponse{
		BidderResults: make(map[string]*BidderResult),
		DebugInfo: &DebugInfo{
			RequestTime:     startTime,
			BidderLatencies: make(map[string]time.Duration),
			Errors:          make(map[string][]string),
		},
	}

	// Validate the bid request per OpenRTB 2.x specification
	if validationErr := ValidateRequest(req.BidRequest); validationErr != nil {
		response.DebugInfo.TotalLatency = time.Since(startTime)
		return response, validationErr
	}

	// Get timeout from request or config
	// P1-NEW-1: Validate TMax bounds to prevent abuse
	timeout := req.Timeout
	if timeout == 0 && req.BidRequest.TMax > 0 {
		tmax := req.BidRequest.TMax
		// Cap TMax at reasonable maximum to prevent resource exhaustion
		if tmax > maxAllowedTMax {
			tmax = maxAllowedTMax
		}
		timeout = time.Duration(tmax) * time.Millisecond
	}
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get available bidders from static registry
	availableBidders := e.registry.ListEnabledBidders()

	// Snapshot config-protected fields under lock for consistent view during auction
	e.configMu.RLock()
	fpdProcessor := e.fpdProcessor
	eidFilter := e.eidFilter
	e.configMu.RUnlock()

	if len(availableBidders) == 0 {
		response.BidResponse = e.buildEmptyResponse(req.BidRequest, openrtb.NoBidNoBiddersAvailable)
		return response, nil
	}

	// Run IDR selection if enabled
	selectedBidders := availableBidders
	if e.idrClient != nil && e.config.IDREnabled {
		idrStart := time.Now()

		// P1-15: Build minimal request to reduce payload size
		minReq := e.buildMinimalIDRRequest(req.BidRequest)
		idrResult, err := e.idrClient.SelectPartnersMinimal(ctx, minReq, availableBidders)

		response.DebugInfo.IDRLatency = time.Since(idrStart)

		if err == nil && idrResult != nil {
			response.IDRResult = idrResult
			selectedBidders = make([]string, 0, len(idrResult.SelectedBidders))
			for _, sb := range idrResult.SelectedBidders {
				selectedBidders = append(selectedBidders, sb.BidderCode)
			}

			for _, eb := range idrResult.ExcludedBidders {
				response.DebugInfo.ExcludedBidders = append(response.DebugInfo.ExcludedBidders, eb.BidderCode)
			}
		}
		// If IDR fails, fall back to all bidders
	}

	selectedBidders = filterBiddersWithImpExt(req.BidRequest.Imp, selectedBidders)

	response.DebugInfo.SelectedBidders = selectedBidders

	logger.Log.Debug().
		Strs("selected_bidders", selectedBidders).
		Int("count", len(selectedBidders)).
		Msg("Bidders selected for auction")

	// Process FPD and filter EIDs (using snapshotted processor/filter for consistency)
	var bidderFPD fpd.BidderFPD
	if fpdProcessor != nil {
		// Filter EIDs first
		if eidFilter != nil {
			eidFilter.ProcessRequestEIDs(req.BidRequest)
		}

		// Process FPD for each bidder
		var err error
		bidderFPD, err = fpdProcessor.ProcessRequest(req.BidRequest, selectedBidders)
		if err != nil {
			// Log error but continue - FPD is not critical
			response.DebugInfo.AddError("fpd", []string{err.Error()})
		}
	}


	// CP-2: EID field mapping audit — detect UIDs orphaned in user.ext.eids
	if req.BidRequest.User != nil {
		// Index top-level EIDs by source
		topLevelSources := make(map[string]int) // source → uid count
		for _, eid := range req.BidRequest.User.EIDs {
			topLevelSources[eid.Source] = len(eid.UIDs)
		}

		// Walk ext.eids and cross-reference
		var extEIDsRaw []struct {
			Source string            `json:"source"`
			UIDs   []json.RawMessage `json:"uids"`
		}
		if len(req.BidRequest.User.Ext) > 0 {
			var userExt struct {
				EIDs json.RawMessage `json:"eids"`
			}
			if err := json.Unmarshal(req.BidRequest.User.Ext, &userExt); err == nil && len(userExt.EIDs) > 0 {
				_ = json.Unmarshal(userExt.EIDs, &extEIDsRaw)
			}
		}

		extSources := make(map[string]int)
		for _, eid := range extEIDsRaw {
			extSources[eid.Source] = len(eid.UIDs)
		}

		// Log per source — flag orphans
		allSources := make(map[string]struct{})
		for s := range topLevelSources {
			allSources[s] = struct{}{}
		}
		for s := range extSources {
			allSources[s] = struct{}{}
		}

		for source := range allSources {
			inTop := topLevelSources[source]
			inExt := extSources[source]
			location := "user.eids"
			if inTop > 0 && inExt > 0 {
				location = "both"
			}
			if inTop == 0 && inExt > 0 {
				location = "user.ext.eids"
			}

			logEvent := logger.Log.Debug()
			if location == "user.ext.eids" {
				logEvent = logger.Log.Warn()
			}
			logEvent.
				Str("request_id", req.BidRequest.ID).
				Str("eid_source", source).
				Int("uid_count_top", inTop).
				Int("uid_count_ext", inExt).
				Str("location", location).
				Msg("CP-2: EID mapping")
		}
	}

	// Call bidders in parallel
	results := e.callBiddersWithFPD(ctx, req.BidRequest, selectedBidders, timeout, bidderFPD)

	// Extract request context for event recording
	var country, deviceType, mediaType, adSize, publisherID string
	if req.BidRequest.Device != nil && req.BidRequest.Device.Geo != nil {
		country = req.BidRequest.Device.Geo.Country
	}
	if req.BidRequest.Device != nil {
		switch req.BidRequest.Device.DeviceType {
		case 1:
			deviceType = "mobile"
		case 2:
			deviceType = "desktop"
		case 3:
			deviceType = "ctv"
		default:
			deviceType = "unknown"
		}
	}
	if len(req.BidRequest.Imp) > 0 {
		imp := req.BidRequest.Imp[0]
		if imp.Banner != nil {
			mediaType = "banner"
			if imp.Banner.W > 0 && imp.Banner.H > 0 {
				adSize = fmt.Sprintf("%dx%d", imp.Banner.W, imp.Banner.H)
			}
		} else if imp.Video != nil {
			mediaType = "video"
		} else if imp.Native != nil {
			mediaType = "native"
		}
	}
	if req.BidRequest.Site != nil && req.BidRequest.Site.Publisher != nil {
		publisherID = req.BidRequest.Site.Publisher.ID
	}

	// P1-2: Check context deadline before expensive validation work
	// If we've already timed out, return early with whatever we have
	select {
	case <-ctx.Done():
		response.DebugInfo.TotalLatency = time.Since(startTime)
		response.BidResponse = e.buildEmptyResponse(req.BidRequest, openrtb.NoBidTimeout)
		return response, nil // Return empty response rather than error on timeout
	default:
		// Context still valid, proceed with validation
	}

	// Build impression floor map for bid validation (with multiplier applied to floors)
	impFloors := e.buildImpFloorMap(ctx, req.BidRequest)

	// Build impression map for O(1) lookups during bid validation
	impMap := adapters.BuildImpMap(req.BidRequest.Imp)

	// Track seen bid IDs for deduplication
	seenBidIDs := make(map[string]struct{})

	// Collect and validate all bids using pooled slices to reduce GC pressure
	validBidsPtr := getValidBidsSlice()
	defer putValidBidsSlice(validBidsPtr)
	validBids := *validBidsPtr

	validationErrorsPtr := getValidationErrorsSlice()
	defer putValidationErrorsSlice(validationErrorsPtr)
	validationErrors := *validationErrorsPtr

	// Collect results
	for bidderCode, result := range results {
		response.BidderResults[bidderCode] = result
		response.DebugInfo.BidderLatencies[bidderCode] = result.Latency

		// Record bidder request metrics
		if e.metrics != nil {
			hasError := len(result.Errors) > 0
			e.metrics.RecordBidderRequest(bidderCode, result.Latency, hasError, result.TimedOut)
		}

		// Log bidder response status for timeout monitoring
		hadBids := len(result.Bids) > 0
		hadErrors := len(result.Errors) > 0

		logEvent := logger.Log.Info().
			Str("bidder", bidderCode).
			Dur("latency_ms", result.Latency).
			Bool("timed_out", result.TimedOut).
			Bool("had_bids", hadBids).
			Bool("had_errors", hadErrors)

		if hadBids {
			logEvent.Int("bid_count", len(result.Bids))
		}

		if result.TimedOut {
			logEvent.Msg("Bidder timeout detected")
		} else if !hadBids && !hadErrors {
			logEvent.Msg("Bidder returned no bids (no error)")
		} else if hadBids {
			logEvent.Msg("Bidder responded with bids")
		} else if hadErrors {
			logEvent.Msg("Bidder failed with errors")
		}

		if len(result.Errors) > 0 {
			errStrs := make([]string, len(result.Errors))
			for i, err := range result.Errors {
				errStrs[i] = err.Error()
			}
			response.DebugInfo.AddError(bidderCode, errStrs)
		}

		// Record event to IDR
		if e.eventRecorder != nil {
			hadBid := len(result.Bids) > 0
			var bidCPM *float64
			if hadBid && len(result.Bids) > 0 {
				cpm := result.Bids[0].Bid.Price
				bidCPM = &cpm
			}
			hadError := len(result.Errors) > 0
			var errorMsg string
			if hadError {
				// P2-7: Aggregate all errors instead of just the first
				if len(result.Errors) == 1 {
					errorMsg = result.Errors[0].Error()
				} else {
					errMsgs := make([]string, len(result.Errors))
					for i, err := range result.Errors {
						errMsgs[i] = err.Error()
					}
					errorMsg = fmt.Sprintf("%d errors: %s", len(result.Errors), strings.Join(errMsgs, "; "))
				}
			}

			e.eventRecorder.RecordBidResponse(
				req.BidRequest.ID,
				bidderCode,
				float64(result.Latency.Milliseconds()),
				hadBid,
				bidCPM,
				nil, // floor price
				country,
				deviceType,
				mediaType,
				adSize,
				publisherID,
				result.TimedOut, // P2-2: use actual timeout status
				hadError,
				errorMsg,
			)
		}

		// Validate and deduplicate bids
		for _, tb := range result.Bids {
			// Skip nil bids
			if tb == nil || tb.Bid == nil {
				continue
			}

			// Record bid received metric
			if e.metrics != nil {
				e.metrics.RecordBid(bidderCode, mediaType, tb.Bid.Price)
			}

			// Validate bid
			if validErr := e.validateBid(tb.Bid, bidderCode, req.BidRequest, impMap, impFloors); validErr != nil {
				// P3-1: Log bid validation failures for debugging
				logger.Log.Debug().
					Str("bidder", bidderCode).
					Str("bidID", tb.Bid.ID).
					Str("impID", tb.Bid.ImpID).
					Float64("price", tb.Bid.Price).
					Err(validErr).
					Msg("bid validation failed")
				validationErrors = append(validationErrors, validErr) //nolint:staticcheck
				response.DebugInfo.AppendError(bidderCode, validErr.Error())
				continue
			}

			// Check for duplicate bid IDs
			if _, seen := seenBidIDs[tb.Bid.ID]; seen {
				dupErr := &BidValidationError{
					BidID:      tb.Bid.ID,
					ImpID:      tb.Bid.ImpID,
					BidderCode: bidderCode,
					Reason:     "duplicate bid ID",
				}
				validationErrors = append(validationErrors, dupErr) //nolint:staticcheck
				response.DebugInfo.AppendError(bidderCode, dupErr.Error())
				continue
			}
			seenBidIDs[tb.Bid.ID] = struct{}{}

			// Add to valid bids with demand type
			validBids = append(validBids, ValidatedBid{
				Bid:        tb,
				BidderCode: bidderCode,
				DemandType: e.getDemandType(bidderCode),
			})
		}
	}

	// Apply auction logic (first-price or second-price)
	auctionedBids := e.runAuctionLogic(validBids, impFloors)

	// Apply multiformat bid selection if enabled
	if e.mfProcessor != nil && e.mfProcessor.config.Enabled {
		auctionedBids = e.applyMultiformatSelection(req.BidRequest, auctionedBids)
	}

	// Apply bid multiplier if publisher is configured with one
	auctionedBids = e.applyBidMultiplier(ctx, auctionedBids)

	// Build seat bids with demand type obfuscation:
	// - Platform demand: aggregated into single "thenexusengine" seat (highest bid per impression)
	// - Publisher demand: shown transparently with original bidder codes
	seatBidMap := make(map[string]*openrtb.SeatBid)

	for _, impBids := range auctionedBids {
		// Separate platform and publisher bids for this impression
		var platformBids []ValidatedBid
		var publisherBids []ValidatedBid

		for _, vb := range impBids {
			if vb.DemandType == adapters.DemandTypePublisher {
				publisherBids = append(publisherBids, vb)
			} else {
				// Default to platform for obfuscation
				platformBids = append(platformBids, vb)
			}
		}

		// Add highest platform bid to "thenexusengine" seat (obfuscated)
		if len(platformBids) > 0 {
			// Find highest CPM platform bid for this impression
			highestPlatformBid := platformBids[0]
			for _, vb := range platformBids[1:] {
				if vb.Bid.Bid.Price > highestPlatformBid.Bid.Bid.Price {
					highestPlatformBid = vb
				}
			}

			// Get or create the thenexusengine seat
			nexusSeat, ok := seatBidMap[adapters.PlatformSeatName]
			if !ok {
				nexusSeat = &openrtb.SeatBid{
					Seat: adapters.PlatformSeatName,
					Bid:  []openrtb.Bid{},
				}
				seatBidMap[adapters.PlatformSeatName] = nexusSeat
			}

			// Create obfuscated bid with "thenexusengine" branding in targeting
			bid := *highestPlatformBid.Bid.Bid
			bidExt := e.buildBidExtension(highestPlatformBid)
			if extBytes, err := json.Marshal(bidExt); err == nil {
				bid.Ext = extBytes
			}
			nexusSeat.Bid = append(nexusSeat.Bid, bid)
		}

		// Add all publisher bids transparently
		for _, vb := range publisherBids {
			sb, ok := seatBidMap[vb.BidderCode]
			if !ok {
				sb = &openrtb.SeatBid{
					Seat: vb.BidderCode,
					Bid:  []openrtb.Bid{},
				}
				seatBidMap[vb.BidderCode] = sb
			}

			// Create bid copy with Prebid extension for targeting
			bid := *vb.Bid.Bid
			bidExt := e.buildBidExtension(vb)
			if extBytes, err := json.Marshal(bidExt); err == nil {
				bid.Ext = extBytes
			}
			sb.Bid = append(sb.Bid, bid)
		}
	}

	// Convert seat bid map to slice
	allBids := make([]openrtb.SeatBid, 0, len(seatBidMap))
	for _, sb := range seatBidMap {
		allBids = append(allBids, *sb)
	}

	// Build response
	response.BidResponse = &openrtb.BidResponse{
		ID:      req.BidRequest.ID,
		SeatBid: allBids,
		Cur:     e.config.DefaultCurrency,
	}

	response.DebugInfo.TotalLatency = time.Since(startTime)

	// P3-1: Log auction completion with summary stats
	totalBids := 0
	for _, sb := range allBids {
		totalBids += len(sb.Bid)
	}

	// Calculate timeout and response statistics
	timedOutCount := 0
	emptyResponseCount := 0
	bidsReceivedCount := 0
	errorCount := 0
	for _, result := range response.BidderResults {
		if result.TimedOut {
			timedOutCount++
		} else if len(result.Bids) > 0 {
			bidsReceivedCount++
		} else if len(result.Errors) > 0 {
			errorCount++
		} else {
			emptyResponseCount++
		}
	}

	logger.Log.Info().
		Str("requestID", req.BidRequest.ID).
		Int("bidders_total", len(selectedBidders)).
		Int("bidders_with_bids", bidsReceivedCount).
		Int("bidders_timed_out", timedOutCount).
		Int("bidders_empty", emptyResponseCount).
		Int("bidders_error", errorCount).
		Int("impressions", len(req.BidRequest.Imp)).
		Int("total_bids", totalBids).
		Dur("total_latency", response.DebugInfo.TotalLatency).
		Msg("Auction completed")

	// Record auction metrics
	if e.metrics != nil {
		// Determine auction status
		auctionStatus := "success"
		if totalBids == 0 {
			auctionStatus = "no_bids"
		}

		// Get media type from first impression (already extracted earlier)
		// Use the mediaType variable from line 1018

		// Record auction completion
		e.metrics.RecordAuction(auctionStatus, mediaType, response.DebugInfo.TotalLatency, len(selectedBidders), 0)
	}

	// NEW: Log analytics object with rich auction transaction data
	if e.analytics != nil {
		auctionObj := e.buildAuctionObject(
			ctx,
			req,
			response,
			selectedBidders,
			availableBidders,
			startTime,
		)

		// Non-blocking analytics call - errors logged internally by analytics module
		if err := e.analytics.LogAuctionObject(ctx, auctionObj); err != nil {
			logger.Log.Warn().
				Err(err).
				Str("auction_id", req.BidRequest.ID).
				Msg("Failed to log auction analytics")
		}
	}

	return response, nil
}

// buildAuctionObject constructs a complete analytics.AuctionObject from auction results
// This provides rich transaction data for multi-sink analytics (IDR, DataDog, BigQuery, etc.)
func (e *Exchange) buildAuctionObject(
	ctx context.Context,
	req *AuctionRequest,
	resp *AuctionResponse,
	selectedBidders []string,
	availableBidders []string,
	startTime time.Time,
) *analytics.AuctionObject {
	// Extract publisher ID
	publisherID := ""
	if req.BidRequest.Site != nil && req.BidRequest.Site.Publisher != nil {
		publisherID = req.BidRequest.Site.Publisher.ID
	} else if req.BidRequest.App != nil && req.BidRequest.App.Publisher != nil {
		publisherID = req.BidRequest.App.Publisher.ID
	}

	// Extract publisher domain
	publisherDomain := ""
	if req.BidRequest.Site != nil {
		publisherDomain = req.BidRequest.Site.Domain
	} else if req.BidRequest.App != nil {
		publisherDomain = req.BidRequest.App.Bundle
	}

	// Build excluded bidders map
	excludedBidders := make(map[string]analytics.ExclusionReason)
	selectedMap := make(map[string]bool)
	for _, bidder := range selectedBidders {
		selectedMap[bidder] = true
	}
	for _, bidder := range availableBidders {
		if !selectedMap[bidder] {
			// Check if circuit breaker was reason
			breaker := e.getBidderCircuitBreaker(bidder)
			if breaker != nil && breaker.IsOpen() {
				excludedBidders[bidder] = analytics.ExclusionReason{
					Code:    "circuit_breaker_open",
					Message: "Circuit breaker is open",
				}
			} else if resp.IDRResult != nil {
				// Check if IDR excluded this bidder
				for _, eb := range resp.IDRResult.ExcludedBidders {
					if eb.BidderCode == bidder {
						excludedBidders[bidder] = analytics.ExclusionReason{
							Code:    "idr_excluded",
							Message: eb.Reason,
						}
						break
					}
				}
			}
		}
	}

	// Convert bidder results to analytics format
	analyticsResults := make(map[string]*analytics.BidderResult)
	for bidderCode, result := range resp.BidderResults {
		bids := make([]analytics.BidDetails, 0, len(result.Bids))
		for _, tb := range result.Bids {
			if tb != nil && tb.Bid != nil {
				bids = append(bids, analytics.BidDetails{
					BidID:         tb.Bid.ID,
					ImpID:         tb.Bid.ImpID,
					OriginalPrice: tb.Bid.Price,
					AdjustedPrice: tb.Bid.Price, // Will be adjusted if multiplier applied
					Currency:      result.Currency,
					ADomain:       tb.Bid.ADomain,
					CreativeID:    tb.Bid.CRID,
					DemandType:    string(e.getDemandType(bidderCode)),
				})
			}
		}

		noBidReason := ""
		if len(result.Errors) > 0 && len(result.Bids) == 0 {
			noBidReason = result.Errors[0].Error()
		}

		analyticsResults[bidderCode] = &analytics.BidderResult{
			BidderCode:  bidderCode,
			Latency:     result.Latency,
			HttpStatus:  0, // Not easily accessible from current structure
			Bids:        bids,
			SeatBids:    len(result.Bids),
			TimedOut:    result.TimedOut,
			NoBidReason: noBidReason,
			Errors:      extractErrorStrings(result.Errors),
		}
	}

	// Extract winning bids from response
	winningBids := make([]analytics.WinningBid, 0)
	if resp.BidResponse != nil {
		for _, seatBid := range resp.BidResponse.SeatBid {
			for _, bid := range seatBid.Bid {
				winningBids = append(winningBids, analytics.WinningBid{
					BidID:         bid.ID,
					ImpID:         bid.ImpID,
					BidderCode:    seatBid.Seat,
					OriginalPrice: bid.Price,
					AdjustedPrice: bid.Price,
					PlatformCut:   0, // Calculate if multiplier applied
					Currency:      resp.BidResponse.Cur,
					ADomain:       bid.ADomain,
					CreativeID:    bid.CRID,
					DemandType:    "", // Extract from bid extension if needed
					ClearPrice:    bid.Price,
				})
			}
		}
	}

	// Determine auction status
	status := "success"
	if len(winningBids) == 0 {
		status = "no_bids"
	} else if len(resp.DebugInfo.Errors) > 0 {
		status = "error"
	}

	// Convert impressions to analytics format
	impressions := make([]analytics.Impression, 0, len(req.BidRequest.Imp))
	for _, imp := range req.BidRequest.Imp {
		mediaTypes := []string{}
		sizes := []string{}

		if imp.Banner != nil {
			mediaTypes = append(mediaTypes, "banner")
			if imp.Banner.W > 0 && imp.Banner.H > 0 {
				sizes = append(sizes, fmt.Sprintf("%dx%d", imp.Banner.W, imp.Banner.H))
			}
		}
		if imp.Video != nil {
			mediaTypes = append(mediaTypes, "video")
		}
		if imp.Native != nil {
			mediaTypes = append(mediaTypes, "native")
		}
		if imp.Audio != nil {
			mediaTypes = append(mediaTypes, "audio")
		}

		impressions = append(impressions, analytics.Impression{
			ID:         imp.ID,
			TagID:      imp.TagID,
			MediaTypes: mediaTypes,
			Sizes:      sizes,
			Floor:      imp.BidFloor,
		})
	}

	// Extract device info
	var deviceInfo *analytics.DeviceInfo
	if req.BidRequest.Device != nil {
		deviceType := "unknown"
		switch req.BidRequest.Device.DeviceType {
		case 1:
			deviceType = "mobile"
		case 2:
			deviceType = "desktop"
		case 3:
			deviceType = "ctv"
		}

		country := ""
		region := ""
		if req.BidRequest.Device.Geo != nil {
			country = req.BidRequest.Device.Geo.Country
			region = req.BidRequest.Device.Geo.Region
		}

		deviceInfo = &analytics.DeviceInfo{
			Type:    deviceType,
			Country: country,
			Region:  region,
			IP:      req.BidRequest.Device.IP,
			UA:      req.BidRequest.Device.UA,
		}
	}

	// Extract user info (privacy-safe)
	var userInfo *analytics.UserInfo
	if req.BidRequest.User != nil {
		userInfo = &analytics.UserInfo{
			BuyerUID: req.BidRequest.User.BuyerUID,
			HasEIDs:  len(req.BidRequest.User.EIDs) > 0,
		}
	}

	// Extract GDPR data
	var gdprData *analytics.GDPRData
	if req.BidRequest.Regs != nil && req.BidRequest.Regs.GDPR != nil {
		gdprData = &analytics.GDPRData{
			Applies: *req.BidRequest.Regs.GDPR == 1,
		}
		if req.BidRequest.User != nil {
			gdprData.ConsentString = req.BidRequest.User.Consent
		}
	}

	// Extract CCPA data
	var ccpaData *analytics.CCPAData
	if req.BidRequest.Regs != nil {
		usPrivacy := ""

		// Check top-level USPrivacy field first (OpenRTB 2.5)
		if req.BidRequest.Regs.USPrivacy != "" {
			usPrivacy = req.BidRequest.Regs.USPrivacy
		} else if req.BidRequest.Regs.Ext != nil {
			// Fallback to ext location (older implementations)
			var regsExt struct {
				USPrivacy string `json:"us_privacy"`
			}
			if err := json.Unmarshal(req.BidRequest.Regs.Ext, &regsExt); err == nil {
				usPrivacy = regsExt.USPrivacy
			}
		}

		// Parse US Privacy string if present
		if usPrivacy != "" && len(usPrivacy) >= 4 {
			// Format: "1YNN" where position 2 is opt-out flag
			ccpaData = &analytics.CCPAData{
				Applies:   true,
				OptOut:    usPrivacy[2:3] == "Y",  // Position 2 (0-indexed)
				USPrivacy: usPrivacy,
			}
		}
	}

	// Count total bids
	totalBids := 0
	for _, result := range resp.BidderResults {
		totalBids += len(result.Bids)
	}

	return &analytics.AuctionObject{
		AuctionID:        req.BidRequest.ID,
		RequestID:        req.BidRequest.ID,
		PublisherID:      publisherID,
		PublisherDomain:  publisherDomain,
		Timestamp:        startTime,
		Impressions:      impressions,
		Device:           deviceInfo,
		User:             userInfo,
		TMax:             req.BidRequest.TMax,
		Currency:         e.config.DefaultCurrency,
		Test:             req.BidRequest.Test == 1,
		SelectedBidders:  selectedBidders,
		ExcludedBidders:  excludedBidders,
		TotalBidders:     len(availableBidders),
		BidderResults:    analyticsResults,
		WinningBids:      winningBids,
		TotalBids:        totalBids,
		AuctionDuration:  time.Since(startTime),
		Status:           status,
		BidMultiplier:    1.0, // TODO: Extract from context if multiplier applied
		FloorAdjustments: make(map[string]float64),
		TotalRevenue:     0, // TODO: Calculate platform cuts
		TotalPayout:      0, // TODO: Calculate publisher payouts
		GDPR:             gdprData,
		CCPA:             ccpaData,
		COPPA:            req.BidRequest.Regs != nil && req.BidRequest.Regs.COPPA == 1,
		ConsentOK:        e.checkConsentOK(ctx),
		ValidationErrors: []analytics.ValidationError{},
		RequestErrors:    []string{},
		BidderErrors:     extractBidderErrors(resp.DebugInfo),
	}
}

// extractErrorStrings converts []error to []string
func extractErrorStrings(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}
	strs := make([]string, len(errs))
	for i, err := range errs {
		strs[i] = err.Error()
	}
	return strs
}

// extractBidderErrors converts debug info errors to map
func extractBidderErrors(debug *DebugInfo) map[string][]string {
	if debug == nil || len(debug.Errors) == 0 {
		return nil
	}
	return debug.Errors
}

// checkConsentOK determines if consent is valid based on privacy context
func (e *Exchange) checkConsentOK(ctx context.Context) bool {
	// Check GDPR consent
	if middleware.GDPRApplies(ctx) {
		if !middleware.GDPRConsentValidated(ctx) {
			return false
		}
	}

	// Check CCPA opt-out
	if middleware.CCPAOptOut(ctx) {
		return false
	}

	return true
}

// callBiddersWithFPD calls all selected bidders in parallel with FPD support
// P0-1: Uses sync.Map for thread-safe result collection
// P0-4: Uses semaphore to limit concurrent bidder goroutines
func (e *Exchange) callBiddersWithFPD(ctx context.Context, req *openrtb.BidRequest, bidders []string, timeout time.Duration, bidderFPD fpd.BidderFPD) map[string]*BidderResult {
	var results sync.Map // P0-1: Thread-safe map for concurrent writes
	var wg sync.WaitGroup

	// P0-4: Create semaphore to limit concurrent bidder calls (0 = unlimited)
	maxConcurrent := e.config.MaxConcurrentBidders
	var sem chan struct{}
	if maxConcurrent > 0 {
		sem = make(chan struct{}, maxConcurrent)
	}
	// If maxConcurrent <= 0, sem remains nil (unlimited concurrency)

	for _, bidderCode := range bidders {
		logger.Log.Debug().
			Str("bidder", bidderCode).
			Msg("Processing bidder in auction")

		// Check circuit breaker before calling bidder
		breaker := e.getBidderCircuitBreaker(bidderCode)
		if breaker != nil && breaker.IsOpen() {
			// Circuit breaker is open - skip this bidder
			result := &BidderResult{
				BidderCode: bidderCode,
				Errors:     []error{fmt.Errorf("circuit breaker open")},
				TimedOut:   false, // NOT a timeout - circuit breaker state
			}
			results.Store(bidderCode, result)

			// Record rejected request metric
			if e.metrics != nil {
				e.metrics.RecordBidderCircuitRejected(bidderCode)
			}

			logger.Log.Debug().
				Str("bidder_code", bidderCode).
				Msg("Skipping bidder - circuit breaker OPEN")

			continue // Don't launch goroutine
		}

		// Try static registry first
		adapterWithInfo, ok := e.registry.Get(bidderCode)
		if ok {
			// Filter bidders by inventory type capability (OpenRTB 2.6)
			if !bidderSupportsInventoryType(req, adapterWithInfo) {
				logger.Log.Debug().
					Str("bidder", bidderCode).
					Str("inventory_type", string(inventoryTypeLabel(req))).
					Msg("Skipping bidder - does not support inventory type")
				continue
			}

			wg.Add(1)
			go func(code string, awi adapters.AdapterWithInfo) {
				defer wg.Done()

				// P0-4: Acquire semaphore if concurrency limit is configured
				if sem != nil {
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }() // Release on completion
					case <-ctx.Done():
						// Context canceled while waiting for semaphore
						results.Store(code, &BidderResult{
							BidderCode: code,
							Errors:     []error{ctx.Err()},
							TimedOut:   true,
						})
						return
					}
				}

				// Check geo-aware consent filtering (GDPR, CCPA, etc.)
				gvlID := awi.Info.GVLVendorID
				if middleware.ShouldFilterBidderByGeo(req, gvlID) {
					// Detect which regulation applies
					regulation := middleware.RegulationNone
					if req.Device != nil && req.Device.Geo != nil {
						regulation = middleware.DetectRegulationFromGeo(req.Device.Geo)
					}

					logger.Log.Info().
						Str("bidder", code).
						Int("gvl_id", gvlID).
						Str("request_id", req.ID).
						Str("regulation", string(regulation)).
						Str("country", func() string {
							if req.Device != nil && req.Device.Geo != nil {
								return req.Device.Geo.Country
							}
							return ""
						}()).
						Str("region", func() string {
							if req.Device != nil && req.Device.Geo != nil {
								return req.Device.Geo.Region
							}
							return ""
						}()).
						Msg("Skipping bidder - no consent for user's geographic location")

					results.Store(code, &BidderResult{
						BidderCode: code,
						Errors:     []error{fmt.Errorf("no %s consent for vendor %d", regulation, gvlID)},
					})
					return
				}

				// Clone request and apply bidder-specific FPD
				publisherID := middleware.PublisherIDFromContext(ctx)
				bidderReq := e.cloneRequestWithFPD(req, code, bidderFPD, publisherID)

				result := e.callBidder(ctx, bidderReq, code, awi.Adapter, timeout)

				// Record result in circuit breaker
				breaker := e.getBidderCircuitBreaker(code)
				if breaker != nil {
					// Record request metric
					if e.metrics != nil {
						e.metrics.RecordBidderCircuitRequest(code)
					}

					if len(result.Errors) > 0 || result.TimedOut {
						breaker.RecordFailure()
						// Record failure metric
						if e.metrics != nil {
							e.metrics.RecordBidderCircuitFailure(code)
						}
					} else {
						// Any valid response (including no-bid) counts as success
						breaker.RecordSuccess()
						// Record success metric
						if e.metrics != nil {
							e.metrics.RecordBidderCircuitSuccess(code)
						}
					}
				}

				results.Store(code, result) // P0-1: Thread-safe store
			}(bidderCode, adapterWithInfo)
			continue
		}
	}

	wg.Wait()

	// P0-1: Convert sync.Map to regular map for return
	finalResults := make(map[string]*BidderResult)
	results.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.(*BidderResult); ok {
				finalResults[k] = v
			}
		}
		return true
	})
	// CP-5: Single auction summary -- one grep-able record per auction
	type sspResult struct {
		Bidder          string  `json:"bidder"`
		Status          int     `json:"status"`
		LatencyMS       float64 `json:"latency_ms"`
		HadBids         bool    `json:"had_bids"`
		RejectionReason string  `json:"rejection_reason,omitempty"`
	}
	summary := make([]sspResult, 0, len(finalResults))
	totalBids := 0
	for bidder, r := range finalResults {
		summary = append(summary, sspResult{
			Bidder:          bidder,
			Status:          r.LastStatusCode,
			LatencyMS:       r.Latency.Seconds() * 1000,
			HadBids:         len(r.Bids) > 0,
			RejectionReason: r.RejectionReason,
		})
		totalBids += len(r.Bids)
	}
	summaryJSON, _ := json.Marshal(summary)
	logger.Log.Info().
		Str("request_id", req.ID).
		Str("domain", func() string {
			if req.Site != nil {
				return req.Site.Domain
			}
			return ""
		}()).
		Int("bidders_fired", len(finalResults)).
		Int("total_bids", totalBids).
		RawJSON("ssp_results", summaryJSON).
		Msg("CP-5: Auction summary")

	return finalResults
}

// cloneRequestWithFPD creates a selective copy of the request with bidder-specific FPD applied
// and enforces USD currency for all bid requests.
// PERF: Only clones fields that are modified (Cur, Imp, Site/App/User if FPD applies).
// Deep copies Device, Regs, Source to prevent cross-bidder data races.
func (e *Exchange) cloneRequestWithFPD(req *openrtb.BidRequest, bidderCode string, bidderFPD fpd.BidderFPD, publisherID string) *openrtb.BidRequest {
	// Shallow copy of top-level struct
	clone := *req

	// Clone Cur slice (we overwrite it)
	// Only set currency if DefaultCurrency is configured, otherwise leave nil to accept any currency
	if e.config.DefaultCurrency != "" {
		clone.Cur = []string{e.config.DefaultCurrency}
	} else {
		clone.Cur = nil
	}

	// Deep copy Device to prevent adapter mutations from affecting other bidders
	if req.Device != nil {
		deviceCopy := *req.Device
		if req.Device.Geo != nil {
			geoCopy := *req.Device.Geo
			deviceCopy.Geo = &geoCopy
		}
		// Deep copy SUA (2.6: Structured User-Agent)
		if req.Device.SUA != nil {
			suaCopy := *req.Device.SUA
			if len(req.Device.SUA.Browsers) > 0 {
				suaCopy.Browsers = make([]openrtb.BrandVersion, len(req.Device.SUA.Browsers))
				copy(suaCopy.Browsers, req.Device.SUA.Browsers)
			}
			if req.Device.SUA.Platform != nil {
				platCopy := *req.Device.SUA.Platform
				suaCopy.Platform = &platCopy
			}
			if req.Device.SUA.Mobile != nil {
				mobileCopy := *req.Device.SUA.Mobile
				suaCopy.Mobile = &mobileCopy
			}
			deviceCopy.SUA = &suaCopy
		}
		clone.Device = &deviceCopy
	}

	// Deep copy Regs to prevent adapter mutations from affecting other bidders
	if req.Regs != nil {
		regsCopy := *req.Regs
		clone.Regs = &regsCopy
	}

	// Deep copy Source to prevent adapter mutations from affecting other bidders
	if req.Source != nil {
		sourceCopy := *req.Source
		if req.Source.SChain != nil {
			schainCopy := *req.Source.SChain
			if len(req.Source.SChain.Nodes) > 0 {
				// Bounded allocation for supply chain nodes
				limits := e.config.CloneLimits
				nodeCount := len(req.Source.SChain.Nodes)
				if nodeCount > limits.MaxSChainNodes {
					nodeCount = limits.MaxSChainNodes
				}
				schainCopy.Nodes = make([]openrtb.SupplyChainNode, nodeCount)
				copy(schainCopy.Nodes, req.Source.SChain.Nodes[:nodeCount])
			}
			sourceCopy.SChain = &schainCopy
		}
		clone.Source = &sourceCopy
	}

	// Clone Imp slice - deep copy to prevent bidder modifications from corrupting original request
	if len(req.Imp) > 0 {
		limits := e.config.CloneLimits
		impCount := len(req.Imp)
		if impCount > limits.MaxImpressionsPerRequest {
			impCount = limits.MaxImpressionsPerRequest
		}
		clone.Imp = make([]openrtb.Imp, impCount)
		for i := 0; i < impCount; i++ {
			clone.Imp[i] = req.Imp[i] // Shallow copy of Imp struct

			// Strip all other SSP keys from imp.ext — each bidder only sees its own params.
			// This prevents Rubicon params leaking into PubMatic requests, etc.
			if len(clone.Imp[i].Ext) > 0 {
				clone.Imp[i].Ext = filterImpExtForBidder(clone.Imp[i].Ext, bidderCode)
			}

			// Ensure every imp has a floor — SSPs expect bidfloor+bidfloorcur together.
			if clone.Imp[i].BidFloor == 0 {
				clone.Imp[i].BidFloor = 0.10
			}
			if clone.Imp[i].BidFloorCur == "" {
				clone.Imp[i].BidFloorCur = e.config.DefaultCurrency
			}

			// Deep copy pointer fields to prevent data corruption (CVE-2026-XXXX)
			if req.Imp[i].Banner != nil {
				bannerCopy := *req.Imp[i].Banner
				clone.Imp[i].Banner = &bannerCopy
			}
			if req.Imp[i].Video != nil {
				videoCopy := *req.Imp[i].Video
				clone.Imp[i].Video = &videoCopy
			}
			if req.Imp[i].Audio != nil {
				audioCopy := *req.Imp[i].Audio
				clone.Imp[i].Audio = &audioCopy
			}
			if req.Imp[i].Native != nil {
				nativeCopy := *req.Imp[i].Native
				clone.Imp[i].Native = &nativeCopy
			}
			if req.Imp[i].PMP != nil {
				pmpCopy := *req.Imp[i].PMP
				clone.Imp[i].PMP = &pmpCopy
			}
			if req.Imp[i].Secure != nil {
				secureCopy := *req.Imp[i].Secure
				clone.Imp[i].Secure = &secureCopy
			}
			// Deep copy Qty (2.6: DOOH multiplier)
			if req.Imp[i].Qty != nil {
				qtyCopy := *req.Imp[i].Qty
				clone.Imp[i].Qty = &qtyCopy
			}
			// Deep copy Refresh (2.6: auto-refresh settings)
			if req.Imp[i].Refresh != nil {
				refreshCopy := *req.Imp[i].Refresh
				if len(req.Imp[i].Refresh.RefSettings) > 0 {
					refreshCopy.RefSettings = make([]openrtb.RefInfo, len(req.Imp[i].Refresh.RefSettings))
					copy(refreshCopy.RefSettings, req.Imp[i].Refresh.RefSettings)
				}
				clone.Imp[i].Refresh = &refreshCopy
			}
		}
	}

	// Check if FPD will be applied (requires cloning Site/App/User)
	var fpdData *fpd.ResolvedFPD
	if bidderFPD != nil {
		fpdData = bidderFPD[bidderCode]
	}
	hasFPD := fpdData != nil && e.fpdProcessor != nil

	// Always deep-copy Site+Publisher to prevent cross-bidder data races.
	// Without this, all clones share the same Publisher pointer. The Rubicon
	// adapter mutates Publisher.Ext through that pointer, poisoning every
	// other bidder's clone (e.g. site.publisher.ext.rp leaking into PubMatic).
	if req.Site != nil {
		siteCopy := *req.Site
		if req.Site.Publisher != nil {
			pubCopy := *req.Site.Publisher
			siteCopy.Publisher = &pubCopy
		}
		clone.Site = &siteCopy
	}

	// Clone App only if FPD will modify it
	if req.App != nil && hasFPD && fpdData.App != nil {
		appCopy := *req.App
		clone.App = &appCopy
	}

	// Always deep-copy DOOH+Publisher to prevent cross-bidder data races (2.6)
	if req.DOOH != nil {
		doohCopy := *req.DOOH
		if req.DOOH.Publisher != nil {
			pubCopy := *req.DOOH.Publisher
			doohCopy.Publisher = &pubCopy
		}
		clone.DOOH = &doohCopy
	}

	// Clone User only if FPD will modify it
	if req.User != nil && hasFPD && fpdData.User != nil {
		userCopy := *req.User
		clone.User = &userCopy
	}

	// Strip Rubicon-specific publisher.ext.rp for non-Rubicon bidders.
	// These bleed in from the Rubicon adapter config and confuse other SSPs.
	if bidderCode != "rubicon" && clone.Site != nil && clone.Site.Publisher != nil && len(clone.Site.Publisher.Ext) > 0 {
		siteCopy := *clone.Site
		pubCopy := *clone.Site.Publisher
		pubCopy.Ext = stripPublisherExtRP(pubCopy.Ext)
		siteCopy.Publisher = &pubCopy
		clone.Site = &siteCopy
	}

	// Apply FPD if available (now safe since we cloned the affected objects)
	if hasFPD {
		_ = e.fpdProcessor.ApplyFPDToRequest(&clone, bidderCode, fpdData) //nolint:errcheck
	}

	// Augment supply chain with platform and bidder nodes
	e.augmentSChain(&clone, bidderCode, publisherID)

	return &clone
}

// filterImpExtForBidder strips all known SSP keys from imp.ext except the one
// belonging to bidderCode. This ensures each SSP only receives its own params
// and prevents cross-SSP data bleed (e.g. Rubicon params leaking into PubMatic).
// Unknown keys (e.g. "gpid", "tid", "data") are preserved as-is.
func filterImpExtForBidder(impExt []byte, bidderCode string) []byte {
	// All known bidder keys in imp.ext (must match adapter names as used in the ext object)
	knownBidders := map[string]struct{}{
		"33across":     {},
		"adform":       {},
		"appnexus":     {},
		"beachfront":   {},
		"conversant":   {},
		"criteo":       {},
		"gumgum":       {},
		"improvedigital": {},
		"ix":           {},
		"kargo":        {},
		"medianet":     {},
		"onetag":       {},
		"openx":        {},
		"outbrain":     {},
		"pubmatic":     {},
		"rubicon":      {},
		"sharethrough": {},
		"smartadserver": {},
		"sovrn":        {},
		"spotx":        {},
		"taboola":      {},
		"teads":        {},
		"triplelift":   {},
		"unruly":       {},
	}

	var ext map[string]json.RawMessage
	if err := json.Unmarshal(impExt, &ext); err != nil {
		// If we can't parse, return unchanged
		return impExt
	}

	for key := range ext {
		if _, isKnownBidder := knownBidders[key]; isKnownBidder && key != bidderCode {
			delete(ext, key)
		}
	}

	filtered, err := json.Marshal(ext)
	if err != nil {
		return impExt
	}
	return filtered
}

// stripPublisherExtRP removes the Rubicon-specific "rp" key from site.publisher.ext.
// This prevents Rubicon account IDs from being sent to non-Rubicon SSPs.
func stripPublisherExtRP(pubExt []byte) []byte {
	var ext map[string]json.RawMessage
	if err := json.Unmarshal(pubExt, &ext); err != nil {
		return pubExt
	}
	if _, ok := ext["rp"]; !ok {
		// Nothing to strip
		return pubExt
	}
	delete(ext, "rp")
	stripped, err := json.Marshal(ext)
	if err != nil {
		return pubExt
	}
	return stripped
}

// augmentSChain augments the supply chain with platform and bidder nodes
// per OpenRTB 2.5 section 3.2.2 (Supply Chain Object)
func (e *Exchange) augmentSChain(req *openrtb.BidRequest, bidderCode string, publisherID string) {
	// Ensure Source exists
	if req.Source == nil {
		req.Source = &openrtb.Source{}
	}

	// Move SChain from source.ext if present (legacy location)
	if req.Source.SChain == nil && req.Source.Ext != nil {
		var sourceExt struct {
			SChain *openrtb.SupplyChain `json:"schain"`
		}
		if err := json.Unmarshal(req.Source.Ext, &sourceExt); err == nil && sourceExt.SChain != nil {
			req.Source.SChain = sourceExt.SChain
			// Note: We don't remove from ext to maintain backward compatibility
		}
	}

	// Create SChain if missing
	if req.Source.SChain == nil {
		req.Source.SChain = &openrtb.SupplyChain{
			Complete: 1, // We know the complete chain
			Ver:      "1.0",
			Nodes:    []openrtb.SupplyChainNode{},
		}
	}

	// Partner (Bizbudding) node - prepended before platform node.
	// Bizbudding holds the direct SSP relationships; we resell through them.
	partnerASI := "bizbudding.com"
	bizbuddingSellerIDs := map[string]string{
		"kargo": "9039",
		// Add Bizbudding's seller ID at other SSPs as they are confirmed
	}
	if partnerSID, ok := bizbuddingSellerIDs[bidderCode]; ok {
		hasPartnerNode := false
		for _, node := range req.Source.SChain.Nodes {
			if node.ASI == partnerASI {
				hasPartnerNode = true
				break
			}
		}
		if !hasPartnerNode {
			partnerNode := openrtb.SupplyChainNode{
				ASI: partnerASI,
				SID: partnerSID,
				HP:  1,
			}
			// Prepend so chain reads: Bizbudding → TheNexusEngine
			req.Source.SChain.Nodes = append([]openrtb.SupplyChainNode{partnerNode}, req.Source.SChain.Nodes...)
		}
	}

	// Platform (TheNexusEngine) node
	platformASI := "thenexusengine.com"

	// Per-publisher seller IDs as published in TheNexusEngine sellers.json.
	// The SID identifies which publisher TNE is representing in this auction.
	publisherSellerIDs := map[string]string{
		"12345": "NXS001", // BizBudding
		// beIN Sports and other publishers to be added as they are onboarded
	}
	platformSID, ok := publisherSellerIDs[publisherID]
	if !ok {
		platformSID = "NXS001" // default until publisher is explicitly mapped
	}

	// Check if platform node already exists (avoid duplicates)
	hasPlatformNode := false
	for _, node := range req.Source.SChain.Nodes {
		if node.ASI == platformASI {
			hasPlatformNode = true
			break
		}
	}

	if !hasPlatformNode {
		platformNode := openrtb.SupplyChainNode{
			ASI: platformASI,
			SID: platformSID,
			HP:  1,
			RID: req.ID,
		}
		req.Source.SChain.Nodes = append(req.Source.SChain.Nodes, platformNode)
	}
}

// deepCloneRequest creates a deep copy of the BidRequest to avoid race conditions
// when multiple bidders modify request data concurrently
// P3-1: Uses configurable limits to bound allocations
func deepCloneRequest(req *openrtb.BidRequest, limits *CloneLimits) *openrtb.BidRequest {
	clone := *req

	// P1-NEW-2: Deep copy top-level string slices to prevent shared references
	if len(req.Cur) > 0 {
		clone.Cur = make([]string, len(req.Cur))
		copy(clone.Cur, req.Cur)
	}
	if len(req.WSeat) > 0 {
		clone.WSeat = make([]string, len(req.WSeat))
		copy(clone.WSeat, req.WSeat)
	}
	if len(req.BSeat) > 0 {
		clone.BSeat = make([]string, len(req.BSeat))
		copy(clone.BSeat, req.BSeat)
	}
	if len(req.WLang) > 0 {
		clone.WLang = make([]string, len(req.WLang))
		copy(clone.WLang, req.WLang)
	}
	if len(req.BCat) > 0 {
		clone.BCat = make([]string, len(req.BCat))
		copy(clone.BCat, req.BCat)
	}
	if len(req.BAdv) > 0 {
		clone.BAdv = make([]string, len(req.BAdv))
		copy(clone.BAdv, req.BAdv)
	}
	if len(req.BApp) > 0 {
		clone.BApp = make([]string, len(req.BApp))
		copy(clone.BApp, req.BApp)
	}
	if len(req.WLangB) > 0 {
		clone.WLangB = make([]string, len(req.WLangB))
		copy(clone.WLangB, req.WLangB)
	}

	// Deep copy Site
	if req.Site != nil {
		siteCopy := *req.Site
		if req.Site.Publisher != nil {
			pubCopy := *req.Site.Publisher
			siteCopy.Publisher = &pubCopy
		}
		if req.Site.Content != nil {
			deepCloneContent(req.Site.Content, &siteCopy.Content, limits)
		}
		clone.Site = &siteCopy
	}

	// Deep copy App
	if req.App != nil {
		appCopy := *req.App
		if req.App.Publisher != nil {
			pubCopy := *req.App.Publisher
			appCopy.Publisher = &pubCopy
		}
		if req.App.Content != nil {
			deepCloneContent(req.App.Content, &appCopy.Content, limits)
		}
		clone.App = &appCopy
	}

	// Deep copy DOOH (2.6)
	if req.DOOH != nil {
		doohCopy := *req.DOOH
		if req.DOOH.Publisher != nil {
			pubCopy := *req.DOOH.Publisher
			doohCopy.Publisher = &pubCopy
		}
		if req.DOOH.Content != nil {
			deepCloneContent(req.DOOH.Content, &doohCopy.Content, limits)
		}
		if len(req.DOOH.VenueType) > 0 {
			doohCopy.VenueType = make([]string, len(req.DOOH.VenueType))
			copy(doohCopy.VenueType, req.DOOH.VenueType)
		}
		if len(req.DOOH.KwArray) > 0 {
			doohCopy.KwArray = make([]string, len(req.DOOH.KwArray))
			copy(doohCopy.KwArray, req.DOOH.KwArray)
		}
		clone.DOOH = &doohCopy
	}

	// Deep copy User
	if req.User != nil {
		userCopy := *req.User
		if req.User.Geo != nil {
			geoCopy := *req.User.Geo
			userCopy.Geo = &geoCopy
		}
		// Deep copy EIDs slice (P1-3: bounded allocation)
		if len(req.User.EIDs) > 0 {
			eidCount := len(req.User.EIDs)
			if eidCount > limits.MaxEIDsPerUser {
				eidCount = limits.MaxEIDsPerUser
			}
			userCopy.EIDs = make([]openrtb.EID, eidCount)
			copy(userCopy.EIDs, req.User.EIDs[:eidCount])
		}
		// Deep copy Data slice (P1-3: bounded allocation)
		if len(req.User.Data) > 0 {
			dataCount := len(req.User.Data)
			if dataCount > limits.MaxDataPerUser {
				dataCount = limits.MaxDataPerUser
			}
			userCopy.Data = make([]openrtb.Data, dataCount)
			copy(userCopy.Data, req.User.Data[:dataCount])
		}
		clone.User = &userCopy
	}

	// Deep copy Device
	if req.Device != nil {
		deviceCopy := *req.Device
		if req.Device.Geo != nil {
			geoCopy := *req.Device.Geo
			deviceCopy.Geo = &geoCopy
		}
		// Deep copy SUA (2.6: Structured User-Agent)
		if req.Device.SUA != nil {
			suaCopy := *req.Device.SUA
			if len(req.Device.SUA.Browsers) > 0 {
				suaCopy.Browsers = make([]openrtb.BrandVersion, len(req.Device.SUA.Browsers))
				copy(suaCopy.Browsers, req.Device.SUA.Browsers)
			}
			if req.Device.SUA.Platform != nil {
				platCopy := *req.Device.SUA.Platform
				suaCopy.Platform = &platCopy
			}
			if req.Device.SUA.Mobile != nil {
				mobileCopy := *req.Device.SUA.Mobile
				suaCopy.Mobile = &mobileCopy
			}
			deviceCopy.SUA = &suaCopy
		}
		clone.Device = &deviceCopy
	}

	// Deep copy Regs
	if req.Regs != nil {
		regsCopy := *req.Regs
		if len(req.Regs.GPPSID) > 0 {
			regsCopy.GPPSID = make([]int, len(req.Regs.GPPSID))
			copy(regsCopy.GPPSID, req.Regs.GPPSID)
		}
		clone.Regs = &regsCopy
	}

	// Deep copy Source
	if req.Source != nil {
		sourceCopy := *req.Source
		if req.Source.SChain != nil {
			schainCopy := *req.Source.SChain
			// P1-3: bounded allocation for supply chain nodes
			if len(req.Source.SChain.Nodes) > 0 {
				nodeCount := len(req.Source.SChain.Nodes)
				if nodeCount > limits.MaxSChainNodes {
					nodeCount = limits.MaxSChainNodes
				}
				schainCopy.Nodes = make([]openrtb.SupplyChainNode, nodeCount)
				copy(schainCopy.Nodes, req.Source.SChain.Nodes[:nodeCount])
			}
			sourceCopy.SChain = &schainCopy
		}
		clone.Source = &sourceCopy
	}

	// Deep copy Imp slice (P1-3: bounded allocation)
	if len(req.Imp) > 0 {
		impCount := len(req.Imp)
		if impCount > limits.MaxImpressionsPerRequest {
			impCount = limits.MaxImpressionsPerRequest
		}
		clone.Imp = make([]openrtb.Imp, impCount)
		for i := 0; i < impCount; i++ {
			imp := req.Imp[i]
			impCopy := imp
			if imp.Banner != nil {
				bannerCopy := *imp.Banner
				impCopy.Banner = &bannerCopy
			}
			if imp.Video != nil {
				videoCopy := *imp.Video
				impCopy.Video = &videoCopy
			}
			if imp.Audio != nil {
				audioCopy := *imp.Audio
				impCopy.Audio = &audioCopy
			}
			if imp.Native != nil {
				nativeCopy := *imp.Native
				impCopy.Native = &nativeCopy
			}
			if imp.PMP != nil {
				pmpCopy := *imp.PMP
				// P1-3: bounded allocation for deals
				if len(imp.PMP.Deals) > 0 {
					dealCount := len(imp.PMP.Deals)
					if dealCount > limits.MaxDealsPerImp {
						dealCount = limits.MaxDealsPerImp
					}
					pmpCopy.Deals = make([]openrtb.Deal, dealCount)
					copy(pmpCopy.Deals, imp.PMP.Deals[:dealCount])
				}
				impCopy.PMP = &pmpCopy
			}
			// Deep copy Qty (2.6: DOOH multiplier)
			if imp.Qty != nil {
				qtyCopy := *imp.Qty
				impCopy.Qty = &qtyCopy
			}
			// Deep copy Refresh (2.6: auto-refresh settings)
			if imp.Refresh != nil {
				refreshCopy := *imp.Refresh
				if len(imp.Refresh.RefSettings) > 0 {
					refreshCopy.RefSettings = make([]openrtb.RefInfo, len(imp.Refresh.RefSettings))
					copy(refreshCopy.RefSettings, imp.Refresh.RefSettings)
				}
				impCopy.Refresh = &refreshCopy
			}
			clone.Imp[i] = impCopy
		}
	}

	return &clone
}

// deepCloneContent creates a deep copy of a Content object with all 2.6 sub-objects
func deepCloneContent(src *openrtb.Content, dst **openrtb.Content, limits *CloneLimits) {
	contentCopy := *src
	if len(src.Data) > 0 {
		dataCount := len(src.Data)
		if dataCount > limits.MaxDataPerUser {
			dataCount = limits.MaxDataPerUser
		}
		contentCopy.Data = make([]openrtb.Data, dataCount)
		copy(contentCopy.Data, src.Data[:dataCount])
	}
	if len(src.KwArray) > 0 {
		contentCopy.KwArray = make([]string, len(src.KwArray))
		copy(contentCopy.KwArray, src.KwArray)
	}
	if src.Producer != nil {
		prodCopy := *src.Producer
		contentCopy.Producer = &prodCopy
	}
	// 2.6: Deep copy Network and Channel (CTV context)
	if src.Network != nil {
		netCopy := *src.Network
		contentCopy.Network = &netCopy
	}
	if src.Channel != nil {
		chanCopy := *src.Channel
		contentCopy.Channel = &chanCopy
	}
	*dst = &contentCopy
}

// callBidder calls a single bidder
func (e *Exchange) callBidder(ctx context.Context, req *openrtb.BidRequest, bidderCode string, adapter adapters.Adapter, timeout time.Duration) *BidderResult {
	start := time.Now()

	result := &BidderResult{
		BidderCode: bidderCode,
		Selected:   true,
	}

	// Execute per-bidder request hooks (BEFORE adapter)
	// Hook execution order is critical:
	// 1. Identity Gating - sets user.id from eids (only if consent permits)
	// 2. SChain Augmentation - appends platform node to supply chain
	hookExecutor := hooks.NewHookExecutor()
	hookExecutor.RegisterBidderRequestHook(hooks.NewIdentityGatingHook())

	// SChain is built per-bidder by augmentSChain (called during request cloning)
	// The hook below handles identity gating only; schain augmentation is complete by this point

	if err := hookExecutor.ExecuteBidderRequestHooks(ctx, req, bidderCode); err != nil {
		logger.Log.Error().
			Err(err).
			Str("bidder", bidderCode).
			Str("request_id", req.ID).
			Msg("❌ Bidder request hook failed")
		result.Errors = append(result.Errors, err)
		result.Latency = time.Since(start)
		return result
	}

	// Build requests
	extraInfo := &adapters.ExtraRequestInfo{
		BidderCoreName: bidderCode,
	}

	requests, errs := adapter.MakeRequests(req, extraInfo)
	if len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
	}

	// P1-NEW-6: Check context after potentially expensive MakeRequests operation
	select {
	case <-ctx.Done():
		// P3-1: Log bidder timeout after MakeRequests
		logger.Log.Debug().
			Str("bidder", bidderCode).
			Dur("elapsed", time.Since(start)).
			Msg("bidder timed out after MakeRequests")
		result.Errors = append(result.Errors, ctx.Err())
		result.Latency = time.Since(start)
		result.TimedOut = true
		return result
	default:
		// Context still valid, continue
	}

	if len(requests) == 0 {
		result.Latency = time.Since(start)
		return result
	}

	// Execute requests (could parallelize for multi-request adapters)
	allBids := make([]*adapters.TypedBid, 0)

	for _, reqData := range requests {
		// Check if context has expired before each request to avoid wasted work
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, ctx.Err())
			result.Latency = time.Since(start)
			result.TimedOut = true // P2-2: mark as timed out
			return result
		default:
			// Context still valid, proceed with request
		}

		// Handle mock requests (e.g., demo adapter) - use request body as response
		var resp *adapters.ResponseData
		if reqData.Method == "MOCK" {
			resp = &adapters.ResponseData{
				StatusCode: 200,
				Body:       reqData.Body,
				Headers:    reqData.Headers,
			}
		} else {
			// Log request body for debugging
			requestPreview := string(reqData.Body)
			if len(requestPreview) > 10000 {
				requestPreview = requestPreview[:10000] + "..."
			}

			logger.Log.Debug().
				Str("bidder", bidderCode).
				Str("uri", reqData.URI).
				Str("method", reqData.Method).
				Interface("request_headers", flattenHeaders(reqData.Headers)).
				Str("request_body", requestPreview).
				Msg("Making HTTP request to bidder")

			var err error
			resp, err = e.httpClient.Do(ctx, reqData, timeout)
			if err != nil {
				// P3-1: Log HTTP request failures with context
				isTimeout := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
				logger.Log.Debug().
					Str("bidder", bidderCode).
					Str("uri", reqData.URI).
					Dur("elapsed", time.Since(start)).
					Bool("timeout", isTimeout).
					Err(err).
					Msg("bidder HTTP request failed")
				result.Errors = append(result.Errors, err)
				// P2-2: Check if this was a timeout error
				if isTimeout {
					result.TimedOut = true
				}
				continue
			}
		}

		// Log HTTP response
		responsePreview := string(resp.Body)
		if len(responsePreview) > 500 {
			responsePreview = responsePreview[:500] + "..."
		}
		respLog := logger.Log.Debug().
			Str("bidder", bidderCode).
			Str("uri", reqData.URI).
			Int("status_code", resp.StatusCode).
			Int("body_size", len(resp.Body)).
			Str("response_preview", responsePreview).
			Dur("elapsed", time.Since(start))
		// CP-4: On non-200, log all response headers — SSPs often include rejection reasons
		if resp.StatusCode != http.StatusOK {
			respLog = respLog.Interface("response_headers", flattenHeaders(resp.Headers))
			if reason := resp.Headers.Get("X-Rejection-Reason"); reason != "" {
				respLog = respLog.Str("rejection_reason", reason)
			}
			if xErr := resp.Headers.Get("X-Error"); xErr != "" {
				respLog = respLog.Str("x_error", xErr)
			}
		}
		respLog.Msg("bidder HTTP response received")

		// CP-5: track last HTTP status and any rejection header
		result.LastStatusCode = resp.StatusCode
		if resp.StatusCode != http.StatusOK {
			result.RejectionReason = resp.Headers.Get("X-Rejection-Reason")
		}

		bidderResp, errs := adapter.MakeBids(req, resp)

		if len(errs) > 0 {
			// Log MakeBids errors for visibility
			for _, err := range errs {
				logger.Log.Debug().
					Str("bidder", bidderCode).
					Err(err).
					Msg("bidder MakeBids error")
			}
			result.Errors = append(result.Errors, errs...)
		}

		// Execute bidder response hook (AFTER MakeBids, BEFORE validation)
		// This normalizes and validates the bid response
		if bidderResp != nil {
			// Convert BidderResponse to OpenRTB BidResponse for hook
			ortbResp := &openrtb.BidResponse{
				ID:  bidderResp.ResponseID,
				Cur: bidderResp.Currency,
			}

			// Build SeatBid array from TypedBids
			if len(bidderResp.Bids) > 0 {
				seatBid := openrtb.SeatBid{
					Bid: make([]openrtb.Bid, 0, len(bidderResp.Bids)),
				}
				for _, typedBid := range bidderResp.Bids {
					if typedBid != nil && typedBid.Bid != nil {
						seatBid.Bid = append(seatBid.Bid, *typedBid.Bid)
					}
				}
				ortbResp.SeatBid = []openrtb.SeatBid{seatBid}
			}

			// Execute response normalization hook
			responseHookExecutor := hooks.NewHookExecutor()
			responseHookExecutor.RegisterBidderResponseHook(hooks.NewResponseNormalizationHook())

			if err := responseHookExecutor.ExecuteBidderResponseHooks(ctx, req, ortbResp, bidderCode); err != nil {
				logger.Log.Error().
					Err(err).
					Str("bidder", bidderCode).
					Str("request_id", req.ID).
					Msg("❌ Bidder response hook failed - rejecting bids")
				result.Errors = append(result.Errors, err)
				continue // Skip this response
			}

			// Update bidderResp with normalized values
			bidderResp.ResponseID = ortbResp.ID
			bidderResp.Currency = ortbResp.Cur

			// Task #52: Apply multibid processing to limit bids per bidder
			if e.multibidProcessor != nil && len(ortbResp.SeatBid) > 0 {
				processedResp, err := e.multibidProcessor.ProcessBidderResponse(bidderCode, ortbResp)
				if err != nil {
					logger.Log.Error().
						Err(err).
						Str("bidder", bidderCode).
						Str("request_id", req.ID).
						Msg("❌ Multibid processing failed")
					result.Errors = append(result.Errors, err)
					continue
				}
				ortbResp = processedResp

				// Update bidderResp.Bids to reflect filtered bids
				// FIX: Save original bids before clearing the array
				originalBids := bidderResp.Bids
				bidderResp.Bids = make([]*adapters.TypedBid, 0)
				for _, seatBid := range ortbResp.SeatBid {
					for i := range seatBid.Bid {
						// Find original TypedBid to preserve BidType
						for _, tb := range originalBids {
							if tb.Bid.ID == seatBid.Bid[i].ID {
								bidderResp.Bids = append(bidderResp.Bids, tb)
								break
							}
						}
					}
				}
			}
		}

		if bidderResp != nil {
			// HIGH FIX #1: Validate BidResponse.ID is present and matches BidRequest.ID
			// OpenRTB 2.5 Section 4.2.1: BidResponse.id is REQUIRED and must echo BidRequest.id
			if bidderResp.ResponseID == "" {
				bidCount := len(bidderResp.Bids)
				logger.Log.Error().
					Str("bidder", bidderCode).
					Str("request_id", req.ID).
					Int("bids_rejected", bidCount).
					Msg("❌ Response missing required ID - rejecting all bids (OpenRTB 2.5 §4.2.1)")
				result.Errors = append(result.Errors, fmt.Errorf(
					"missing required response ID from %s (OpenRTB 2.5 section 4.2.1)",
					bidderCode,
				))
				continue // Reject all bids from this response
			}

			// Check for test mode (allow mismatched IDs for test SSPs)
			isTestRequest := isTestRequest(req, bidderCode)

			if bidderResp.ResponseID != req.ID && !isTestRequest {
				bidCount := len(bidderResp.Bids)
				logger.Log.Error().
					Str("bidder", bidderCode).
					Str("request_id", req.ID).
					Str("response_id", bidderResp.ResponseID).
					Int("bids_rejected", bidCount).
					Msg("❌ Response ID mismatch - rejecting all bids (OpenRTB 2.5 §4.2.1)")
				result.Errors = append(result.Errors, fmt.Errorf(
					"response ID mismatch from %s: expected %q, got %q (bids rejected)",
					bidderCode, req.ID, bidderResp.ResponseID,
				))
				continue // Reject all bids from this response
			}

			if isTestRequest && bidderResp.ResponseID != req.ID {
				logger.Log.Debug().
					Str("bidder", bidderCode).
					Str("request_id", req.ID).
					Str("response_id", bidderResp.ResponseID).
					Int("bids_accepted", len(bidderResp.Bids)).
					Msg("Test mode: Accepting bidder response with mismatched ID in exchange")
			}

			// P1-NEW-3: Normalize and validate response currency
			// Per OpenRTB 2.5 spec section 7.2, empty currency means USD
			responseCurrency := bidderResp.Currency
			if responseCurrency == "" {
				responseCurrency = "USD" // OpenRTB 2.5 default
			}

			// HIGH FIX #2: Validate currency against request allowlist if specified
			// OpenRTB 2.5: If BidRequest.cur is specified, response currency must be in that list
			if len(req.Cur) > 0 {
				currencyAllowed := false
				for _, allowedCur := range req.Cur {
					if strings.EqualFold(responseCurrency, allowedCur) {
						currencyAllowed = true
						break
					}
				}
				if !currencyAllowed {
					bidCount := len(bidderResp.Bids)
					logger.Log.Error().
						Str("bidder", bidderCode).
						Str("request_id", req.ID).
						Str("response_currency", responseCurrency).
						Strs("allowed_currencies", req.Cur).
						Int("bids_rejected", bidCount).
						Msg("❌ Response currency not in request allowlist - rejecting all bids")
					result.Errors = append(result.Errors, fmt.Errorf(
						"currency %s from %s not in request allowlist %v (bids rejected)",
						responseCurrency, bidderCode, req.Cur,
					))
					continue
				}
			}

			// P1-NEW-4: Normalize exchange currency to USD if empty
			exchangeCurrency := e.config.DefaultCurrency
			if exchangeCurrency == "" {
				exchangeCurrency = "USD" // Fallback if misconfigured
			}

			// Convert currency if needed
			if responseCurrency != exchangeCurrency {
				if e.currencyConverter == nil {
					bidCount := len(bidderResp.Bids)
					logger.Log.Error().
						Str("bidder", bidderCode).
						Str("request_id", req.ID).
						Str("response_currency", responseCurrency).
						Str("exchange_currency", exchangeCurrency).
						Int("bids_rejected", bidCount).
						Msg("❌ Currency converter not available - rejecting all bids")
					// No converter available - reject bids
					result.Errors = append(result.Errors, fmt.Errorf(
						"currency mismatch from %s: expected %s, got %s (no converter available, bids rejected)",
						bidderCode, exchangeCurrency, responseCurrency,
					))
					continue
				}

				// Convert each bid price to target currency
				convertedBids := make([]*adapters.TypedBid, 0, len(bidderResp.Bids))
				for _, bid := range bidderResp.Bids {
					if bid == nil || bid.Bid == nil {
						continue
					}

					originalPrice := bid.Bid.Price
					convertedPrice, err := e.convertBidCurrency(
						originalPrice,
						responseCurrency,
						exchangeCurrency,
						nil,   // No custom rates at adapter level
						false, // Use external rates
					)

					if err != nil {
						result.Errors = append(result.Errors, fmt.Errorf(
							"failed to convert bid %s from %s to %s: %w (bid rejected)",
							bid.Bid.ID, responseCurrency, exchangeCurrency, err,
						))
						logger.Log.Debug().
							Str("bidder", bidderCode).
							Str("bidID", bid.Bid.ID).
							Str("from", responseCurrency).
							Str("to", exchangeCurrency).
							Float64("originalPrice", originalPrice).
							Err(err).
							Msg("currency conversion failed for bid")
						// Skip this bid but continue with others
						continue
					}

					// Update bid price with converted value
					bid.Bid.Price = convertedPrice
					convertedBids = append(convertedBids, bid)

					logger.Log.Debug().
						Str("bidder", bidderCode).
						Str("bidID", bid.Bid.ID).
						Str("from", responseCurrency).
						Str("to", exchangeCurrency).
						Float64("originalPrice", originalPrice).
						Float64("convertedPrice", convertedPrice).
						Msg("converted bid currency")
				}

				// Only add successfully converted bids
				allBids = append(allBids, convertedBids...)
			} else {
				// Same currency, no conversion needed
				allBids = append(allBids, bidderResp.Bids...)
			}
		}
	}

	result.Bids = allBids
	result.Currency = e.config.DefaultCurrency
	if result.Currency == "" {
		result.Currency = "USD"
	}
	result.Latency = time.Since(start)
	return result
}

// buildEmptyResponse creates an empty bid response with optional NBR code
// P2-7: Using consolidated NoBidReason type from openrtb package
func (e *Exchange) buildEmptyResponse(req *openrtb.BidRequest, nbr openrtb.NoBidReason) *openrtb.BidResponse {
	return &openrtb.BidResponse{
		ID:      req.ID,
		SeatBid: []openrtb.SeatBid{},
		Cur:     e.config.DefaultCurrency,
		NBR:     int(nbr),
	}
}

// applyMultiformatSelection applies multiformat bid selection logic
// For impressions that support multiple formats, selects the best bid
func (e *Exchange) applyMultiformatSelection(
	bidRequest *openrtb.BidRequest,
	bidsByImp map[string][]ValidatedBid,
) map[string][]ValidatedBid {
	// Build impression map for quick lookups
	impMap := make(map[string]*openrtb.Imp)
	for i := range bidRequest.Imp {
		impMap[bidRequest.Imp[i].ID] = &bidRequest.Imp[i]
	}

	result := make(map[string][]ValidatedBid)

	for impID, bids := range bidsByImp {
		imp, exists := impMap[impID]
		if !exists || len(bids) == 0 {
			result[impID] = bids
			continue
		}

		// Check if this is a multiformat impression
		if !e.mfProcessor.IsMultiformat(imp) {
			// Not multiformat, keep all bids
			result[impID] = bids
			continue
		}

		// Convert ValidatedBids to BidCandidates
		candidates := make([]*BidCandidate, 0, len(bids))
		for i := range bids {
			vb := &bids[i]
			mediaType := e.detectBidMediaType(vb.Bid)
			candidate := NewBidCandidate(vb.Bid.Bid, mediaType, vb.BidderCode)
			candidates = append(candidates, candidate)
		}

		// Get preferred media type from impression
		preferredType := e.mfProcessor.GetPreferredMediaType(imp)

		// Select best bid using multiformat logic
		selectedCandidate := e.mfProcessor.SelectBestBid(imp, candidates, preferredType)

		if selectedCandidate != nil {
			// Find the original ValidatedBid that matches the selected candidate
			for i := range bids {
				if bids[i].Bid.Bid.ID == selectedCandidate.Bid.ID {
					result[impID] = []ValidatedBid{bids[i]}
					break
				}
			}
		} else {
			// No bid selected, keep original list
			result[impID] = bids
		}
	}

	return result
}

// detectBidMediaType detects the media type of a bid
func (e *Exchange) detectBidMediaType(bid *adapters.TypedBid) string {
	if bid == nil {
		return ""
	}

	switch bid.BidType {
	case adapters.BidTypeBanner:
		return "banner"
	case adapters.BidTypeVideo:
		return "video"
	case adapters.BidTypeAudio:
		return "audio"
	case adapters.BidTypeNative:
		return "native"
	default:
		return ""
	}
}

// buildBidExtension creates the Prebid extension for a bid including targeting keys
// This is required for Prebid.js integration to work correctly
func (e *Exchange) buildBidExtension(vb ValidatedBid) *openrtb.BidExt {
	bid := vb.Bid.Bid
	bidType := string(vb.Bid.BidType)

	// Generate price bucket using medium granularity
	priceBucket := formatPriceBucket(bid.Price)

	// Determine display bidder code based on demand type:
	// - Platform demand: use "thenexusengine" (obfuscated)
	// - Publisher demand: use original bidder code (transparent)
	displayBidderCode := vb.BidderCode
	if vb.DemandType != adapters.DemandTypePublisher {
		displayBidderCode = adapters.PlatformSeatName // "thenexusengine"
	}

	// Build targeting keys that Prebid.js expects
	targeting := map[string]string{
		"hb_pb":                          priceBucket,
		"hb_bidder":                      displayBidderCode,
		"hb_pb_" + displayBidderCode:     priceBucket,
		"hb_bidder_" + displayBidderCode: displayBidderCode,
	}

	// Only add hb_size for bids that have valid dimensions
	// Video/native/audio bids often don't set W/H, and "0x0" breaks Prebid targeting
	if bid.W > 0 && bid.H > 0 {
		sizeStr := fmt.Sprintf("%dx%d", bid.W, bid.H)
		targeting["hb_size"] = sizeStr
		targeting["hb_size_"+displayBidderCode] = sizeStr
	}

	// Add deal ID if present
	if bid.DealID != "" {
		targeting["hb_deal"] = bid.DealID
		targeting["hb_deal_"+displayBidderCode] = bid.DealID
	}

	// Catalyst GAM targeting keys (_catalyst suffix to avoid conflicts with client-side Prebid.js)
	// These are set server-side so GAM line items can target Catalyst bids directly
	targeting["hb_pb"] = priceBucket
	targeting["hb_bidder_catalyst"] = displayBidderCode
	targeting["hb_partner"] = displayBidderCode
	targeting["hb_source_catalyst"] = "s2s"
	targeting["hb_format_catalyst"] = bidType
	if bid.CRID != "" {
		targeting["hb_adid_catalyst"] = bid.ID
		targeting["hb_creative_catalyst"] = bid.CRID
	}
	if bid.W > 0 && bid.H > 0 {
		targeting["hb_size_catalyst"] = fmt.Sprintf("%dx%d", bid.W, bid.H)
	}
	if bid.DealID != "" {
		targeting["hb_deal_catalyst"] = bid.DealID
	}
	if len(bid.ADomain) > 0 {
		targeting["hb_adomain_catalyst"] = bid.ADomain[0]
	}

	return &openrtb.BidExt{
		Prebid: &openrtb.ExtBidPrebid{
			Type:      bidType,
			Targeting: targeting,
			Meta: &openrtb.ExtBidPrebidMeta{
				MediaType: bidType,
			},
		},
	}
}

// formatPriceBucket formats price using medium granularity (per Prebid.js spec)
// - $0.01 increments up to $5
// - $0.05 increments from $5-$10
// - $0.50 increments from $10-$20
// - Caps at $20
func formatPriceBucket(price float64) string {
	if price <= 0 {
		return "0.00"
	}
	if price > 20 {
		price = 20
	}

	var bucket float64
	switch {
	case price <= 5:
		bucket = float64(int(price*100)) / 100 // $0.01 increments
	case price <= 10:
		bucket = float64(int(price*20)) / 20 // $0.05 increments
	case price <= 20:
		bucket = float64(int(price*2)) / 2 // $0.50 increments
	default:
		bucket = 20
	}
	return fmt.Sprintf("%.2f", bucket)
}

// buildMinimalIDRRequest extracts only essential fields for IDR partner selection
// P1-15: Significantly reduces payload size vs sending full OpenRTB request
func (e *Exchange) buildMinimalIDRRequest(req *openrtb.BidRequest) *idr.MinimalRequest {
	// Extract domain/publisher info
	var domain, publisher, appBundle string
	var categories []string
	isApp := false

	if req.Site != nil {
		domain = req.Site.Domain
		categories = req.Site.Cat
		if req.Site.Publisher != nil {
			publisher = req.Site.Publisher.ID
		}
	} else if req.App != nil {
		isApp = true
		appBundle = req.App.Bundle
		categories = req.App.Cat
		if req.App.Publisher != nil {
			publisher = req.App.Publisher.ID
		}
	} else if req.DOOH != nil {
		domain = req.DOOH.Domain
		if req.DOOH.Publisher != nil {
			publisher = req.DOOH.Publisher.ID
		}
	}

	// Extract geo info
	var country, region string
	if req.Device != nil && req.Device.Geo != nil {
		country = req.Device.Geo.Country
		region = req.Device.Geo.Region
	} else if req.User != nil && req.User.Geo != nil {
		country = req.User.Geo.Country
		region = req.User.Geo.Region
	}

	// Extract device type
	var deviceType string
	if req.Device != nil {
		switch req.Device.DeviceType {
		case openrtb.DeviceTypeMobile:
			deviceType = "mobile"
		case openrtb.DeviceTypePC:
			deviceType = "pc"
		case openrtb.DeviceTypeCTV:
			deviceType = "ctv"
		case openrtb.DeviceTypePhone:
			deviceType = "phone"
		case openrtb.DeviceTypeTablet:
			deviceType = "tablet"
		case openrtb.DeviceTypeConnected:
			deviceType = "connected_device"
		case openrtb.DeviceTypeSetTopBox:
			deviceType = "set_top_box"
		case openrtb.DeviceTypeOOH:
			deviceType = "ooh"
		}
	}

	// Build minimal impressions
	impressions := make([]idr.MinimalImp, 0, len(req.Imp))
	for _, imp := range req.Imp {
		mediaTypes := make([]string, 0, 4)
		var sizes []string

		if imp.Banner != nil {
			mediaTypes = append(mediaTypes, "banner")
			if imp.Banner.W > 0 && imp.Banner.H > 0 {
				sizes = append(sizes, fmt.Sprintf("%dx%d", imp.Banner.W, imp.Banner.H))
			}
			for _, f := range imp.Banner.Format {
				if f.W > 0 && f.H > 0 {
					sizes = append(sizes, fmt.Sprintf("%dx%d", f.W, f.H))
				}
			}
		}
		if imp.Video != nil {
			mediaTypes = append(mediaTypes, "video")
			if imp.Video.W > 0 && imp.Video.H > 0 {
				sizes = append(sizes, fmt.Sprintf("%dx%d", imp.Video.W, imp.Video.H))
			}
		}
		if imp.Native != nil {
			mediaTypes = append(mediaTypes, "native")
		}
		if imp.Audio != nil {
			mediaTypes = append(mediaTypes, "audio")
		}

		impressions = append(impressions, idr.BuildMinimalImp(imp.ID, mediaTypes, sizes))
	}

	return idr.BuildMinimalRequest(
		req.ID,
		domain,
		publisher,
		categories,
		isApp,
		appBundle,
		impressions,
		country,
		region,
		deviceType,
	)
}

// UpdateFPDConfig updates the FPD configuration at runtime
func (e *Exchange) UpdateFPDConfig(config *fpd.Config) {
	if config == nil {
		return
	}

	// Create new processor and filter before acquiring lock to minimize lock hold time
	newProcessor := fpd.NewProcessor(config)
	newFilter := fpd.NewEIDFilter(config)

	e.configMu.Lock()
	defer e.configMu.Unlock()
	e.config.FPD = config
	e.fpdProcessor = newProcessor
	e.eidFilter = newFilter
}

// GetFPDConfig returns the current FPD configuration
func (e *Exchange) GetFPDConfig() *fpd.Config {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	if e.config == nil {
		return nil
	}
	return e.config.FPD
}

// GetIDRClient returns the IDR client (for metrics/admin)
func (e *Exchange) GetIDRClient() *idr.Client {
	return e.idrClient
}

// getDemandType returns the demand type for a bidder (platform or publisher).
// Platform demand is obfuscated under "thenexusengine" seat, publisher demand is transparent.
// Checks static registry, defaults to platform.
func (e *Exchange) getDemandType(bidderCode string) adapters.DemandType {
	// Check static registry first
	if awi, ok := e.registry.Get(bidderCode); ok {
		return awi.Info.DemandType
	}

	// Default to platform (obfuscated) for unknown bidders
	return adapters.DemandTypePlatform
}

// isTestRequest checks if the request is using test credentials
func isTestRequest(req *openrtb.BidRequest, bidderCode string) bool {
	// Check for test patterns in impression extensions
	for _, imp := range req.Imp {
		if imp.Ext == nil {
			continue
		}

		// Parse extension to check for test parameters
		extStr := string(imp.Ext)

		// Check for PubMatic test credentials
		if bidderCode == "pubmatic" && (strings.Contains(extStr, "\"publisherId\":\"156276\"") ||
			strings.Contains(extStr, "\"adSlot\":\"pubmatic_test\"")) {
			return true
		}

		// Check for other test patterns (add as needed)
		if strings.Contains(extStr, "_test") || strings.Contains(extStr, "test_") {
			return true
		}
	}

	return false
}

// filterBiddersWithImpExt returns only the bidders that have parameters in at
// least one imp.Ext across all impressions, matching Prebid Server OSS behaviour.
// "appnexus" is always included because it acts as a pass-through orchestrator
// and does not need its own key in imp.Ext.
// If imp.Ext is absent on all impressions, the original slice is returned unchanged
// so the exchange degrades gracefully.
func filterBiddersWithImpExt(imps []openrtb.Imp, bidders []string) []string {
	configured := make(map[string]struct{})
	for _, imp := range imps {
		if len(imp.Ext) == 0 {
			continue
		}
		var extMap map[string]json.RawMessage
		if err := json.Unmarshal(imp.Ext, &extMap); err != nil {
			continue
		}
		for bidderCode := range extMap {
			configured[bidderCode] = struct{}{}
		}
	}
	if len(configured) == 0 {
		return bidders // no ext data — pass all through unchanged
	}
	filtered := make([]string, 0, len(bidders))
	for _, b := range bidders {
		if b == "appnexus" { // always-include pass-through orchestrator
			filtered = append(filtered, b)
			continue
		}
		if _, ok := configured[b]; ok {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// bidderSupportsInventoryType checks if a bidder supports the inventory type
// present in the bid request (Site, App, or DOOH per OpenRTB 2.6).
// If no capability info is available, the bidder is assumed to support the inventory type.
func bidderSupportsInventoryType(req *openrtb.BidRequest, awi adapters.AdapterWithInfo) bool {
	caps := awi.Info.Capabilities
	if caps == nil {
		return true // No capabilities declared — allow through
	}

	switch {
	case req.DOOH != nil:
		return caps.DOOH != nil
	case req.App != nil:
		return caps.App != nil
	case req.Site != nil:
		return caps.Site != nil
	default:
		return true
	}
}

// inventoryTypeLabel returns a human-readable label for the request's inventory type
func inventoryTypeLabel(req *openrtb.BidRequest) string {
	switch {
	case req.DOOH != nil:
		return "dooh"
	case req.App != nil:
		return "app"
	case req.Site != nil:
		return "site"
	default:
		return "unknown"
	}
}
