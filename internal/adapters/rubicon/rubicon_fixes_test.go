package rubicon

import (
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// TestAppRequestHandling tests Task #21: Add App request handling
func TestAppRequestHandling(t *testing.T) {
	adapter := New("")

	rubiconExt := json.RawMessage(`{"rubicon":{"accountId":123,"siteId":456,"zoneId":789}}`)
	request := &openrtb.BidRequest{
		ID: "test-app-request",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: rubiconExt},
		},
		App: &openrtb.App{
			ID:     "app-123",
			Name:   "TestApp",
			Bundle: "com.example.app",
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
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Verify App exists and has proper structure
	if parsed.App == nil {
		t.Fatal("Expected App to be present in request")
	}

	// Verify App.Ext has Rubicon extension
	var appExt map[string]interface{}
	if err := json.Unmarshal(parsed.App.Ext, &appExt); err != nil {
		t.Fatalf("Failed to parse App.Ext: %v", err)
	}

	rpData, ok := appExt["rp"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected rp extension in App.Ext")
	}

	siteID, ok := rpData["site_id"].(float64)
	if !ok || int(siteID) != 456 {
		t.Errorf("Expected site_id 456 in App.Ext.rp, got %v", rpData["site_id"])
	}

	// Verify App.Publisher exists and has proper ID
	if parsed.App.Publisher == nil {
		t.Fatal("Expected App.Publisher to be present")
	}

	if parsed.App.Publisher.ID != "123" {
		t.Errorf("Expected App.Publisher.ID to be '123', got '%s'", parsed.App.Publisher.ID)
	}

	// Verify App.Publisher.Ext has Rubicon account ID
	var pubExt map[string]interface{}
	if err := json.Unmarshal(parsed.App.Publisher.Ext, &pubExt); err != nil {
		t.Fatalf("Failed to parse App.Publisher.Ext: %v", err)
	}

	rpPubData, ok := pubExt["rp"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected rp extension in App.Publisher.Ext")
	}

	accountID, ok := rpPubData["account_id"].(float64)
	if !ok || int(accountID) != 123 {
		t.Errorf("Expected account_id 123 in App.Publisher.Ext.rp, got %v", rpPubData["account_id"])
	}
}

// TestPreserveImpExtRPTarget tests Task #22: Preserve existing imp.ext.rp.target
func TestPreserveImpExtRPTarget(t *testing.T) {
	adapter := New("")

	// Create imp.ext with existing rp.target
	impExt := map[string]interface{}{
		"rubicon": map[string]interface{}{
			"accountId": 123,
			"siteId":    456,
			"zoneId":    789,
		},
		"rp": map[string]interface{}{
			"target": map[string]interface{}{
				"key1": "value1",
				"key2": []string{"val2a", "val2b"},
			},
		},
	}
	impExtBytes, _ := json.Marshal(impExt)

	request := &openrtb.BidRequest{
		ID: "test-preserve-target",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: impExtBytes},
		},
		Site: &openrtb.Site{Domain: "example.com"},
	}

	requests, errs := adapter.MakeRequests(request, nil)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	var parsed openrtb.BidRequest
	if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Verify imp.ext.rp.target was preserved
	var parsedImpExt map[string]interface{}
	if err := json.Unmarshal(parsed.Imp[0].Ext, &parsedImpExt); err != nil {
		t.Fatalf("Failed to parse Imp.Ext: %v", err)
	}

	rpData, ok := parsedImpExt["rp"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected rp extension in Imp.Ext")
	}

	target, ok := rpData["target"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected target to be preserved in Imp.Ext.rp")
	}

	if target["key1"] != "value1" {
		t.Errorf("Expected target.key1 to be 'value1', got %v", target["key1"])
	}
}

// TestCreateSitePublisherWhenNil tests Task #23: Create Site.Publisher when nil
func TestCreateSitePublisherWhenNil(t *testing.T) {
	adapter := New("")

	rubiconExt := json.RawMessage(`{"rubicon":{"accountId":123,"siteId":456,"zoneId":789}}`)
	request := &openrtb.BidRequest{
		ID: "test-nil-publisher",
		Imp: []openrtb.Imp{
			{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: rubiconExt},
		},
		Site: &openrtb.Site{
			Domain: "example.com",
			// Publisher is intentionally nil
		},
	}

	requests, errs := adapter.MakeRequests(request, nil)

	if len(errs) > 0 {
		t.Fatalf("Unexpected errors: %v", errs)
	}

	var parsed openrtb.BidRequest
	if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
		t.Fatalf("Failed to parse request body: %v", err)
	}

	// Verify Site.Publisher was created
	if parsed.Site == nil {
		t.Fatal("Expected Site to be present")
	}

	if parsed.Site.Publisher == nil {
		t.Fatal("Expected Site.Publisher to be created when nil")
	}

	if parsed.Site.Publisher.ID != "123" {
		t.Errorf("Expected Publisher.ID to be '123', got '%s'", parsed.Site.Publisher.ID)
	}
}

