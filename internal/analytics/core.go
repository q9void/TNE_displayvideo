// Package analytics provides a standardized interface for auction analytics
package analytics

import (
	"context"
	"time"
)

// Module interface for auction analytics
// Implementations can send data to IDR service, DataDog, BigQuery, etc.
type Module interface {
	LogAuctionObject(ctx context.Context, auction *AuctionObject) error
	LogVideoObject(ctx context.Context, video *VideoObject) error
	Shutdown() error
}

// AuctionObject contains complete auction transaction data
// This rich data model enables comprehensive analytics across multiple sinks
type AuctionObject struct {
	// Request context
	AuctionID       string
	RequestID       string
	PublisherID     string
	PublisherDomain string
	PageURL         string
	Timestamp       time.Time

	// Request details
	Impressions []Impression
	Device      *DeviceInfo
	User        *UserInfo
	TMax        int
	Currency    string
	Test        bool

	// Bidder selection
	SelectedBidders []string
	ExcludedBidders map[string]ExclusionReason // bidder → why excluded
	TotalBidders    int

	// Bidding results
	BidderResults map[string]*BidderResult
	WinningBids   []WinningBid
	TotalBids     int

	// Auction outcome
	AuctionDuration time.Duration
	Status          string // "success", "no_bids", "error"

	// Revenue/Margin (platform-specific)
	BidMultiplier    float64
	FloorAdjustments map[string]float64 // impID → adjusted floor
	TotalRevenue     float64            // sum of platform cuts
	TotalPayout      float64            // sum of publisher payouts

	// Privacy compliance
	GDPR      *GDPRData
	CCPA      *CCPAData
	COPPA     bool
	ConsentOK bool

	// Validation/Errors
	ValidationErrors []ValidationError
	RequestErrors    []string
	BidderErrors     map[string][]string
}

// ExclusionReason represents why a bidder was excluded from auction
type ExclusionReason struct {
	Code    string // "circuit_breaker_open", "no_consent", "no_config", "ivt_detected", "no_supply_chain"
	Message string
}

// BidderResult contains per-bidder auction result
type BidderResult struct {
	BidderCode  string
	Latency     time.Duration
	HttpStatus  int
	Bids        []BidDetails
	SeatBids    int
	TimedOut    bool
	NoBidReason string // OpenRTB NBR code
	Errors      []string
}

// BidDetails contains individual bid information
type BidDetails struct {
	BidID           string
	ImpID           string
	OriginalPrice   float64
	AdjustedPrice   float64 // after multiplier
	Currency        string
	ADomain         []string
	CreativeID      string
	DemandType      string // "platform" or "publisher"
	BelowFloor      bool
	Rejected        bool
	RejectionReason string
}

// WinningBid represents a bid that won the auction
type WinningBid struct {
	BidID         string
	ImpID         string
	BidderCode    string
	OriginalPrice float64
	AdjustedPrice float64
	PlatformCut   float64
	Currency      string
	ADomain       []string
	CreativeID    string
	DemandType    string
	ClearPrice    float64 // second-price auction
}

// ValidationError represents a bid validation failure
type ValidationError struct {
	BidID   string
	ImpID   string
	Bidder  string
	Field   string // "price", "floor", "domain", etc.
	Reason  string
	Value   interface{}
}

// Impression contains impression-level request data
type Impression struct {
	ID         string
	TagID      string   // Ad unit path / tag identifier (imp.tagid in OpenRTB)
	MediaTypes []string // "banner", "video", "native", "audio"
	Sizes      []string // "300x250", "728x90", etc.
	Floor      float64
}

// DeviceInfo contains device information
type DeviceInfo struct {
	Type    string // "desktop", "mobile", "tablet", "ctv"
	Country string
	Region  string
	IP      string
	UA      string
}

// UserInfo contains user information (privacy-safe)
type UserInfo struct {
	BuyerUID string // Anonymized user ID
	HasEIDs  bool   // Has extended IDs
	TotalEIDs int   // Total count of EID sources present

	// Per-source EID UIDs (empty string = not present in request)
	FPID        string // thenexusengine.com first-party ID
	ID5UID      string // id5-sync.com
	RubiconUID  string // rubiconproject.com
	KargoUID    string // kargo.com
	PubmaticUID string // pubmatic.com
	SovrnUID    string // lijit.com (Sovrn)
	AppNexusUID string // adnxs.com
}

// GDPRData contains GDPR consent information
type GDPRData struct {
	Applies       bool
	ConsentString string
	InScope       bool
}

// CCPAData contains CCPA consent information
type CCPAData struct {
	Applies       bool
	OptOut        bool   // User has opted out
	USPrivacy     string // IAB US Privacy string (e.g., "1YNN")
	ConsentString string // Legacy field (deprecated, use USPrivacy)
}

// VideoObject contains video-specific analytics
// Separate from auction analytics to handle video events differently
type VideoObject struct {
	AuctionID string
	VideoID   string
	Event     string // "impression", "start", "firstQuartile", "midpoint", "thirdQuartile", "complete", "error"
	Timestamp time.Time
	Duration  int
	Muted     bool
	Fullscreen bool
	PlayerSize string

	// Tracking-pixel event fields (populated by VideoEventHandler;
	// adapters that only care about player telemetry can ignore these).
	BidID        string
	AccountID    string
	Bidder       string
	Progress     float64
	ErrorCode    string
	ErrorMessage string
	ClickURL     string
	SessionID    string
	ContentID    string
	IPAddress    string // anonymised at ingest, see middleware.AnonymizeIPForLogging
	UserAgent    string
}
