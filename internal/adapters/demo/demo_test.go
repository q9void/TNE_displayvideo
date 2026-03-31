package demo

import (
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestNew(t *testing.T) {
	adapter := New("")
	if adapter == nil {
		t.Fatal("Expected adapter to be created")
	}

	// Verify default configuration
	if adapter.minCPM != 0.50 {
		t.Errorf("Expected minCPM 0.50, got %f", adapter.minCPM)
	}
	if adapter.maxCPM != 5.00 {
		t.Errorf("Expected maxCPM 5.00, got %f", adapter.maxCPM)
	}
	if adapter.bidRate != 0.80 {
		t.Errorf("Expected bidRate 0.80, got %f", adapter.bidRate)
	}
}

func TestMakeRequests(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
		},
		Site: &openrtb.Site{
			Domain: "example.com",
			Page:   "https://example.com/page",
		},
	}

	requests, errs := adapter.MakeRequests(request, nil)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requests))
	}

	req := requests[0]
	if req.Method != "MOCK" {
		t.Errorf("Expected MOCK method, got %s", req.Method)
	}

	if req.URI != "demo://mock-response" {
		t.Errorf("Expected demo://mock-response URI, got %s", req.URI)
	}

	// Verify request body contains valid bid response
	var parsed openrtb.BidResponse
	if err := json.Unmarshal(req.Body, &parsed); err != nil {
		t.Errorf("Request body is not valid JSON: %v", err)
	}

	if parsed.ID != "test-request-1" {
		t.Errorf("Expected response ID 'test-request-1', got '%s'", parsed.ID)
	}

	if parsed.Cur != "USD" {
		t.Errorf("Expected currency USD, got %s", parsed.Cur)
	}
}

func TestMakeRequests_MultipleImpressions(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-2",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
			{
				ID: "imp-2",
				Banner: &openrtb.Banner{
					W: 728,
					H: 90,
				},
			},
		},
	}

	requests, errs := adapter.MakeRequests(request, nil)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(requests))
	}

	// Verify mock response contains potential bids for both impressions
	var parsed openrtb.BidResponse
	if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Demo adapter may or may not bid on all impressions due to random bidRate
	// Just verify structure is valid
	if len(parsed.SeatBid) > 1 {
		t.Errorf("Expected at most 1 seatbid, got %d", len(parsed.SeatBid))
	}
}

func TestMakeBids_Success(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
		},
	}

	responseBody := `{
		"id": "test-response-1",
		"cur": "USD",
		"seatbid": [{
			"seat": "demo-dsp",
			"bid": [{
				"id": "demo-bid-1",
				"impid": "imp-1",
				"price": 2.50,
				"adm": "<div>Mock Ad</div>",
				"w": 300,
				"h": 250,
				"crid": "demo-creative-123"
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: 200,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if bidderResponse == nil {
		t.Fatal("Expected bidder response, got nil")
	}

	if bidderResponse.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", bidderResponse.Currency)
	}

	if bidderResponse.ResponseID != "test-response-1" {
		t.Errorf("Expected response ID 'test-response-1', got '%s'", bidderResponse.ResponseID)
	}

	if len(bidderResponse.Bids) != 1 {
		t.Fatalf("Expected 1 bid, got %d", len(bidderResponse.Bids))
	}

	bid := bidderResponse.Bids[0]
	if bid.Bid.Price != 2.50 {
		t.Errorf("Expected bid price 2.50, got %f", bid.Bid.Price)
	}

	if bid.BidType != adapters.BidTypeBanner {
		t.Errorf("Expected banner bid type, got %v", bid.BidType)
	}
}

func TestMakeBids_InvalidJSON(t *testing.T) {
	adapter := New("")

	response := &adapters.ResponseData{
		StatusCode: 200,
		Body:       []byte("not valid json"),
	}

	_, errs := adapter.MakeBids(&openrtb.BidRequest{}, response)

	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}
}

func TestMakeBids_EmptyResponse(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
		},
	}

	responseBody := `{
		"id": "test-response-1",
		"cur": "USD",
		"seatbid": []
	}`

	response := &adapters.ResponseData{
		StatusCode: 200,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if bidderResponse == nil {
		t.Fatal("Expected bidder response, got nil")
	}

	if len(bidderResponse.Bids) != 0 {
		t.Errorf("Expected 0 bids, got %d", len(bidderResponse.Bids))
	}
}

func TestInfo(t *testing.T) {
	info := Info()

	// Demo adapter is intentionally disabled in production — it competes with real bidders
	if info.Enabled {
		t.Error("Expected adapter to be disabled in production")
	}

	if info.GVLVendorID != 0 {
		t.Errorf("Expected GVL vendor ID 0 (no GDPR), got %d", info.GVLVendorID)
	}

	if info.Capabilities == nil {
		t.Fatal("Expected capabilities to be set")
	}

	if info.Capabilities.Site == nil {
		t.Fatal("Expected site capabilities")
	}

	if info.Capabilities.App == nil {
		t.Fatal("Expected app capabilities")
	}

	// Check banner support in site
	hasBanner := false
	for _, mt := range info.Capabilities.Site.MediaTypes {
		if mt == adapters.BidTypeBanner {
			hasBanner = true
			break
		}
	}
	if !hasBanner {
		t.Error("Expected banner support in site capabilities")
	}

	// Check video support
	hasVideo := false
	for _, mt := range info.Capabilities.Site.MediaTypes {
		if mt == adapters.BidTypeVideo {
			hasVideo = true
			break
		}
	}
	if !hasVideo {
		t.Error("Expected video support in site capabilities")
	}
}

func TestAdapterInfo(t *testing.T) {
	adapter := New("")
	info := adapter.Info()

	// Should return same as package Info() — disabled in production
	if info.Enabled {
		t.Error("Expected adapter to be disabled in production")
	}

	if info.DemandType != adapters.DemandTypePlatform {
		t.Errorf("Expected platform demand type, got %s", info.DemandType)
	}
}

func TestGenerateMockCreative(t *testing.T) {
	adapter := New("")

	creative := adapter.generateMockCreative(300, 250, 1.50)

	if creative == "" {
		t.Error("Expected non-empty creative")
	}

	// Verify creative contains size
	if len(creative) < 100 {
		t.Error("Expected creative to be substantial HTML")
	}
}
