package agentic

import (
	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

// OriginatorStamper is the small value type that embeds our seller identity
// into outbound RTBRequest messages. SellerID is read once at boot from
// AGENTIC_SELLER_ID and is constant for the process lifetime — there is no
// reason for this to be an interface or a goroutine-safe object.
type OriginatorStamper struct {
	SellerID string // e.g. "9131" — must match the schain seller_id
}

// StampRTBRequest sets Lifecycle and Originator on req. Idempotent —
// callers may invoke it multiple times before Dispatch without side effects.
// Mutates req in place; never returns an error.
//
// Edition 2023 generated proto types use pointer fields for optional values,
// so we use the generated Enum() helpers + take the address of SellerID.
func (s OriginatorStamper) StampRTBRequest(req *pb.RTBRequest, lc Lifecycle) {
	if req == nil {
		return
	}
	lcProto := lc.Proto()
	req.Lifecycle = &lcProto
	id := s.SellerID
	req.Originator = &pb.Originator{
		Type: pb.Originator_TYPE_SSP.Enum(),
		Id:   &id,
	}
}
