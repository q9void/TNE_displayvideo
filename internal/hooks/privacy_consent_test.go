package hooks

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestPrivacyConsentHook_NoGDPR_AllowsIDs(t *testing.T) {
	hook := NewPrivacyConsentHook()
	ctx := context.Background()

	// Create request with user.ext.eids but no GDPR
	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "user-123",
			Ext: userExtBytes,
		},
		Site: &openrtb.Site{
			ID: "catalyst-account-12345",
			Publisher: &openrtb.Publisher{
				ID: "catalyst-pub-123",
			},
		},
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve user.ext.eids (no GDPR = allow)
	var resultExt map[string]interface{}
	json.Unmarshal(req.User.Ext, &resultExt)
	if resultExt["eids"] == nil {
		t.Error("expected user.ext.eids to be preserved (no GDPR)")
	}

	// Should always clear internal IDs (even without GDPR)
	if req.Site.ID != "" {
		t.Errorf("expected site.id cleared, got: %s", req.Site.ID)
	}
	if req.Site.Publisher.ID != "" {
		t.Errorf("expected publisher.id cleared, got: %s", req.Site.Publisher.ID)
	}
}

func TestPrivacyConsentHook_GDPR_NoConsent_StripsIDs(t *testing.T) {
	hook := NewPrivacyConsentHook()
	ctx := context.Background()

	// Create request with GDPR=1 but no consent string
	gdpr := int8(1)
	regsExt := map[string]interface{}{
		"gdpr": gdpr,
	}
	regsExtBytes, _ := json.Marshal(regsExt)

	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-123"},
				},
			},
		},
		// No consent string
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:       "user-123",
			BuyerUID: "buyer-uid-456",
			Ext:      userExtBytes,
		},
		Regs: &openrtb.Regs{
			Ext: regsExtBytes,
		},
		Site: &openrtb.Site{
			ID: "catalyst-account-12345",
		},
		Device: &openrtb.Device{
			IFA: "device-ifa-789",
		},
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should strip user IDs (GDPR applies, no consent)
	if req.User.ID != "" {
		t.Errorf("expected user.id cleared, got: %s", req.User.ID)
	}
	if req.User.BuyerUID != "" {
		t.Errorf("expected buyeruid cleared, got: %s", req.User.BuyerUID)
	}

	// Should strip user.ext.eids
	var resultExt map[string]interface{}
	json.Unmarshal(req.User.Ext, &resultExt)
	if resultExt["eids"] != nil {
		t.Error("expected user.ext.eids to be stripped (GDPR, no consent)")
	}

	// Should strip device IDs
	if req.Device.IFA != "" {
		t.Errorf("expected device.ifa cleared, got: %s", req.Device.IFA)
	}
}

func TestPrivacyConsentHook_GDPR_WithConsent_PreservesIDs(t *testing.T) {
	hook := NewPrivacyConsentHook()
	ctx := context.Background()

	// Create request with GDPR=1 AND valid consent string
	gdpr := int8(1)
	regsExt := map[string]interface{}{
		"gdpr": gdpr,
	}
	regsExtBytes, _ := json.Marshal(regsExt)

	userExt := map[string]interface{}{
		"consent": "COvzTO5OvzTO5AHABBENAlCsAP_AAH_AACiQGVNf_X_fb39j-_59_9t0eY1f9_7_v-0zjgeds-8Nyd_X_L8X42M7vF36pq4KuR4Eu3LBIQdlHOHcTUmw6IkVqTPsbk2Mr7NKJ7PEinMbe2dYGH9_n9XTuZKYr97s___z__-__v__7-f___-_____7AAAAA",
		"eids": []interface{}{
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "user-123",
			Ext: userExtBytes,
		},
		Regs: &openrtb.Regs{
			Ext: regsExtBytes,
		},
		Site: &openrtb.Site{
			ID: "catalyst-account-12345",
		},
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve user.ext.eids (GDPR, valid consent)
	var resultExt map[string]interface{}
	json.Unmarshal(req.User.Ext, &resultExt)
	if resultExt["eids"] == nil {
		t.Error("expected user.ext.eids to be preserved (GDPR with consent)")
	}
	if resultExt["consent"] == nil {
		t.Error("expected consent string to be preserved")
	}

	// Should still clear internal IDs
	if req.Site.ID != "" {
		t.Errorf("expected site.id cleared, got: %s", req.Site.ID)
	}
}

func TestPrivacyConsentHook_CCPA_OptOut_StripsIDs(t *testing.T) {
	hook := NewPrivacyConsentHook()
	ctx := context.Background()

	// Create request with CCPA opt-out (position 2 = 'Y')
	regsExt := map[string]interface{}{
		"us_privacy": "1YYN", // Version=1, Notice=Y, Opt-out=Y (user opted out)
	}
	regsExtBytes, _ := json.Marshal(regsExt)

	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "user-123",
			Ext: userExtBytes,
		},
		Regs: &openrtb.Regs{
			Ext: regsExtBytes,
		},
		Site: &openrtb.Site{
			ID: "catalyst-account-12345",
		},
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should strip user IDs (CCPA opt-out)
	if req.User.ID != "" {
		t.Errorf("expected user.id cleared (CCPA opt-out), got: %s", req.User.ID)
	}

	// Should strip user.ext.eids
	var resultExt map[string]interface{}
	json.Unmarshal(req.User.Ext, &resultExt)
	if resultExt["eids"] != nil {
		t.Error("expected user.ext.eids to be stripped (CCPA opt-out)")
	}
}

