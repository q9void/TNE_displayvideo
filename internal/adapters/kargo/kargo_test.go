package kargo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestNew(t *testing.T) {
	adapter := New("")
	if adapter.endpoint != defaultEndpoint {
		t.Errorf("Expected default endpoint %s, got %s", defaultEndpoint, adapter.endpoint)
	}

	customEndpoint := "https://custom.endpoint.com"
	adapter = New(customEndpoint)
	if adapter.endpoint != customEndpoint {
		t.Errorf("Expected custom endpoint %s, got %s", customEndpoint, adapter.endpoint)
	}
}

func TestMakeRequests(t *testing.T) {
	adapter := New("")

	// Task #20: Add required kargo.placementId extension
	impExt := json.RawMessage(`{"kargo":{"placementId":"test-placement-123"}}`)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
				Ext: impExt,
			},
		},
		Site: &openrtb.Site{
			Domain: "example.com",
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
	if req.Method != "POST" {
		t.Errorf("Expected POST method, got %s", req.Method)
	}

	if req.URI != defaultEndpoint {
		t.Errorf("Expected URI %s, got %s", defaultEndpoint, req.URI)
	}

	var parsed openrtb.BidRequest
	if err := json.Unmarshal(req.Body, &parsed); err != nil {
		t.Errorf("Request body is not valid JSON: %v", err)
	}

	if parsed.ID != request.ID {
		t.Errorf("Expected request ID %s, got %s", request.ID, parsed.ID)
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
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 1.50,
				"adm": "<div>Ad</div>",
				"w": 300,
				"h": 250
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if bidderResponse == nil {
		t.Fatal("Expected bidder response, got nil")
	}

	if len(bidderResponse.Bids) != 1 {
		t.Fatalf("Expected 1 bid, got %d", len(bidderResponse.Bids))
	}

	bid := bidderResponse.Bids[0]
	if bid.Bid.Price != 1.50 {
		t.Errorf("Expected bid price 1.50, got %f", bid.Bid.Price)
	}

	if bid.BidType != adapters.BidTypeBanner {
		t.Errorf("Expected bid type banner, got %s", bid.BidType)
	}

	if bidderResponse.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", bidderResponse.Currency)
	}

	if bidderResponse.ResponseID != "response-1" {
		t.Errorf("Expected response ID response-1, got %s", bidderResponse.ResponseID)
	}
}

func TestMakeBids_Video(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Video: &openrtb.Video{
					W: 640,
					H: 480,
				},
			},
		},
	}

	responseBody := `{
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 2.00,
				"adm": "<VAST></VAST>",
				"w": 640,
				"h": 480
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(bidderResponse.Bids) != 1 {
		t.Fatalf("Expected 1 bid, got %d", len(bidderResponse.Bids))
	}

	bid := bidderResponse.Bids[0]
	if bid.BidType != adapters.BidTypeVideo {
		t.Errorf("Expected bid type video, got %s", bid.BidType)
	}
}

func TestMakeBids_Native(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Native: &openrtb.Native{
					Request: "{}",
				},
			},
		},
	}

	responseBody := `{
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 1.25,
				"adm": "{\"native\":{}}"
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(bidderResponse.Bids) != 1 {
		t.Fatalf("Expected 1 bid, got %d", len(bidderResponse.Bids))
	}

	bid := bidderResponse.Bids[0]
	if bid.BidType != adapters.BidTypeNative {
		t.Errorf("Expected bid type native, got %s", bid.BidType)
	}
}

func TestMakeBids_NoContent(t *testing.T) {
	adapter := New("")

	response := &adapters.ResponseData{
		StatusCode: http.StatusNoContent,
		Body:       nil,
	}

	bidderResponse, errs := adapter.MakeBids(&openrtb.BidRequest{}, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if bidderResponse != nil {
		t.Error("Expected nil response for NoContent")
	}
}

func TestMakeBids_BadRequest(t *testing.T) {
	adapter := New("")

	response := &adapters.ResponseData{
		StatusCode: http.StatusBadRequest,
		Body:       []byte("invalid request"),
	}

	bidderResponse, errs := adapter.MakeBids(&openrtb.BidRequest{}, response)

	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	if bidderResponse != nil {
		t.Error("Expected nil response for bad request")
	}
}

func TestMakeBids_UnexpectedStatus(t *testing.T) {
	adapter := New("")

	response := &adapters.ResponseData{
		StatusCode: http.StatusInternalServerError,
		Body:       []byte("server error"),
	}

	bidderResponse, errs := adapter.MakeBids(&openrtb.BidRequest{}, response)

	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}

	if bidderResponse != nil {
		t.Error("Expected nil response for unexpected status")
	}
}

func TestMakeBids_InvalidJSON(t *testing.T) {
	adapter := New("")

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte("not json"),
	}

	_, errs := adapter.MakeBids(&openrtb.BidRequest{}, response)

	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}
}

