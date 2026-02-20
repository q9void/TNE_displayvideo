package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

func TestPrivacyMiddleware_NoGDPR(t *testing.T) {
	// Request without GDPR signal should pass through
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-1",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		// No Regs field - GDPR doesn't apply
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when GDPR doesn't apply")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_GDPRWithValidConsent(t *testing.T) {
	// Request with GDPR=1 and valid consent should pass through
	// Note: Using StrictMode=false to allow valid format without purpose consent check
	config := DefaultPrivacyConfig()
	config.StrictMode = false // Don't require specific purpose consents
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	// This is a real TCF v2 consent string (base64url encoded)
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	req := &openrtb.BidRequest{
		ID:  "test-2",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		User: &openrtb.User{
			Consent: validConsent,
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called with valid consent")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_GDPRNoConsent(t *testing.T) {
	// Request with GDPR=1 but no consent should be blocked
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	req := &openrtb.BidRequest{
		ID:  "test-3",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		// No User.Consent - GDPR violation
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called without consent when GDPR applies")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	// Check error response
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "GDPR" {
		t.Errorf("Expected regulation=GDPR, got %v", resp["regulation"])
	}
}

func TestPrivacyMiddleware_GDPRInvalidConsent(t *testing.T) {
	// Request with GDPR=1 but invalid consent string should be blocked
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	req := &openrtb.BidRequest{
		ID:  "test-4",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		User: &openrtb.User{
			Consent: "invalid-not-base64", // Invalid consent string
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called with invalid consent")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_COPPA(t *testing.T) {
	// COPPA requests should be blocked by default
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-5",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			COPPA: 1, // Child-directed content
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called for COPPA requests")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "COPPA" {
		t.Errorf("Expected regulation=COPPA, got %v", resp["regulation"])
	}
}

func TestPrivacyMiddleware_GETRequest(t *testing.T) {
	// GET requests should pass through without privacy checks
	config := DefaultPrivacyConfig()
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	httpReq := httptest.NewRequest(http.MethodGet, "/openrtb2/auction", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called for GET requests")
	}
}

func TestPrivacyMiddleware_DisabledGDPR(t *testing.T) {
	// When GDPR enforcement is disabled, requests should pass through
	config := DefaultPrivacyConfig()
	config.EnforceGDPR = false
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	req := &openrtb.BidRequest{
		ID:  "test-6",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		// No consent - would normally be blocked
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when GDPR enforcement is disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestIsValidTCFv2String(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	tests := []struct {
		name    string
		consent string
		valid   bool
	}{
		{"empty", "", false},
		{"too short", "abc", false},
		{"invalid base64", "not-valid-base64-!!!", false},
		{"valid TCF v2", "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA", true},
		// Add more test cases as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.isValidTCFv2String(tt.consent)
			if result != tt.valid {
				t.Errorf("isValidTCFv2String(%q) = %v, want %v", tt.consent, result, tt.valid)
			}
		})
	}
}

func TestPrivacyMiddleware_CCPAOptOut(t *testing.T) {
	// When CCPA enforcement is enabled and user opts out, request should be blocked
	config := DefaultPrivacyConfig()
	config.EnforceCCPA = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-ccpa-1",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			USPrivacy: "1YYN", // User has opted out (Y in position 3)
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called when user opts out under CCPA")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "CCPA" {
		t.Errorf("Expected regulation=CCPA, got %v", resp["regulation"])
	}
}

func TestPrivacyMiddleware_CCPANoOptOut(t *testing.T) {
	// When CCPA enforcement is enabled but user doesn't opt out, request should pass
	config := DefaultPrivacyConfig()
	config.EnforceCCPA = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-ccpa-2",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			USPrivacy: "1YNN", // User has NOT opted out (N in position 3)
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when user doesn't opt out")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestPrivacyMiddleware_CCPADisabled(t *testing.T) {
	// When CCPA enforcement is disabled, opt-out should be ignored
	config := DefaultPrivacyConfig()
	config.EnforceCCPA = false
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-ccpa-3",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			USPrivacy: "1YYN", // User has opted out, but enforcement is disabled
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when CCPA enforcement is disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestParseTCFv2String(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	// Valid TCF v2 consent string (base64url encoded)
	// This is a minimal valid consent string
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	data, err := m.parseTCFv2String(validConsent)
	if err != nil {
		t.Fatalf("parseTCFv2String failed: %v", err)
	}

	// Check that version is 2
	if data.Version != 2 {
		t.Errorf("Expected version 2, got %d", data.Version)
	}

	// Check that purpose consents slice is populated
	if len(data.PurposeConsents) == 0 {
		t.Error("Expected purpose consents to be populated")
	}
}

func TestCheckPurposeConsents(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	// Create test TCF data with some purposes granted
	data := &TCFv2Data{
		Version:         2,
		PurposeConsents: make([]bool, 24),
	}

	// Grant purposes 1, 2, 7 (required for programmatic ads)
	data.PurposeConsents[0] = true // Purpose 1
	data.PurposeConsents[1] = true // Purpose 2
	data.PurposeConsents[6] = true // Purpose 7

	// Check required purposes
	missing := m.checkPurposeConsents(data, RequiredPurposes)
	if len(missing) != 0 {
		t.Errorf("Expected no missing purposes, got %v", missing)
	}

	// Remove purpose 2 consent
	data.PurposeConsents[1] = false
	missing = m.checkPurposeConsents(data, RequiredPurposes)
	if len(missing) != 1 || missing[0] != 2 {
		t.Errorf("Expected missing purpose 2, got %v", missing)
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		defaultVal bool
		expected   bool
	}{
		{"empty returns default true", "", true, true},
		{"empty returns default false", "", false, false},
		{"true string", "true", false, true},
		{"TRUE string", "TRUE", false, true},
		{"1 string", "1", false, true},
		{"false string", "false", true, false},
		{"0 string", "0", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			key := "TEST_ENV_BOOL_" + tt.name
			if tt.envValue != "" {
				t.Setenv(key, tt.envValue)
			}

			result := getEnvBool(key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("getEnvBool(%q, %v) = %v, want %v", key, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

// P2-2: IP Anonymization Tests

func TestAnonymizeIP_IPv4(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard IPv4", "192.168.1.100", "192.168.1.0"},
		{"already anonymized", "192.168.1.0", "192.168.1.0"},
		{"loopback", "127.0.0.1", "127.0.0.0"},
		{"public IP", "8.8.8.8", "8.8.8.0"},
		{"max octets", "255.255.255.255", "255.255.255.0"},
		{"zeros except last", "0.0.0.100", "0.0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnonymizeIP(tt.input)
			if result != tt.expected {
				t.Errorf("AnonymizeIP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAnonymizeIP_IPv6(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"full IPv6",
			"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			"2001:db8:85a3::",
		},
		{
			"compressed IPv6",
			"2001:db8:85a3::8a2e:370:7334",
			"2001:db8:85a3::",
		},
		{
			"loopback IPv6",
			"::1",
			"::",
		},
		{
			"link-local",
			"fe80::1",
			"fe80::",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnonymizeIP(tt.input)
			if result != tt.expected {
				t.Errorf("AnonymizeIP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAnonymizeIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"invalid IP", "not-an-ip", ""},
		{"malformed", "192.168.1", ""},
		{"too many octets", "192.168.1.1.1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnonymizeIP(tt.input)
			if result != tt.expected {
				t.Errorf("AnonymizeIP(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPrivacyMiddleware_IPAnonymization(t *testing.T) {
	// Test that IP addresses are anonymized when GDPR applies
	config := DefaultPrivacyConfig()
	config.StrictMode = false // Don't require specific purpose consents
	config.AnonymizeIP = true // Enable IP anonymization
	mw := NewPrivacyMiddleware(config)

	var capturedBody []byte
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = json.Marshal(r.Body)
		// Read the body that was passed to us
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	req := &openrtb.BidRequest{
		ID:  "test-ip-anon",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		User: &openrtb.User{
			Consent: validConsent,
		},
		Device: &openrtb.Device{
			IP:   "192.168.1.100",
			IPv6: "2001:db8:85a3::8a2e:370:7334",
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	// Parse the captured body to verify IP anonymization
	var modifiedReq openrtb.BidRequest
	if err := json.Unmarshal(capturedBody, &modifiedReq); err != nil {
		t.Fatalf("Failed to parse modified request: %v", err)
	}

	if modifiedReq.Device == nil {
		t.Fatal("Device should not be nil")
	}

	if modifiedReq.Device.IP != "192.168.1.0" {
		t.Errorf("Expected anonymized IPv4 '192.168.1.0', got %q", modifiedReq.Device.IP)
	}

	if modifiedReq.Device.IPv6 != "2001:db8:85a3::" {
		t.Errorf("Expected anonymized IPv6 '2001:db8:85a3::', got %q", modifiedReq.Device.IPv6)
	}
}

func TestPrivacyMiddleware_IPAnonymizationDisabled(t *testing.T) {
	// Test that IP addresses are NOT anonymized when AnonymizeIP is false
	config := DefaultPrivacyConfig()
	config.StrictMode = false
	config.AnonymizeIP = false // Disable IP anonymization
	mw := NewPrivacyMiddleware(config)

	var capturedBody []byte
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
	}))

	gdpr := 1
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	req := &openrtb.BidRequest{
		ID:  "test-ip-no-anon",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Regs: &openrtb.Regs{
			GDPR: &gdpr,
		},
		User: &openrtb.User{
			Consent: validConsent,
		},
		Device: &openrtb.Device{
			IP:   "192.168.1.100",
			IPv6: "2001:db8:85a3::8a2e:370:7334",
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	// Parse the captured body to verify IPs are NOT anonymized
	var modifiedReq openrtb.BidRequest
	if err := json.Unmarshal(capturedBody, &modifiedReq); err != nil {
		t.Fatalf("Failed to parse request: %v", err)
	}

	if modifiedReq.Device.IP != "192.168.1.100" {
		t.Errorf("Expected original IPv4 '192.168.1.100', got %q", modifiedReq.Device.IP)
	}

	if modifiedReq.Device.IPv6 != "2001:db8:85a3::8a2e:370:7334" {
		t.Errorf("Expected original IPv6, got %q", modifiedReq.Device.IPv6)
	}
}

func TestPrivacyMiddleware_NoAnonymizationWithoutGDPR(t *testing.T) {
	// Test that IP addresses are NOT anonymized when GDPR doesn't apply
	config := DefaultPrivacyConfig()
	config.AnonymizeIP = true // Enable, but GDPR won't apply
	mw := NewPrivacyMiddleware(config)

	var capturedBody []byte
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		capturedBody = body
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-no-gdpr",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		// No GDPR signal
		Device: &openrtb.Device{
			IP: "192.168.1.100",
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	// Parse and verify original IP is preserved
	var modifiedReq openrtb.BidRequest
	json.Unmarshal(capturedBody, &modifiedReq)

	if modifiedReq.Device.IP != "192.168.1.100" {
		t.Errorf("Expected original IP without GDPR, got %q", modifiedReq.Device.IP)
	}
}

// Vendor Consent Validation Tests

func TestCheckVendorConsent(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	// Valid TCF v2 consent string
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	tests := []struct {
		name          string
		consentString string
		gvlID         int
		expectConsent bool
	}{
		{"empty consent string", "", 123, false},
		{"zero GVL ID", validConsent, 0, false},
		{"negative GVL ID", validConsent, -1, false},
		{"invalid consent string", "invalid", 123, false},
		{"valid consent with vendor", validConsent, 1, false}, // This consent string may not have vendor 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.CheckVendorConsent(tt.consentString, tt.gvlID)
			if result != tt.expectConsent {
				t.Errorf("CheckVendorConsent(%q, %d) = %v, want %v",
					tt.consentString, tt.gvlID, result, tt.expectConsent)
			}
		})
	}
}

func TestCheckVendorConsents_Multiple(t *testing.T) {
	m := &PrivacyMiddleware{config: DefaultPrivacyConfig()}

	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	t.Run("empty consent string", func(t *testing.T) {
		gvlIDs := []int{1, 2, 3}
		results := m.CheckVendorConsents("", gvlIDs)

		if len(results) != len(gvlIDs) {
			t.Errorf("Expected %d results, got %d", len(gvlIDs), len(results))
		}

		// All should be false with empty consent
		for gvlID, hasConsent := range results {
			if hasConsent {
				t.Errorf("Expected vendor %d to have no consent with empty string", gvlID)
			}
		}
	})

	t.Run("invalid consent string", func(t *testing.T) {
		gvlIDs := []int{1, 2, 3}
		results := m.CheckVendorConsents("invalid-consent", gvlIDs)

		for gvlID, hasConsent := range results {
			if hasConsent {
				t.Errorf("Expected vendor %d to have no consent with invalid string", gvlID)
			}
		}
	})

	t.Run("valid consent string", func(t *testing.T) {
		gvlIDs := []int{1, 2, 3}
		results := m.CheckVendorConsents(validConsent, gvlIDs)

		if len(results) != len(gvlIDs) {
			t.Errorf("Expected %d results, got %d", len(gvlIDs), len(results))
		}
	})
}

func TestCheckVendorConsentStatic(t *testing.T) {
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"

	tests := []struct {
		name          string
		consentString string
		gvlID         int
		expectConsent bool
	}{
		{"empty consent", "", 123, false},
		{"zero GVL ID", validConsent, 0, false},
		{"negative GVL ID", validConsent, -1, false},
		{"invalid consent", "not-valid", 123, false},
		{"valid inputs", validConsent, 1, false}, // May not have vendor 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckVendorConsentStatic(tt.consentString, tt.gvlID)
			if result != tt.expectConsent {
				t.Errorf("CheckVendorConsentStatic(%q, %d) = %v, want %v",
					tt.consentString, tt.gvlID, result, tt.expectConsent)
			}
		})
	}
}

func TestDetectRegulationFromGeo(t *testing.T) {
	tests := []struct {
		name       string
		geo        *openrtb.Geo
		regulation PrivacyRegulation
	}{
		{
			"nil geo",
			nil,
			RegulationNone,
		},
		{
			"Germany (GDPR)",
			&openrtb.Geo{Country: "DE"},
			RegulationGDPR,
		},
		{
			"France (GDPR)",
			&openrtb.Geo{Country: "FR"},
			RegulationGDPR,
		},
		{
			"United Kingdom (GDPR)",
			&openrtb.Geo{Country: "GB"},
			RegulationGDPR,
		},
		{
			"California (CCPA)",
			&openrtb.Geo{Country: "US", Region: "CA"},
			RegulationCCPA,
		},
		{
			"Virginia (VCDPA)",
			&openrtb.Geo{Country: "US", Region: "VA"},
			RegulationVCDPA,
		},
		{
			"Colorado (CPA)",
			&openrtb.Geo{Country: "US", Region: "CO"},
			RegulationCPA,
		},
		{
			"Connecticut (CTDPA)",
			&openrtb.Geo{Country: "US", Region: "CT"},
			RegulationCTDPA,
		},
		{
			"Utah (UCPA)",
			&openrtb.Geo{Country: "US", Region: "UT"},
			RegulationUCPA,
		},
		{
			"Texas (no regulation)",
			&openrtb.Geo{Country: "US", Region: "TX"},
			RegulationNone,
		},
		{
			"Brazil (LGPD)",
			&openrtb.Geo{Country: "BR"},
			RegulationLGPD,
		},
		{
			"Canada (PIPEDA)",
			&openrtb.Geo{Country: "CA"},
			RegulationPIPEDA,
		},
		{
			"Singapore (PDPA)",
			&openrtb.Geo{Country: "SG"},
			RegulationPDPA,
		},
		{
			"Japan (no regulation)",
			&openrtb.Geo{Country: "JP"},
			RegulationNone,
		},
		{
			"Empty country",
			&openrtb.Geo{Country: ""},
			RegulationNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRegulationFromGeo(tt.geo)
			if result != tt.regulation {
				t.Errorf("DetectRegulationFromGeo(%+v) = %v, want %v",
					tt.geo, result, tt.regulation)
			}
		})
	}
}

func TestShouldFilterBidderByGeo_GDPR(t *testing.T) {
	validConsent := "CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA"
	gdpr := 1

	tests := []struct {
		name          string
		req           *openrtb.BidRequest
		gvlID         int
		shouldFilter  bool
		description   string
	}{
		{
			"nil request",
			nil,
			123,
			false,
			"nil request should not filter",
		},
		{
			"no geo data",
			&openrtb.BidRequest{
				ID: "test",
			},
			123,
			false,
			"no geo data should not filter",
		},
		{
			"EU country with GDPR and valid consent",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "DE"},
				},
				Regs: &openrtb.Regs{GDPR: &gdpr},
				User: &openrtb.User{Consent: validConsent},
			},
			1,
			true, // Will filter because vendor 1 likely not in consent
			"EU with consent but vendor not consented",
		},
		{
			"EU country with GDPR but no consent string",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "FR"},
				},
				Regs: &openrtb.Regs{GDPR: &gdpr},
				User: &openrtb.User{Consent: ""},
			},
			123,
			true,
			"EU with GDPR but no consent should filter",
		},
		{
			"EU country but GDPR not set",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "DE"},
				},
			},
			123,
			false,
			"EU country but no GDPR flag should not filter",
		},
		{
			"EU country with zero GVL ID",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "DE"},
				},
				Regs: &openrtb.Regs{GDPR: &gdpr},
				User: &openrtb.User{Consent: validConsent},
			},
			0,
			false,
			"zero GVL ID should not filter even in GDPR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldFilterBidderByGeo(tt.req, tt.gvlID)
			if result != tt.shouldFilter {
				t.Errorf("%s: ShouldFilterBidderByGeo() = %v, want %v",
					tt.description, result, tt.shouldFilter)
			}
		})
	}
}

func TestShouldFilterBidderByGeo_CCPA(t *testing.T) {
	tests := []struct {
		name         string
		req          *openrtb.BidRequest
		gvlID        int
		shouldFilter bool
		description  string
	}{
		{
			"California with opt-out",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "US", Region: "CA"},
				},
				Regs: &openrtb.Regs{
					USPrivacy: "1YYN", // User opted out
				},
			},
			123,
			true,
			"CA with opt-out should filter",
		},
		{
			"California without opt-out",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "US", Region: "CA"},
				},
				Regs: &openrtb.Regs{
					USPrivacy: "1YNN", // User did NOT opt out
				},
			},
			123,
			false,
			"CA without opt-out should not filter",
		},
		{
			"Virginia with opt-out",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "US", Region: "VA"},
				},
				Regs: &openrtb.Regs{
					USPrivacy: "1YYN",
				},
			},
			123,
			true,
			"VA with opt-out should filter",
		},
		{
			"Colorado with opt-out",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "US", Region: "CO"},
				},
				Regs: &openrtb.Regs{
					USPrivacy: "1YYN",
				},
			},
			123,
			true,
			"CO with opt-out should filter",
		},
		{
			"Privacy state but no USPrivacy string",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "US", Region: "CA"},
				},
			},
			123,
			false,
			"no USPrivacy string should not filter",
		},
		{
			"Privacy state with malformed USPrivacy",
			&openrtb.BidRequest{
				ID: "test",
				Device: &openrtb.Device{
					Geo: &openrtb.Geo{Country: "US", Region: "CA"},
				},
				Regs: &openrtb.Regs{
					USPrivacy: "1Y", // Too short
				},
			},
			123,
			false,
			"malformed USPrivacy should not filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldFilterBidderByGeo(tt.req, tt.gvlID)
			if result != tt.shouldFilter {
				t.Errorf("%s: ShouldFilterBidderByGeo() = %v, want %v",
					tt.description, result, tt.shouldFilter)
			}
		})
	}
}