func TestPrivacyConsentHook_CCPA_NoOptOut_PreservesIDs(t *testing.T) {
	hook := NewPrivacyConsentHook()
	ctx := context.Background()

	// Create request with CCPA no opt-out (position 2 = 'N')
	regsExt := map[string]interface{}{
		"us_privacy": "1YNN", // Version=1, Notice=Y, Opt-out=N (user did NOT opt out)
	}
	regsExtBytes, _ := json.Marshal(regsExt)

	userExt := map[string]interface{}{
		"eids": []interface{}{
			map[string]interface{}{
				"source": "rubiconproject.com",
				"uids": []interface{}{
					map[string]interface{}{"id": "rubicon-uid-123"},
				},
			},
		},
	}
	userExtBytes, _ := json.Marshal(userExt)

	req := &openrtb.BidRequest{
		ID: "test-request-123",
		User: &openrtb.User{
			ID:  "user-123",
			Ext: userExtBytes,
		},
		Regs: &openrtb.Regs{
			Ext: regsExtBytes,
		},
		Site: &openrtb.Site{
			ID: "catalyst-account-12345",
		},
	}

	err := hook.ProcessRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve user.ext.eids (CCPA, no opt-out)
	var resultExt map[string]interface{}
	json.Unmarshal(req.User.Ext, &resultExt)
	if resultExt["eids"] == nil {
		t.Error("expected user.ext.eids to be preserved (CCPA, no opt-out)")
	}
}

func TestPrivacyConsentHook_AlwaysClearsInternalIDs(t *testing.T) {
	hook := NewPrivacyConsentHook()
	ctx := context.Background()

	tests := []struct {
		name     string
		hasSite  bool
		hasApp   bool
		siteID   string
		appID    string
		pubIDSite string
		pubIDApp  string
	}{
		{
			name:      "site with IDs",
			hasSite:   true,
			siteID:    "catalyst-account-12345",
			pubIDSite: "catalyst-pub-123",
		},
		{
			name:     "app with IDs",
			hasApp:   true,
			appID:    "catalyst-account-12345",
			pubIDApp: "catalyst-pub-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &openrtb.BidRequest{
				ID: "test-request-123",
			}

			if tt.hasSite {
				req.Site = &openrtb.Site{
					ID: tt.siteID,
					Publisher: &openrtb.Publisher{
						ID: tt.pubIDSite,
					},
				}
			}

			if tt.hasApp {
				req.App = &openrtb.App{
					ID: tt.appID,
					Publisher: &openrtb.Publisher{
						ID: tt.pubIDApp,
					},
				}
			}

			err := hook.ProcessRequest(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify IDs always cleared
			if tt.hasSite {
				if req.Site.ID != "" {
					t.Errorf("expected site.id cleared, got: %s", req.Site.ID)
				}
				if req.Site.Publisher.ID != "" {
					t.Errorf("expected site.publisher.id cleared, got: %s", req.Site.Publisher.ID)
				}
			}

			if tt.hasApp {
				if req.App.ID != "" {
					t.Errorf("expected app.id cleared, got: %s", req.App.ID)
				}
				if req.App.Publisher.ID != "" {
					t.Errorf("expected app.publisher.id cleared, got: %s", req.App.Publisher.ID)
				}
			}
		})
	}
}
