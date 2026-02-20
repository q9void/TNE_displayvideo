package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDefaultCORSConfig(t *testing.T) {
	// Clear environment first
	os.Unsetenv("CORS_ENABLED")
	os.Unsetenv("CORS_ORIGINS")
	os.Unsetenv("PBS_DEV_MODE")
	os.Unsetenv("CORS_ALLOW_ALL")
	os.Unsetenv("CORS_ALLOW_CREDENTIALS")

	config := DefaultCORSConfig()

	if !config.Enabled {
		t.Error("Expected CORS to be enabled by default")
	}

	if len(config.AllowedOrigins) != 0 {
		t.Error("Expected no allowed origins in production mode by default")
	}

	if len(config.AllowedMethods) == 0 {
		t.Error("Expected default allowed methods")
	}
}

func TestDefaultCORSConfig_DevMode(t *testing.T) {
	os.Setenv("PBS_DEV_MODE", "true")
	defer os.Unsetenv("PBS_DEV_MODE")

	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Error("Expected wildcard origin in dev mode")
	}
}

func TestDefaultCORSConfig_AllowAll(t *testing.T) {
	os.Setenv("CORS_ALLOW_ALL", "true")
	defer os.Unsetenv("CORS_ALLOW_ALL")

	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Error("Expected wildcard origin when CORS_ALLOW_ALL=true")
	}
}

func TestDefaultCORSConfig_ExplicitOrigins(t *testing.T) {
	os.Setenv("CORS_ORIGINS", "https://example.com,https://test.com")
	defer os.Unsetenv("CORS_ORIGINS")

	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 2 {
		t.Errorf("Expected 2 allowed origins, got %d", len(config.AllowedOrigins))
	}

	if config.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("Expected first origin https://example.com, got %s", config.AllowedOrigins[0])
	}
}

func TestDefaultCORSConfig_Disabled(t *testing.T) {
	os.Setenv("CORS_ENABLED", "false")
	defer os.Unsetenv("CORS_ENABLED")

	config := DefaultCORSConfig()

	if config.Enabled {
		t.Error("Expected CORS to be disabled when CORS_ENABLED=false")
	}
}

func TestDefaultCORSConfig_AllowCredentials(t *testing.T) {
	os.Setenv("CORS_ALLOW_CREDENTIALS", "true")
	defer os.Unsetenv("CORS_ALLOW_CREDENTIALS")

	config := DefaultCORSConfig()

	if !config.AllowCredentials {
		t.Error("Expected credentials to be allowed when CORS_ALLOW_CREDENTIALS=true")
	}
}

func TestParseCommaSeparated(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"  ", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a,,b", []string{"a", "b"}},
		{" , , ", nil},
	}

	for _, tc := range testCases {
		result := parseCommaSeparated(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("For input %q: expected %d items, got %d", tc.input, len(tc.expected), len(result))
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("For input %q: expected %v, got %v", tc.input, tc.expected, result)
				break
			}
		}
	}
}

func TestNewCORS(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
	}

	cors := NewCORS(config)

	if cors == nil {
		t.Fatal("Expected CORS instance to be created")
	}

	if cors.config.AllowedOrigins[0] != "https://example.com" {
		t.Error("Expected config to be set")
	}
}

func TestNewCORS_NilConfig(t *testing.T) {
	cors := NewCORS(nil)

	if cors == nil {
		t.Fatal("Expected CORS instance with default config")
	}

	if cors.config == nil {
		t.Error("Expected default config to be set")
	}
}

func TestMiddleware_Disabled(t *testing.T) {
	config := &CORSConfig{
		Enabled: false,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should not set CORS headers when disabled
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers when disabled")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
}

func TestMiddleware_AllowedOrigin(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected allowed origin to be echoed, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Vary") != "Origin" {
		t.Error("Expected Vary: Origin header")
	}
}

func TestMiddleware_DisallowedOrigin(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers for disallowed origin")
	}
}

func TestMiddleware_WildcardOrigin(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any-domain.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://any-domain.com" {
		t.Error("Expected origin to be allowed with wildcard")
	}
}

func TestMiddleware_WildcardSubdomain(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*.example.com"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		origin  string
		allowed bool
	}{
		{"https://sub.example.com", true},
		{"https://api.example.com", true},
		{"https://deep.sub.example.com", true},
		{"https://example.com", false},
		{"https://notexample.com", false},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		hasHeader := w.Header().Get("Access-Control-Allow-Origin") != ""
		if hasHeader != tc.allowed {
			t.Errorf("For origin %s: expected allowed=%v, got allowed=%v", tc.origin, tc.allowed, hasHeader)
		}
	}
}

func TestMiddleware_PreflightRequest(t *testing.T) {
	config := &CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called for OPTIONS
		t.Error("Handler should not be called for preflight request")
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 No Content for preflight, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" {
		t.Error("Expected allowed methods header")
	}

	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Error("Expected allowed headers")
	}

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Expected credentials header in preflight")
	}

	if w.Header().Get("Access-Control-Max-Age") != "3600" {
		t.Error("Expected max age header")
	}
}

func TestMiddleware_ExposedHeaders(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
		ExposedHeaders: []string{"X-Custom-Header", "X-Request-ID"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	exposedHeaders := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposedHeaders, "X-Custom-Header") {
		t.Error("Expected exposed headers to be set")
	}
}

func TestMiddleware_AllowCredentials(t *testing.T) {
	config := &CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Expected credentials header for non-preflight request")
	}
}

func TestIsOriginAllowed_ThreadSafe(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://example.com"},
	}
	cors := NewCORS(config)

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			allowed := cors.isOriginAllowed("https://example.com")
			if !allowed {
				t.Error("Expected origin to be allowed")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSetEnabled(t *testing.T) {
	config := &CORSConfig{
		Enabled: true,
	}
	cors := NewCORS(config)

	cors.SetEnabled(false)

	if cors.config.Enabled {
		t.Error("Expected CORS to be disabled after SetEnabled(false)")
	}

	cors.SetEnabled(true)

	if !cors.config.Enabled {
		t.Error("Expected CORS to be enabled after SetEnabled(true)")
	}
}

func TestSetAllowedOrigins(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://old.com"},
	}
	cors := NewCORS(config)

	newOrigins := []string{"https://new.com", "https://another.com"}
	cors.SetAllowedOrigins(newOrigins)

	if len(cors.config.AllowedOrigins) != 2 {
		t.Errorf("Expected 2 allowed origins, got %d", len(cors.config.AllowedOrigins))
	}

	if cors.config.AllowedOrigins[0] != "https://new.com" {
		t.Error("Expected origins to be updated")
	}
}

func TestMiddleware_NoOriginHeader(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No Origin header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should not crash, should process normally
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	// Should set Vary header even without origin
	if w.Header().Get("Vary") != "Origin" {
		t.Error("Expected Vary header even without origin")
	}
}

func TestMiddleware_EmptyAllowedOrigins(t *testing.T) {
	config := &CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{}, // No origins allowed
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should reject all origins when list is empty (secure by default)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers with empty allowed origins")
	}
}