func TestShouldFilterBidderByGeo_OtherRegulations(t *testing.T) {
	tests := []struct {
		name        string
		geo         *openrtb.Geo
		shouldFilter bool
	}{
		{"Brazil (LGPD)", &openrtb.Geo{Country: "BR"}, false},
		{"Canada (PIPEDA)", &openrtb.Geo{Country: "CA"}, false},
		{"Singapore (PDPA)", &openrtb.Geo{Country: "SG"}, false},
		{"No regulation country", &openrtb.Geo{Country: "JP"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &openrtb.BidRequest{
				ID:     "test",
				Device: &openrtb.Device{Geo: tt.geo},
			}
			result := ShouldFilterBidderByGeo(req, 123)
			if result != tt.shouldFilter {
				t.Errorf("ShouldFilterBidderByGeo() for %s = %v, want %v",
					tt.name, result, tt.shouldFilter)
			}
		})
	}
}

func TestValidateGeoConsent_EUWithoutGDPR(t *testing.T) {
	// EU user detected but GDPR flag not set
	config := DefaultPrivacyConfig()
	config.GeoEnforcement = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-geo-1",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Device: &openrtb.Device{
			Geo: &openrtb.Geo{Country: "DE"}, // Germany
		},
		// Missing: Regs.GDPR = 1
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called - EU user without GDPR flag")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "GDPR" {
		t.Errorf("Expected regulation=GDPR, got %v", resp["regulation"])
	}
}

