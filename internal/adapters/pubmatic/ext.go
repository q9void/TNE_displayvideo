package pubmatic

import "encoding/json"

// ExtImpPubmatic defines the PubMatic-specific impression extension
type ExtImpPubmatic struct {
	PublisherId string                     `json:"publisherId"`
	AdSlot      string                     `json:"adSlot,omitempty"`
	Dctr        string                     `json:"dctr,omitempty"`
	PmZoneID    string                     `json:"pmZoneId,omitempty"`
	Kadfloor    string                     `json:"kadfloor,omitempty"`
	Keywords    []*ExtImpPubmaticKeyVal    `json:"keywords,omitempty"`
	WrapExt     json.RawMessage            `json:"wrapper,omitempty"`
}

// ExtImpPubmaticKeyVal defines key-value pair for keywords
type ExtImpPubmaticKeyVal struct {
	Key    string   `json:"key"`
	Values []string `json:"value"`
}

// ExtImpBidderPubmatic defines the bidder-specific impression extension
type ExtImpBidderPubmatic struct {
	Bidder      json.RawMessage `json:"bidder"`
	Data        json.RawMessage `json:"data,omitempty"`
	GPID        string          `json:"gpid,omitempty"`
	AE          int             `json:"ae,omitempty"`
	SKAdNetwork json.RawMessage `json:"skadn,omitempty"`
}

// PubmaticWrapperExt defines wrapper extension
type PubmaticWrapperExt struct {
	ProfileID  int    `json:"profile,omitempty"`
	VersionID  int    `json:"version,omitempty"`
	BidderCode string `json:"biddercode,omitempty"`
}

// ExtRequestPubmatic defines request-level extensions
type ExtRequestPubmatic struct {
	Wrapper     *PubmaticWrapperExt  `json:"wrapper,omitempty"`
	Acat        []string             `json:"acat,omitempty"`
	Marketplace *MarketplaceReqExt   `json:"marketplace,omitempty"`
}

// MarketplaceReqExt defines marketplace extension
type MarketplaceReqExt struct {
	AllowedBidders []string `json:"allowedbidders,omitempty"`
}

// ExtRequest defines the request extension structure
type ExtRequest struct {
	Prebid *ExtRequestPrebid `json:"prebid,omitempty"`
}

// ExtRequestPrebid defines prebid extension in request
type ExtRequestPrebid struct {
	BidderParams       json.RawMessage              `json:"bidderparams,omitempty"`
	Aliases            map[string]string            `json:"aliases,omitempty"`
	AlternateBidderCodes *ExtAlternateBidderCodes  `json:"alternatebiddercodes,omitempty"`
}

// ExtAlternateBidderCodes defines alternate bidder codes extension
type ExtAlternateBidderCodes struct {
	Enabled bool                                   `json:"enabled"`
	Bidders map[string]*ExtAlternateBidderCodesRule `json:"bidders,omitempty"`
}

// ExtAlternateBidderCodesRule defines rules for alternate bidder codes
type ExtAlternateBidderCodesRule struct {
	Enabled            bool     `json:"enabled"`
	AllowedBidderCodes []string `json:"allowedbiddercodes,omitempty"`
}

// PubmaticBidExt defines bid extension from PubMatic
type PubmaticBidExt struct {
	VideoCreativeInfo  *PubmaticBidExtVideo `json:"video,omitempty"`
	Marketplace        string               `json:"marketplace,omitempty"`
	PrebidDealPriority int                  `json:"prebiddealpriority,omitempty"`
	InBannerVideo      bool                 `json:"ibv,omitempty"`
}

// PubmaticBidExtVideo defines video info in bid extension
type PubmaticBidExtVideo struct {
	Duration *int `json:"duration,omitempty"`
}

// ExtAppPrebid defines app prebid extension
type ExtAppPrebid struct {
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
}

// ExtApp defines app extension
type ExtApp struct {
	Prebid  *ExtAppPrebid   `json:"prebid,omitempty"`
	Source  string          `json:"source,omitempty"`
	Version string          `json:"version,omitempty"`
}

// RespExt defines response extension
type RespExt struct {
	FledgeAuctionConfigs map[string]json.RawMessage `json:"fledge_auction_configs,omitempty"`
}

// FledgeAuctionConfig defines FLEDGE auction config
type FledgeAuctionConfig struct {
	ImpID  string          `json:"impid"`
	Config json.RawMessage `json:"config"`
}

// ExtAdServer defines ad server extension
type ExtAdServer struct {
	Name   string `json:"name"`
	AdSlot string `json:"adslot"`
}

// ExtData defines first-party data extension
type ExtData struct {
	AdServer  *ExtAdServer `json:"adserver,omitempty"`
	PBAdSlot  string       `json:"pbadslot,omitempty"`
}

const (
	// Extension key names
	DctrKeyName        = "key_val"
	PmZoneIDKeyName    = "pmZoneId"
	PmZoneIDKeyNameOld = "pmZoneID"
	ImpExtAdUnitKey    = "dfp_ad_unit_code"
	AdServerGAM        = "gam"
	AdServerKey        = "adserver"
	PBAdSlotKey        = "pbadslot"
	GPIDKey            = "gpid"
	SKAdNetworkKey     = "skadn"
	AEKey              = "ae"
)
