package idr

import "time"

// AuctionEvent represents auction-level analytics
// Enhanced format with auction context, not just per-bidder events
type AuctionEvent struct {
	AuctionID        string    `json:"auction_id"`
	RequestID        string    `json:"request_id"`
	PublisherID      string    `json:"publisher_id"`
	Timestamp        time.Time `json:"timestamp"`
	ImpressionCount  int       `json:"impression_count"`
	BiddersSelected  int       `json:"bidders_selected"`
	BiddersExcluded  int       `json:"bidders_excluded"`
	TotalBids        int       `json:"total_bids"`
	WinningBids      int       `json:"winning_bids"`
	DurationMs       int64     `json:"duration_ms"`
	Status           string    `json:"status"` // "success", "no_bids", "error"
	BidMultiplier    float64   `json:"bid_multiplier"`
	TotalRevenue     float64   `json:"total_revenue"`
	TotalPayout      float64   `json:"total_payout"`
	Device           Device    `json:"device"`
	ConsentOK        bool      `json:"consent_ok"`
	ValidationErrors int       `json:"validation_errors"`
}

// BidderEvent represents per-bidder analytics (for ML model)
type BidderEvent struct {
	AuctionID   string   `json:"auction_id"`
	BidderCode  string   `json:"bidder_code"`
	LatencyMs   int64    `json:"latency_ms"`
	HadBid      bool     `json:"had_bid"`
	BidCount    int      `json:"bid_count"`
	FirstBidCPM *float64 `json:"first_bid_cpm,omitempty"`
	FloorPrice  *float64 `json:"floor_price,omitempty"`
	BelowFloor  bool     `json:"below_floor"`
	TimedOut    bool     `json:"timed_out"`
	HadError    bool     `json:"had_error"`
	NoBidReason string   `json:"no_bid_reason,omitempty"`
	Country     string   `json:"country"`
	DeviceType  string   `json:"device_type"`
	MediaType   string   `json:"media_type"`
}

// WinEvent represents winning bid analytics (for revenue tracking)
type WinEvent struct {
	AuctionID   string  `json:"auction_id"`
	BidID       string  `json:"bid_id"`
	ImpID       string  `json:"imp_id"`
	BidderCode  string  `json:"bidder_code"`
	OriginalCPM float64 `json:"original_cpm"`
	AdjustedCPM float64 `json:"adjusted_cpm"`
	PlatformCut float64 `json:"platform_cut"`
	ClearPrice  float64 `json:"clear_price"`
	DemandType  string  `json:"demand_type"` // "platform" or "publisher"
}

// Device contains device information for analytics
type Device struct {
	Country string `json:"country"`
	Type    string `json:"type"` // "desktop", "mobile", "tablet", "ctv"
}
