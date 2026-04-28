package agentic

import (
	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

// Lifecycle is the auction stage at which an extension-point call fires.
// We deliberately mirror — but do not export — the proto enum so callers in
// internal/exchange don't need to import the generated proto package.
type Lifecycle int

const (
	LifecycleUnspecified         Lifecycle = 0
	LifecyclePublisherBidRequest Lifecycle = 1
	LifecycleDSPBidResponse      Lifecycle = 2
)

// String returns the canonical ARTF lifecycle name.
func (l Lifecycle) String() string {
	switch l {
	case LifecyclePublisherBidRequest:
		return "PUBLISHER_BID_REQUEST"
	case LifecycleDSPBidResponse:
		return "DSP_BID_RESPONSE"
	default:
		return "UNSPECIFIED"
	}
}

// Proto converts to the generated protobuf enum.
func (l Lifecycle) Proto() pb.Lifecycle {
	switch l {
	case LifecyclePublisherBidRequest:
		return pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST
	case LifecycleDSPBidResponse:
		return pb.Lifecycle_LIFECYCLE_DSP_BID_RESPONSE
	default:
		return pb.Lifecycle_LIFECYCLE_UNSPECIFIED
	}
}

// LifecycleFromProto is the inverse of Proto().
func LifecycleFromProto(p pb.Lifecycle) Lifecycle {
	switch p {
	case pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST:
		return LifecyclePublisherBidRequest
	case pb.Lifecycle_LIFECYCLE_DSP_BID_RESPONSE:
		return LifecycleDSPBidResponse
	default:
		return LifecycleUnspecified
	}
}

// ParseLifecycle accepts the ARTF canonical names plus a few common variants.
func ParseLifecycle(s string) Lifecycle {
	switch s {
	case "PUBLISHER_BID_REQUEST", "publisher_bid_request", "publisher-bid-request":
		return LifecyclePublisherBidRequest
	case "DSP_BID_RESPONSE", "dsp_bid_response", "dsp-bid-response":
		return LifecycleDSPBidResponse
	default:
		return LifecycleUnspecified
	}
}
