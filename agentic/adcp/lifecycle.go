package adcp

// Lifecycle is the auction stage at which an AdCP capability call fires.
// AdCP is capability-oriented rather than mutation-oriented, so the stages
// here describe when the server wants to *consult* an agent rather than
// when an agent gets to *mutate* the bid stream.
type Lifecycle int

const (
	LifecycleUnspecified Lifecycle = 0

	// LifecyclePreAuctionSignals fires before bidder fanout. The server
	// queries signal agents (get_signals) to enrich the bid request with
	// audience/contextual segments the page is eligible for.
	LifecyclePreAuctionSignals Lifecycle = 1

	// LifecyclePreAuctionProducts fires before bidder fanout when a sales
	// agent flow is in scope (direct/PMP). Maps to AdCP get_products.
	LifecyclePreAuctionProducts Lifecycle = 2

	// LifecyclePostAuctionReporting fires after the auction closes. Maps
	// to AdCP update_performance_index for closed-loop optimization.
	LifecyclePostAuctionReporting Lifecycle = 3
)

func (l Lifecycle) String() string {
	switch l {
	case LifecyclePreAuctionSignals:
		return "PRE_AUCTION_SIGNALS"
	case LifecyclePreAuctionProducts:
		return "PRE_AUCTION_PRODUCTS"
	case LifecyclePostAuctionReporting:
		return "POST_AUCTION_REPORTING"
	default:
		return "UNSPECIFIED"
	}
}

// ParseLifecycle accepts the canonical names plus underscore/dash variants.
func ParseLifecycle(s string) Lifecycle {
	switch s {
	case "PRE_AUCTION_SIGNALS", "pre_auction_signals", "pre-auction-signals":
		return LifecyclePreAuctionSignals
	case "PRE_AUCTION_PRODUCTS", "pre_auction_products", "pre-auction-products":
		return LifecyclePreAuctionProducts
	case "POST_AUCTION_REPORTING", "post_auction_reporting", "post-auction-reporting":
		return LifecyclePostAuctionReporting
	default:
		return LifecycleUnspecified
	}
}

// Capability is an AdCP verb the server may invoke on an agent. Phase 1
// recognizes the read-side capabilities; the write-side (activate_signal,
// create_media_buy, etc.) lands in Phase 2.
type Capability string

const (
	CapabilityGetSignals            Capability = "get_signals"
	CapabilityGetProducts           Capability = "get_products"
	CapabilityListCreativeFormats   Capability = "list_creative_formats"
	CapabilityActivateSignal        Capability = "activate_signal"
	CapabilityCreateMediaBuy        Capability = "create_media_buy"
	CapabilityUpdatePerformanceIdx  Capability = "update_performance_index"
	CapabilityGetMediaBuyDelivery   Capability = "get_media_buy_delivery"
)

// IsKnown returns true iff cap is one of the AdCP capabilities this build
// recognizes. Unknown capabilities in agents.json are tolerated by the
// schema (additionalProperties:true) but ignored by the dispatcher.
func (c Capability) IsKnown() bool {
	switch c {
	case CapabilityGetSignals,
		CapabilityGetProducts,
		CapabilityListCreativeFormats,
		CapabilityActivateSignal,
		CapabilityCreateMediaBuy,
		CapabilityUpdatePerformanceIdx,
		CapabilityGetMediaBuyDelivery:
		return true
	default:
		return false
	}
}