// TestEnsurePublisherIDAlwaysSet tests Task #25: Ensure publisher.id always set
func TestEnsurePublisherIDAlwaysSet(t *testing.T) {
	adapter := New("")

	rubiconExt := json.RawMessage(`{"rubicon":{"accountId":999,"siteId":456,"zoneId":789}}`)

	testCases := []struct {
		name        string
		request     *openrtb.BidRequest
		expectedID  string
		checkSite   bool
		checkApp    bool
	}{
		{
			name: "Site with existing publisher",
			request: &openrtb.BidRequest{
				ID:   "test-site-existing-pub",
				Imp:  []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: rubiconExt}},
				Site: &openrtb.Site{Domain: "example.com", Publisher: &openrtb.Publisher{ID: "old-id"}},
			},
			expectedID: "999",
			checkSite:  true,
		},
		{
			name: "Site with nil publisher",
			request: &openrtb.BidRequest{
				ID:   "test-site-nil-pub",
				Imp:  []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: rubiconExt}},
				Site: &openrtb.Site{Domain: "example.com"},
			},
			expectedID: "999",
			checkSite:  true,
		},
		{
			name: "App with nil publisher",
			request: &openrtb.BidRequest{
				ID:  "test-app-nil-pub",
				Imp: []openrtb.Imp{{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: rubiconExt}},
				App: &openrtb.App{Bundle: "com.example.app"},
			},
			expectedID: "999",
			checkApp:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			requests, errs := adapter.MakeRequests(tc.request, nil)

			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			var parsed openrtb.BidRequest
			if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
				t.Fatalf("Failed to parse request body: %v", err)
			}

			if tc.checkSite {
				if parsed.Site == nil || parsed.Site.Publisher == nil {
					t.Fatal("Expected Site.Publisher to be present")
				}
				if parsed.Site.Publisher.ID != tc.expectedID {
					t.Errorf("Expected Site.Publisher.ID to be '%s', got '%s'", tc.expectedID, parsed.Site.Publisher.ID)
				}
			}

			if tc.checkApp {
				if parsed.App == nil || parsed.App.Publisher == nil {
					t.Fatal("Expected App.Publisher to be present")
				}
				if parsed.App.Publisher.ID != tc.expectedID {
					t.Errorf("Expected App.Publisher.ID to be '%s', got '%s'", tc.expectedID, parsed.App.Publisher.ID)
				}
			}
		})
	}
}

// TestPreserveValidTrackingData tests Task #26: Don't overwrite valid tracking data
func TestPreserveValidTrackingData(t *testing.T) {
	adapter := New("")

	testCases := []struct {
		name            string
		impExt          map[string]interface{}
		expectMint      string
		expectVersion   string
		shouldPreserve  bool
	}{
		{
			name: "Valid tracking data with both fields",
			impExt: map[string]interface{}{
				"rubicon": map[string]interface{}{"accountId": 123, "siteId": 456, "zoneId": 789},
				"rp": map[string]interface{}{
					"track": map[string]interface{}{
						"mint":         "abc123",
						"mint_version": "1.0",
					},
				},
			},
			expectMint:     "abc123",
			expectVersion:  "1.0",
			shouldPreserve: true,
		},
		{
			name: "Valid tracking data with only mint",
			impExt: map[string]interface{}{
				"rubicon": map[string]interface{}{"accountId": 123, "siteId": 456, "zoneId": 789},
				"rp": map[string]interface{}{
					"track": map[string]interface{}{
						"mint":         "xyz789",
						"mint_version": "",
					},
				},
			},
			expectMint:     "xyz789",
			expectVersion:  "",
			shouldPreserve: true,
		},
		{
			name: "Empty tracking data - use defaults",
			impExt: map[string]interface{}{
				"rubicon": map[string]interface{}{"accountId": 123, "siteId": 456, "zoneId": 789},
				"rp": map[string]interface{}{
					"track": map[string]interface{}{
						"mint":         "",
						"mint_version": "",
					},
				},
			},
			expectMint:     "",
			expectVersion:  "",
			shouldPreserve: false,
		},
		{
			name: "No tracking data - use defaults",
			impExt: map[string]interface{}{
				"rubicon": map[string]interface{}{"accountId": 123, "siteId": 456, "zoneId": 789},
			},
			expectMint:     "",
			expectVersion:  "",
			shouldPreserve: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			impExtBytes, _ := json.Marshal(tc.impExt)

			request := &openrtb.BidRequest{
				ID: "test-tracking",
				Imp: []openrtb.Imp{
					{ID: "imp-1", Banner: &openrtb.Banner{W: 300, H: 250}, Ext: impExtBytes},
				},
				Site: &openrtb.Site{Domain: "example.com"},
			}

			requests, errs := adapter.MakeRequests(request, nil)

			if len(errs) > 0 {
				t.Fatalf("Unexpected errors: %v", errs)
			}

			var parsed openrtb.BidRequest
			if err := json.Unmarshal(requests[0].Body, &parsed); err != nil {
				t.Fatalf("Failed to parse request body: %v", err)
			}

			var parsedImpExt map[string]interface{}
			if err := json.Unmarshal(parsed.Imp[0].Ext, &parsedImpExt); err != nil {
				t.Fatalf("Failed to parse Imp.Ext: %v", err)
			}

			rpData, ok := parsedImpExt["rp"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected rp extension in Imp.Ext")
			}

			track, ok := rpData["track"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected track in Imp.Ext.rp")
			}

			mint := track["mint"].(string)
			mintVersion := track["mint_version"].(string)

			if mint != tc.expectMint {
				t.Errorf("Expected mint '%s', got '%s'", tc.expectMint, mint)
			}

			if mintVersion != tc.expectVersion {
				t.Errorf("Expected mint_version '%s', got '%s'", tc.expectVersion, mintVersion)
			}
		})
	}
}
