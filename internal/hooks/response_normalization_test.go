package hooks

import (
	"context"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestResponseNormalizationHook_ValidResponse(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
		},
		Cur: []string{"USD"},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:     "bid-1",
						ImpID:  "imp-1",
						Price:  1.50,
						AdM:    "<creative>",
						CRID:   "creative-123",
						W:      300,
						H:      250,
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Currency should be normalized to uppercase
	if resp.Cur != "USD" {
		t.Errorf("expected currency USD, got: %s", resp.Cur)
	}
}

func TestResponseNormalizationHook_ResponseIDMismatch(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "different-id", // WRONG - doesn't match request ID
		Cur: "USD",
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err == nil {
		t.Fatal("expected error for response ID mismatch, got nil")
	}
}

func TestResponseNormalizationHook_CurrencyNormalization(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		Cur: []string{"USD", "EUR"},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "usd", // lowercase - should be normalized
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Cur != "USD" {
		t.Errorf("expected currency normalized to USD, got: %s", resp.Cur)
	}
}

func TestResponseNormalizationHook_DefaultCurrency(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "", // Empty - should default to USD
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Cur != "USD" {
		t.Errorf("expected currency defaulted to USD, got: %s", resp.Cur)
	}
}

func TestResponseNormalizationHook_InvalidCurrency(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
		Cur: []string{"USD"}, // Only USD allowed
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "EUR", // Not in allowlist
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to first allowed currency
	if resp.Cur != "USD" {
		t.Errorf("expected currency changed to USD (first allowed), got: %s", resp.Cur)
	}
}

func TestResponseNormalizationHook_InvalidBid_MissingID(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "", // Missing - invalid
						ImpID: "imp-1",
						Price: 1.50,
						AdM:   "<creative>",
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out invalid bid
	if len(resp.SeatBid) > 0 && len(resp.SeatBid[0].Bid) > 0 {
		t.Error("expected invalid bid to be filtered out")
	}
}

func TestResponseNormalizationHook_InvalidBid_MissingImpID(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "bid-1",
						ImpID: "", // Missing - invalid
						Price: 1.50,
						AdM:   "<creative>",
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out invalid bid
	if len(resp.SeatBid) > 0 && len(resp.SeatBid[0].Bid) > 0 {
		t.Error("expected invalid bid to be filtered out")
	}
}

func TestResponseNormalizationHook_InvalidBid_WrongImpID(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "bid-1",
						ImpID: "imp-999", // Doesn't match any request impression
						Price: 1.50,
						AdM:   "<creative>",
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out invalid bid
	if len(resp.SeatBid) > 0 && len(resp.SeatBid[0].Bid) > 0 {
		t.Error("expected invalid bid to be filtered out")
	}
}

func TestResponseNormalizationHook_InvalidBid_NegativePrice(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "bid-1",
						ImpID: "imp-1",
						Price: -1.50, // Negative - invalid
						AdM:   "<creative>",
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out invalid bid
	if len(resp.SeatBid) > 0 && len(resp.SeatBid[0].Bid) > 0 {
		t.Error("expected invalid bid (negative price) to be filtered out")
	}
}

func TestResponseNormalizationHook_InvalidBid_ZeroPrice(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "bid-1",
						ImpID: "imp-1",
						Price: 0, // Zero - invalid
						AdM:   "<creative>",
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out invalid bid
	if len(resp.SeatBid) > 0 && len(resp.SeatBid[0].Bid) > 0 {
		t.Error("expected invalid bid (zero price) to be filtered out")
	}
}

func TestResponseNormalizationHook_InvalidBid_MissingCreative(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "bid-1",
						ImpID: "imp-1",
						Price: 1.50,
						AdM:   "", // No adm
						NURL:  "", // No nurl either - invalid
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should filter out invalid bid
	if len(resp.SeatBid) > 0 && len(resp.SeatBid[0].Bid) > 0 {
		t.Error("expected invalid bid (missing creative) to be filtered out")
	}
}

func TestResponseNormalizationHook_MixedValidInvalidBids(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
			{ID: "imp-2", Banner: &openrtb.Banner{W: 728, H: 90}},
		},
	}

	resp := &openrtb.BidResponse{
		ID:  "test-request-123",
		Cur: "USD",
		SeatBid: []openrtb.SeatBid{
			{
				Bid: []openrtb.Bid{
					{
						ID:    "bid-1",
						ImpID: "imp-1",
						Price: 1.50,
						AdM:   "<creative>",
						W:     300,
						H:     250,
					},
					{
						ID:    "bid-2",
						ImpID: "imp-999", // Invalid - wrong impid
						Price: 2.00,
						AdM:   "<creative>",
					},
					{
						ID:    "bid-3",
						ImpID: "imp-2",
						Price: 1.75,
						AdM:   "<creative>",
						W:     728,
						H:     90,
					},
				},
			},
		},
	}

	err := hook.ProcessBidderResponse(ctx, req, resp, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should keep valid bids, filter invalid
	if len(resp.SeatBid) == 0 {
		t.Fatal("expected seat bid to be preserved")
	}
	if len(resp.SeatBid[0].Bid) != 2 {
		t.Errorf("expected 2 valid bids, got: %d", len(resp.SeatBid[0].Bid))
	}

	// Verify correct bids preserved
	bids := resp.SeatBid[0].Bid
	if bids[0].ID != "bid-1" || bids[1].ID != "bid-3" {
		t.Error("expected bid-1 and bid-3 to be preserved")
	}
}

func TestResponseNormalizationHook_NilResponse(t *testing.T) {
	hook := NewResponseNormalizationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:  "test-request-123",
		Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}}},
	}

	err := hook.ProcessBidderResponse(ctx, req, nil, "rubicon")
	if err != nil {
		t.Fatalf("unexpected error for nil response: %v", err)
	}
}
