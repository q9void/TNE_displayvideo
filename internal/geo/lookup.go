// Package geo provides IP geolocation lookup using MaxMind GeoIP2
package geo

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/oschwald/geoip2-golang"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// GeoInfo contains geographic location data from IP lookup
type GeoInfo struct {
	Country   string  // ISO 3166-1 alpha-2 country code (US, GB, etc.)
	Region    string  // ISO 3166-2 subdivision code (CA, NY, etc.)
	City      string  // City name
	Metro     string  // Metro code (DMA code in US)
	Zip       string  // Postal code
	Lat       float64 // Latitude
	Lon       float64 // Longitude
	Accuracy  int     // Accuracy radius in km
	TimeZone  string  // IANA time zone (America/New_York, etc.)
}

// Service provides IP geolocation lookup
type Service struct {
	reader *geoip2.Reader
	mu     sync.RWMutex
	dbPath string
}

var (
	defaultService *Service
	serviceOnce    sync.Once
)

// NewService creates a new geo lookup service
// dbPath should point to a GeoIP2 City or GeoLite2 City database file
func NewService(dbPath string) (*Service, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("GeoIP2 database path is required")
	}

	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("GeoIP2 database file not found: %s", dbPath)
	}

	reader, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP2 database: %w", err)
	}

	return &Service{
		reader: reader,
		dbPath: dbPath,
	}, nil
}

// GetDefaultService returns the singleton geo service instance
// Database path is read from GEOIP2_DB_PATH environment variable
func GetDefaultService() (*Service, error) {
	var err error
	serviceOnce.Do(func() {
		dbPath := os.Getenv("GEOIP2_DB_PATH")
		if dbPath == "" {
			// Default path if not configured
			dbPath = "/usr/share/GeoIP/GeoLite2-City.mmdb"
		}

		defaultService, err = NewService(dbPath)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("db_path", dbPath).
				Msg("Failed to initialize GeoIP2 service - geo lookups will be unavailable")
		} else {
			logger.Log.Info().
				Str("db_path", dbPath).
				Msg("GeoIP2 service initialized successfully")
		}
	})

	return defaultService, err
}

// Lookup performs a geo lookup for the given IP address
func (s *Service) Lookup(ipString string) (*GeoInfo, error) {
	if s == nil || s.reader == nil {
		return nil, fmt.Errorf("geo service not initialized")
	}

	// Parse IP
	ip := net.ParseIP(ipString)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipString)
	}

	// Skip private/local IPs
	if isPrivateIP(ip) {
		return nil, fmt.Errorf("private IP address: %s", ipString)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Lookup city data
	record, err := s.reader.City(ip)
	if err != nil {
		return nil, fmt.Errorf("geo lookup failed: %w", err)
	}

	info := &GeoInfo{
		Country: record.Country.IsoCode,
		City:    record.City.Names["en"],
		Lat:     record.Location.Latitude,
		Lon:     record.Location.Longitude,
		Accuracy: int(record.Location.AccuracyRadius),
		TimeZone: record.Location.TimeZone,
	}

	// Region (subdivision)
	if len(record.Subdivisions) > 0 {
		info.Region = record.Subdivisions[0].IsoCode
	}

	// Postal code
	info.Zip = record.Postal.Code

	// Metro code (US DMA)
	if record.Location.MetroCode > 0 {
		info.Metro = fmt.Sprintf("%d", record.Location.MetroCode)
	}

	return info, nil
}

// LookupSafe performs a geo lookup and returns nil on error (no error returned)
// This is useful when geo data is optional and you want to continue on failure
func (s *Service) LookupSafe(ipString string) *GeoInfo {
	info, err := s.Lookup(ipString)
	if err != nil {
		logger.Log.Debug().
			Err(err).
			Str("ip", ipString).
			Msg("Geo lookup failed (non-fatal)")
		return nil
	}
	return info
}

// Close closes the geo database reader
func (s *Service) Close() error {
	if s == nil || s.reader == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reader.Close()
}

// isPrivateIP checks if an IP is private/local
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7", // IPv6 unique local
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// LookupOrDefault returns geo info or a default with just country code if lookup fails
func LookupOrDefault(ipString, defaultCountry string) *GeoInfo {
	service, err := GetDefaultService()
	if err != nil {
		// Return minimal default
		return &GeoInfo{Country: defaultCountry}
	}

	info := service.LookupSafe(ipString)
	if info == nil {
		// Return minimal default
		return &GeoInfo{Country: defaultCountry}
	}

	return info
}
