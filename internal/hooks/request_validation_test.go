package hooks

import (
	"context"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestRequestValidationHook_ValidRequest(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 728,
					H: 90,
				},
			},
		},
		Site: &openrtb.Site{
			Domain: "example.com",
		},
		Cur: []string{"usd", "eur"}, // lowercase - should be normalized
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify currency normalization
	if req.Cur[0] != "USD" {
		t.Errorf("expected currency normalized to USD, got: %s", req.Cur[0])
	}
	if req.Cur[1] != "EUR" {
		t.Errorf("expected currency normalized to EUR, got: %s", req.Cur[1])
	}
}

func TestRequestValidationHook_SiteXORApp_BothPresent(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
		},
		Site: &openrtb.Site{Domain: "example.com"},
		App:  &openrtb.App{Name: "TestApp"}, // Both site and app - invalid
	}

	err := hook.ProcessRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error for both site and app, got nil")
	}
	if err.Error() != "request cannot have both site and app" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRequestValidationHook_SiteXORApp_NeitherPresent(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
		},
		// No site or app - invalid
	}

	err := hook.ProcessRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing site and app, got nil")
	}
	if err.Error() != "request must have either site or app" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRequestValidationHook_NoImpressions(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID:   "test-request-123",
		Imp:  []openrtb.Imp{}, // Empty - invalid
		Site: &openrtb.Site{Domain: "example.com"},
	}

	err := hook.ProcessRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error for no impressions, got nil")
	}
	if err.Error() != "request must have at least one impression" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRequestValidationHook_DuplicateImpID(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
			{ID: "imp-1", Banner: &openrtb.Banner{W: 728, H: 90}}, // Duplicate ID
		},
		Site: &openrtb.Site{Domain: "example.com"},
	}

	err := hook.ProcessRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error for duplicate imp.id, got nil")
	}
	if err.Error() != "duplicate impression id: imp-1" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRequestValidationHook_EmptyImpID(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "", Banner: &openrtb.Banner{W: 300, H: 250}}, // Empty ID
		},
		Site: &openrtb.Site{Domain: "example.com"},
	}

	err := hook.ProcessRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error for empty imp.id, got nil")
	}
	if err.Error() != "impression missing required id" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRequestValidationHook_NoMediaType(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{ID: "imp-1"}, // No banner, video, native, or audio
		},
		Site: &openrtb.Site{Domain: "example.com"},
	}

	err := hook.ProcessRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error for no media type, got nil")
	}
}

func TestRequestValidationHook_TMaxBounds(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	tests := []struct {
		name          string
		inputTMax     int
		expectedTMax  int
		shouldEnforce bool
	}{
		{
			name:          "tmax too low",
			inputTMax:     50,
			expectedTMax:  100,
			shouldEnforce: true,
		},
		{
			name:          "tmax too high",
			inputTMax:     6000,
			expectedTMax:  5000,
			shouldEnforce: true,
		},
		{
			name:          "tmax within bounds",
			inputTMax:     2000,
			expectedTMax:  2000,
			shouldEnforce: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &openrtb.BidRequest{
				ID: "test-request-123",
				Imp: []openrtb.Imp{
					{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}},
				},
				Site: &openrtb.Site{Domain: "example.com"},
				TMax: tt.inputTMax,
			}

			err := hook.ProcessRequest(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if req.TMax != tt.expectedTMax {
				t.Errorf("expected tmax=%d, got tmax=%d", tt.expectedTMax, req.TMax)
			}
		})
	}
}

func TestRequestValidationHook_BidFloorCurNormalization(t *testing.T) {
	hook := NewRequestValidationHook()
	ctx := context.Background()

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		Imp: []openrtb.Imp{
			{
				ID:          "imp-1",
				Banner:      &openrtb.Banner{W: 300, H: 250},
				BidFloorCur: "usd", // lowercase - should be normalized
			},
		},
		Site: &openrtb.Site{Domain: "example.com"},
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Imp[0].BidFloorCur != "USD" {
		t.Errorf("expected BidFloorCur normalized to USD, got: %s", req.Imp[0].BidFloorCur)
	}
}