func TestValidateGeoConsent_CaliforniaWithoutUSPrivacy(t *testing.T) {
	// California user detected but USPrivacy string not provided
	config := DefaultPrivacyConfig()
	config.GeoEnforcement = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-geo-2",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Device: &openrtb.Device{
			Geo: &openrtb.Geo{Country: "US", Region: "CA"}, // California
		},
		// Missing: Regs.USPrivacy
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called - CA user without USPrivacy string")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["regulation"] != "CCPA" {
		t.Errorf("Expected regulation=CCPA, got %v", resp["regulation"])
	}
}

func TestValidateGeoConsent_GeoEnforcementDisabled(t *testing.T) {
	// When geo enforcement is disabled, EU users without GDPR flag should pass
	config := DefaultPrivacyConfig()
	config.GeoEnforcement = false // Disabled
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-geo-3",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		Device: &openrtb.Device{
			Geo: &openrtb.Geo{Country: "DE"}, // Germany
		},
		// No GDPR flag, but geo enforcement is disabled
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if !called {
		t.Error("Handler should have been called when geo enforcement is disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestValidateGeoConsent_UserGeoFallback(t *testing.T) {
	// Test that user.geo is used when device.geo is not available
	config := DefaultPrivacyConfig()
	config.GeoEnforcement = true
	mw := NewPrivacyMiddleware(config)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := &openrtb.BidRequest{
		ID:  "test-geo-4",
		Imp: []openrtb.Imp{{ID: "imp1", Banner: &openrtb.Banner{}}},
		// No device.geo
		User: &openrtb.User{
			Geo: &openrtb.Geo{Country: "FR"}, // France
		},
		// Missing: Regs.GDPR = 1
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/openrtb2/auction", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, httpReq)

	if called {
		t.Error("Handler should NOT have been called - EU user (via user.geo) without GDPR flag")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}
