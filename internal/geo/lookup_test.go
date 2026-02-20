package geo

import (
	"net"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewService_MissingDBPath(t *testing.T) {
	_, err := NewService("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestNewService_InvalidPath(t *testing.T) {
	_, err := NewService("/nonexistent/path/to/db.mmdb")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestIsPrivateIP(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		// Private IPs
		{"127.0.0.1", true},      // Loopback
		{"10.0.0.1", true},       // Private class A
		{"172.16.0.1", true},     // Private class B
		{"192.168.1.1", true},    // Private class C
		{"169.254.1.1", true},    // Link local
		{"::1", true},            // IPv6 loopback
		{"fe80::1", true},        // IPv6 link local
		{"fc00::1", true},        // IPv6 unique local

		// Public IPs
		{"8.8.8.8", false},       // Google DNS
		{"1.1.1.1", false},       // Cloudflare DNS
		{"208.67.222.222", false}, // OpenDNS
		{"2001:4860:4860::8888", false}, // IPv6 Google DNS
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tc.ip)
			}
			result := isPrivateIP(ip)
			assert.Equal(t, tc.expected, result, "IP: %s", tc.ip)
		})
	}
}

// Note: These tests require a GeoIP2 database file
// Set GEOIP2_DB_PATH to run full integration tests

func TestLookup_WithValidIP(t *testing.T) {
	dbPath := os.Getenv("GEOIP2_DB_PATH")
	if dbPath == "" {
		t.Skip("Skipping test: GEOIP2_DB_PATH not set")
	}

	service, err := NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Test with Google DNS (known to be in US)
	info, err := service.Lookup("8.8.8.8")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "US", info.Country)
	assert.NotZero(t, info.Lat)
	assert.NotZero(t, info.Lon)
}

func TestLookup_WithPrivateIP(t *testing.T) {
	dbPath := os.Getenv("GEOIP2_DB_PATH")
	if dbPath == "" {
		t.Skip("Skipping test: GEOIP2_DB_PATH not set")
	}

	service, err := NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Should return error for private IP
	info, err := service.Lookup("192.168.1.1")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "private IP")
}

func TestLookup_WithInvalidIP(t *testing.T) {
	dbPath := os.Getenv("GEOIP2_DB_PATH")
	if dbPath == "" {
		t.Skip("Skipping test: GEOIP2_DB_PATH not set")
	}

	service, err := NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Should return error for invalid IP
	info, err := service.Lookup("not-an-ip")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "invalid IP")
}

func TestLookupSafe_ReturnsNilOnError(t *testing.T) {
	dbPath := os.Getenv("GEOIP2_DB_PATH")
	if dbPath == "" {
		t.Skip("Skipping test: GEOIP2_DB_PATH not set")
	}

	service, err := NewService(dbPath)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Should return nil (not error) for private IP
	info := service.LookupSafe("192.168.1.1")
	assert.Nil(t, info)

	// Should return nil (not error) for invalid IP
	info = service.LookupSafe("not-an-ip")
	assert.Nil(t, info)
}

func TestLookupOrDefault(t *testing.T) {
	// Test with no database configured
	originalPath := os.Getenv("GEOIP2_DB_PATH")
	os.Setenv("GEOIP2_DB_PATH", "")
	defer os.Setenv("GEOIP2_DB_PATH", originalPath)

	// Reset singleton for test
	defaultService = nil
	serviceOnce = sync.Once{}

	info := LookupOrDefault("8.8.8.8", "US")
	assert.NotNil(t, info)
	assert.Equal(t, "US", info.Country)

	// Reset singleton after test
	defaultService = nil
	serviceOnce = sync.Once{}
}

func TestGeoInfo_Fields(t *testing.T) {
	info := &GeoInfo{
		Country:  "US",
		Region:   "CA",
		City:     "San Francisco",
		Metro:    "807",
		Zip:      "94102",
		Lat:      37.7749,
		Lon:      -122.4194,
		Accuracy: 10,
		TimeZone: "America/Los_Angeles",
	}

	assert.Equal(t, "US", info.Country)
	assert.Equal(t, "CA", info.Region)
	assert.Equal(t, "San Francisco", info.City)
	assert.Equal(t, "807", info.Metro)
	assert.Equal(t, "94102", info.Zip)
	assert.Equal(t, 37.7749, info.Lat)
	assert.Equal(t, -122.4194, info.Lon)
	assert.Equal(t, 10, info.Accuracy)
	assert.Equal(t, "America/Los_Angeles", info.TimeZone)
}

func TestService_Close(t *testing.T) {
	// Test closing nil service
	var service *Service
	err := service.Close()
	assert.NoError(t, err)

	// Test closing service with nil reader
	service = &Service{}
	err = service.Close()
	assert.NoError(t, err)
}

func TestGetDefaultService_Singleton(t *testing.T) {
	// Reset singleton
	defaultService = nil
	serviceOnce = sync.Once{}

	// Set invalid path to test error handling
	os.Setenv("GEOIP2_DB_PATH", "/invalid/path.mmdb")
	defer os.Unsetenv("GEOIP2_DB_PATH")

	service1, err1 := GetDefaultService()
	service2, _ := GetDefaultService()

	// Should return same instance (singleton)
	assert.Equal(t, service1, service2)
	// First call should have an error
	assert.Error(t, err1)

	// Reset singleton after test
	defaultService = nil
	serviceOnce = sync.Once{}
}

// Benchmarks

func BenchmarkLookup(b *testing.B) {
	dbPath := os.Getenv("GEOIP2_DB_PATH")
	if dbPath == "" {
		b.Skip("Skipping benchmark: GEOIP2_DB_PATH not set")
	}

	service, err := NewService(dbPath)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.Lookup("8.8.8.8")
	}
}

func BenchmarkLookupSafe(b *testing.B) {
	dbPath := os.Getenv("GEOIP2_DB_PATH")
	if dbPath == "" {
		b.Skip("Skipping benchmark: GEOIP2_DB_PATH not set")
	}

	service, err := NewService(dbPath)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.LookupSafe("8.8.8.8")
	}
}
