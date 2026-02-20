package aniview

import (
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

func TestMakeRequests_BizBuddingParams(t *testing.T) {
	adapter := New("")

	impExt := map[string]interface{}{
		"publisherId": "test-publisher-123",
		"channelId":   "test-channel-456",
	}
	extJSON, _ := json.Marshal(impExt)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID:  "imp-1",
				Ext: extJSON,
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
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
		t.Errorf("Expected endpoint %s, got %s", defaultEndpoint, req.URI)
	}

	var parsed openrtb.BidRequest
	if err := json.Unmarshal(req.Body, &parsed); err != nil {
		t.Errorf("Request body is not valid JSON: %v", err)
	}
}

func TestMakeRequests_PrebidParams(t *testing.T) {
	adapter := New("")

	impExt := map[string]interface{}{
		"AV_PUBLISHERID": "test-publisher-123",
		"AV_CHANNELID":   "test-channel-456",
	}
	extJSON, _ := json.Marshal(impExt)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID:  "imp-1",
				Ext: extJSON,
				Video: &openrtb.Video{
					W: 640,
					H: 480,
				},
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
}

func TestMakeRequests_MissingPublisherId(t *testing.T) {
	adapter := New("")

	impExt := map[string]interface{}{
		"channelId": "test-channel-456",
	}
	extJSON, _ := json.Marshal(impExt)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID:  "imp-1",
				Ext: extJSON,
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
		},
	}

	requests, errs := adapter.MakeRequests(request, nil)

	if len(errs) == 0 {
		t.Fatal("Expected error for missing publisherId")
	}

	if len(requests) != 0 {
		t.Errorf("Expected no requests, got %d", len(requests))
	}
}

func TestMakeRequests_MissingChannelId(t *testing.T) {
	adapter := New("")

	impExt := map[string]interface{}{
		"publisherId": "test-publisher-123",
	}
	extJSON, _ := json.Marshal(impExt)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID:  "imp-1",
				Ext: extJSON,
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
		},
	}

	requests, errs := adapter.MakeRequests(request, nil)

	if len(errs) == 0 {
		t.Fatal("Expected error for missing channelId")
	}

	if len(requests) != 0 {
		t.Errorf("Expected no requests, got %d", len(requests))
	}
}

func TestMakeRequests_MultipleImps(t *testing.T) {
	adapter := New("")

	impExt1 := map[string]interface{}{
		"publisherId": "test-publisher-123",
		"channelId":   "test-channel-456",
	}
	extJSON1, _ := json.Marshal(impExt1)

	impExt2 := map[string]interface{}{
		"publisherId": "test-publisher-789",
		"channelId":   "test-channel-012",
	}
	extJSON2, _ := json.Marshal(impExt2)

	request := &openrtb.BidRequest{
		ID: "test-request-1",
		Imp: []openrtb.Imp{
			{
				ID:  "imp-1",
				Ext: extJSON1,
				Banner: &openrtb.Banner{
					W: 300,
					H: 250,
				},
			},
			{
				ID:  "imp-2",
				Ext: extJSON2,
				Video: &openrtb.Video{
					W: 640,
					H: 480,
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

	var parsed openrtb.BidRequest
	if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
		t.Errorf("Request body is not valid JSON: %v", err)
	}

	if len(parsed.Imp) != 2 {
		t.Errorf("Expected 2 impressions in request, got %d", len(parsed.Imp))
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
				"price": 2.50,
				"adm": "<VAST version=\"3.0\"></VAST>",
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

	if bidderResponse.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", bidderResponse.Currency)
	}

	bid := bidderResponse.Bids[0]
	if bid.Bid.Price != 2.50 {
		t.Errorf("Expected price 2.50, got %f", bid.Bid.Price)
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

	if len(errs) == 0 {
		t.Fatal("Expected error for bad request")
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

	if len(errs) == 0 {
		t.Fatal("Expected error for unexpected status")
	}

	if bidderResponse != nil {
		t.Error("Expected nil response for unexpected status")
	}
}

func TestMakeBids_InvalidJSON(t *testing.T) {
	adapter := New("")

	response := &adapters.ResponseData{
		StatusCode: http.StatusOK,
		Body:       []byte("not valid json"),
	}

	_, errs := adapter.MakeBids(&openrtb.BidRequest{}, response)

	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errs))
	}
}

func TestInfo(t *testing.T) {
	info := Info()

	if !info.Enabled {
		t.Error("Expected adapter to be enabled")
	}

	if info.GVLVendorID != 780 {
		t.Errorf("Expected GVL Vendor ID 780, got %d", info.GVLVendorID)
	}

	if info.Endpoint != defaultEndpoint {
		t.Errorf("Expected endpoint %s, got %s", defaultEndpoint, info.Endpoint)
	}

	if info.Capabilities == nil {
		t.Fatal("Expected capabilities to be set")
	}

	if info.Capabilities.Site == nil {
		t.Fatal("Expected site capabilities to be set")
	}

	expectedMediaTypes := 2 // Banner and Video
	if len(info.Capabilities.Site.MediaTypes) != expectedMediaTypes {
		t.Errorf("Expected %d media types, got %d", expectedMediaTypes, len(info.Capabilities.Site.MediaTypes))
	}

	if info.DemandType != adapters.DemandTypePublisher {
		t.Errorf("Expected DemandType to be publisher, got %s", info.DemandType)
	}
}