func TestMakeBids_MultipleBids(t *testing.T) {
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
			{
				ID: "imp-2",
				Banner: &openrtb.Banner{
					W: 728,
					H: 90,
				},
			},
		},
	}

	responseBody := `{
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 1.50,
				"adm": "<div>Ad 1</div>",
				"w": 300,
				"h": 250
			}, {
				"id": "bid-2",
				"impid": "imp-2",
				"price": 2.00,
				"adm": "<div>Ad 2</div>",
				"w": 728,
				"h": 90
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(bidderResponse.Bids) != 2 {
		t.Fatalf("Expected 2 bids, got %d", len(bidderResponse.Bids))
	}
}

func TestInfo(t *testing.T) {
	info := Info()

	if !info.Enabled {
		t.Error("Expected adapter to be enabled")
	}

	if info.GVLVendorID != 972 {
		t.Errorf("Expected GVL vendor ID 972, got %d", info.GVLVendorID)
	}

	if info.Endpoint != defaultEndpoint {
		t.Errorf("Expected endpoint %s, got %s", defaultEndpoint, info.Endpoint)
	}

	if info.Maintainer == nil || info.Maintainer.Email != "kraken@kargo.com" {
		t.Error("Expected maintainer email to be kraken@kargo.com")
	}

	if info.Capabilities == nil {
		t.Fatal("Expected capabilities to be set")
	}

	if info.Capabilities.Site == nil {
		t.Fatal("Expected site capabilities to be set")
	}

	expectedMediaTypes := []adapters.BidType{
		adapters.BidTypeBanner,
		adapters.BidTypeVideo,
		adapters.BidTypeNative,
	}

	if len(info.Capabilities.Site.MediaTypes) != len(expectedMediaTypes) {
		t.Fatalf("Expected %d media types, got %d", len(expectedMediaTypes), len(info.Capabilities.Site.MediaTypes))
	}

	for i, expectedType := range expectedMediaTypes {
		if info.Capabilities.Site.MediaTypes[i] != expectedType {
			t.Errorf("Expected media type %s at index %d, got %s", expectedType, i, info.Capabilities.Site.MediaTypes[i])
		}
	}

	if info.DemandType != adapters.DemandTypePlatform {
		t.Errorf("Expected demand type platform, got %s", info.DemandType)
	}
}

// Task #20: Test placementId validation
func TestMakeRequests_MissingPlacementId(t *testing.T) {
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
				// Missing imp.ext entirely
			},
		},
	}

	_, errs := adapter.MakeRequests(request, nil)

	if len(errs) == 0 {
		t.Fatal("Expected error for missing imp.ext")
	}
}

func TestMakeRequests_EmptyPlacementId(t *testing.T) {
	adapter := New("")

	impExt := json.RawMessage(`{"kargo":{"placementId":""}}`)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
				Ext: impExt,
			},
		},
	}

	_, errs := adapter.MakeRequests(request, nil)

	if len(errs) == 0 {
		t.Fatal("Expected error for empty placementId")
	}
}

// Task #19: Test case-insensitive gzip detection
func TestMakeBids_GzipCaseInsensitive(t *testing.T) {
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
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 1.50,
				"adm": "<div>Ad</div>",
				"w": 300,
				"h": 250
			}]
		}]
	}`

	// Compress the response
	var compressedBody bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBody)
	gzipWriter.Write([]byte(responseBody))
	gzipWriter.Close()

	// Test with uppercase GZIP
	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       compressedBody.Bytes(),
		Headers:    http.Header{"Content-Encoding": []string{"GZIP"}},
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if bidderResponse == nil {
		t.Fatal("Expected bidder response, got nil")
	}

	if len(bidderResponse.Bids) != 1 {
		t.Fatalf("Expected 1 bid, got %d", len(bidderResponse.Bids))
	}
}

// Task #17: Test mediaType validation against impression capabilities
func TestMakeBids_MediaTypeValidation_Valid(t *testing.T) {
	adapter := New("")

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID: "imp-1",
				Video: &openrtb.Video{
					W: 640,
					H: 480,
				},
			},
		},
	}

	// Bid with video mediaType matching video impression
	responseBody := `{
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 2.00,
				"adm": "<VAST></VAST>",
				"ext": {"mediaType": "video"}
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	if len(bidderResponse.Bids) != 1 {
		t.Fatalf("Expected 1 bid, got %d", len(bidderResponse.Bids))
	}

	if bidderResponse.Bids[0].BidType != adapters.BidTypeVideo {
		t.Errorf("Expected bid type video, got %s", bidderResponse.Bids[0].BidType)
	}
}

func TestMakeBids_MediaTypeValidation_Invalid(t *testing.T) {
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
				// Only supports banner
			},
		},
	}

	// Bid claims to be video but imp only supports banner
	responseBody := `{
		"id": "response-1",
		"cur": "USD",
		"seatbid": [{
			"bid": [{
				"id": "bid-1",
				"impid": "imp-1",
				"price": 2.00,
				"adm": "<VAST></VAST>",
				"ext": {"mediaType": "video"}
			}]
		}]
	}`

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte(responseBody),
	}

	bidderResponse, errs := adapter.MakeBids(request, response)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	// Bid should be skipped due to validation failure
	if len(bidderResponse.Bids) != 0 {
		t.Fatalf("Expected 0 bids (invalid bid skipped), got %d", len(bidderResponse.Bids))
	}
}
