package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/thenexusengine/tne_springwire/internal/exchange"
)

// ServerConfig holds all server configuration
type ServerConfig struct {
	// Server
	Port    string
	Timeout time.Duration

	// Database
	DatabaseConfig *DatabaseConfig

	// Redis
	RedisURL string

	// IDR
	IDREnabled bool
	IDRUrl     string
	IDRAPIKey  string

	// Currency
	CurrencyConversionEnabled bool
	DefaultCurrency           string

	// Privacy
	DisableGDPREnforcement bool

	// Cookie Sync
	HostURL string

	// CORS
	CORSOrigins []string
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// ParseConfig parses configuration from flags and environment variables
func ParseConfig() *ServerConfig {
	// Parse flags with environment variable fallbacks
	port := flag.String("port", getEnvOrDefault("PBS_PORT", "8000"), "Server port")
	idrURL := flag.String("idr-url", getEnvOrDefault("IDR_URL", "http://localhost:5050"), "IDR service URL")
	idrEnabled := flag.Bool("idr-enabled", getEnvBoolOrDefault("IDR_ENABLED", true), "Enable IDR integration")
	defaultTimeout := getEnvDurationOrDefault("PBS_TIMEOUT", 2500*time.Millisecond)
	timeout := flag.Duration("timeout", defaultTimeout, "Default auction timeout")
	flag.Parse()

	cfg := &ServerConfig{
		Port:                      *port,
		Timeout:                   *timeout,
		RedisURL:                  os.Getenv("REDIS_URL"),
		IDREnabled:                *idrEnabled,
		IDRUrl:                    *idrURL,
		IDRAPIKey:                 os.Getenv("IDR_API_KEY"),
		CurrencyConversionEnabled: os.Getenv("CURRENCY_CONVERSION_ENABLED") != "false",
		DefaultCurrency:           "USD",
		DisableGDPREnforcement:    os.Getenv("PBS_DISABLE_GDPR_ENFORCEMENT") == "true",
		HostURL:                   getEnvOrDefault("PBS_HOST_URL", "https://ads.thenexusengine.com"),
	}

	// Parse database config if DB_HOST is set
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.DatabaseConfig = &DatabaseConfig{
			Host:            dbHost,
			Port:            getEnvOrDefault("DB_PORT", "5432"),
			User:            getEnvOrDefault("DB_USER", "catalyst"),
			Password:        getEnvOrDefault("DB_PASSWORD", ""),
			Name:            getEnvOrDefault("DB_NAME", "catalyst"),
			SSLMode:         getEnvOrDefault("DB_SSL_MODE", "disable"),
			MaxConnections:  getEnvIntOrDefault("DB_MAX_CONNECTIONS", 100),
			MaxIdleConns:    getEnvIntOrDefault("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: time.Duration(getEnvIntOrDefault("DB_CONN_MAX_LIFETIME_SECONDS", 3600)) * time.Second,
		}
	}

	// Parse CORS origins
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins != "" {
		// Split by comma and trim whitespace
		origins := []string{}
		for _, origin := range splitAndTrim(corsOrigins, ",") {
			if origin != "" {
				origins = append(origins, origin)
			}
		}
		cfg.CORSOrigins = origins
	}

	return cfg
}

// ToExchangeConfig converts ServerConfig to exchange.Config
func (c *ServerConfig) ToExchangeConfig() *exchange.Config {
	return &exchange.Config{
		DefaultTimeout:     c.Timeout,
		MaxBidders:         50,
		IDREnabled:         c.IDREnabled,
		IDRServiceURL:      c.IDRUrl,
		IDRAPIKey:          c.IDRAPIKey,
		EventRecordEnabled: true,
		EventBufferSize:    100,
		CurrencyConv:       c.CurrencyConversionEnabled,
		DefaultCurrency:    c.DefaultCurrency,
	}
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBoolOrDefault returns the environment variable as bool or a default
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}

// getEnvIntOrDefault returns the environment variable as int or a default
func getEnvIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intVal
}

// getEnvDurationOrDefault returns the environment variable as duration or a default
func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}

// splitAndTrim splits a string by delimiter and trims whitespace from each part
func splitAndTrim(s, delimiter string) []string {
	parts := []string{}
	for _, part := range splitString(s, delimiter) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString splits a string by delimiter
func splitString(s, delimiter string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if i <= len(s)-len(delimiter) && s[i:i+len(delimiter)] == delimiter {
			result = append(result, current)
			current = ""
			i += len(delimiter) - 1
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	// Trim leading whitespace
	for start < end && isWhitespace(s[start]) {
		start++
	}

	// Trim trailing whitespace
	for end > start && isWhitespace(s[end-1]) {
		end--
	}

	return s[start:end]
}

// isWhitespace checks if a byte is whitespace
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// isProduction returns true if running in production environment
func isProduction() bool {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("ENV")
	}
	return env == "production" || env == "prod"
}

// Validate validates the server configuration
func (c *ServerConfig) Validate() error {
	// Validate port
	if c.Port == "" {
		return fmt.Errorf("port is required")
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("port must be numeric: %w", err)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be in range 1-65535, got %d", port)
	}

	// Validate timeout
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", c.Timeout)
	}

	if c.Timeout > 30*time.Second {
		return fmt.Errorf("timeout must be less than 30s, got %v", c.Timeout)
	}

	// Validate IDR configuration when enabled
	if c.IDREnabled {
		if c.IDRUrl == "" {
			return fmt.Errorf("IDR URL is required when IDR is enabled")
		}

		if c.IDRAPIKey == "" {
			return fmt.Errorf("IDR API key is required when IDR is enabled")
		}
	}

	// Validate database configuration when present
	if c.DatabaseConfig != nil {
		if err := c.DatabaseConfig.Validate(); err != nil {
			return fmt.Errorf("database config: %w", err)
		}
	}

	// Validate host URL for cookie sync
	if c.HostURL == "" {
		return fmt.Errorf("host URL is required")
	}

	// Validate default currency
	if c.DefaultCurrency == "" {
		return fmt.Errorf("default currency is required")
	}

	// SECURITY: Validate CORS origins in production
	if isProduction() {
		if len(c.CORSOrigins) == 0 {
			return fmt.Errorf("CORS origins must be explicitly configured in production (set CORS_ORIGINS)")
		}

		// Check for wildcard in production
		for _, origin := range c.CORSOrigins {
			if origin == "*" {
				return fmt.Errorf("CORS wildcard '*' is not allowed in production - specify explicit origins")
			}
		}
	}

	return nil
}

// Validate validates the database configuration
func (dc *DatabaseConfig) Validate() error {
	if dc.Host == "" {
		return fmt.Errorf("host is required")
	}

	if dc.Port == "" {
		return fmt.Errorf("port is required")
	}

	port, err := strconv.Atoi(dc.Port)
	if err != nil {
		return fmt.Errorf("port must be numeric: %w", err)
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be in range 1-65535, got %d", port)
	}

	if dc.User == "" {
		return fmt.Errorf("user is required")
	}

	if dc.Password == "" {
		return fmt.Errorf("password is required")
	}

	// SECURITY: Validate password strength and reject placeholder passwords
	if err := validatePassword(dc.Password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	if dc.Name == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate SSL mode
	validSSLModes := map[string]bool{
		"disable":     true,
		"require":     true,
		"verify-ca":   true,
		"verify-full": true,
	}

	if !validSSLModes[dc.SSLMode] {
		return fmt.Errorf("invalid SSL mode: %s (must be one of: disable, require, verify-ca, verify-full)", dc.SSLMode)
	}

	// SECURITY: In production, SSL must not be disabled
	if isProduction() && dc.SSLMode == "disable" {
		return fmt.Errorf("SSL mode 'disable' is not allowed in production (set ENVIRONMENT=production or ENV=production)")
	}

	// SECURITY: Validate connection pool bounds
	if dc.MaxConnections < 1 {
		return fmt.Errorf("max connections must be at least 1, got %d", dc.MaxConnections)
	}

	if dc.MaxConnections > 1000 {
		return fmt.Errorf("max connections must not exceed 1000, got %d", dc.MaxConnections)
	}

	if dc.MaxIdleConns < 0 {
		return fmt.Errorf("max idle connections must be non-negative, got %d", dc.MaxIdleConns)
	}

	if dc.MaxIdleConns > dc.MaxConnections {
		return fmt.Errorf("max idle connections (%d) cannot exceed max connections (%d)", dc.MaxIdleConns, dc.MaxConnections)
	}

	if dc.ConnMaxLifetime < 0 {
		return fmt.Errorf("connection max lifetime must be non-negative, got %v", dc.ConnMaxLifetime)
	}

	return nil
}

// validatePassword validates password strength and rejects common placeholders
func validatePassword(password string) error {
	// Check minimum length
	if len(password) < 16 {
		return fmt.Errorf("password must be at least 16 characters long, got %d", len(password))
	}

	// Convert to lowercase for case-insensitive checks
	passwordLower := toLower(password)

	// Check for placeholder passwords (case-insensitive)
	placeholders := []string{
		"changeme",
		"change_me",
		"change-me",
		"password",
		"secret",
		"admin",
		"root",
		"test",
		"demo",
		"example",
		"default",
		"placeholder",
	}

	for _, placeholder := range placeholders {
		if containsString(passwordLower, placeholder) {
			return fmt.Errorf("password contains placeholder text '%s' - use a strong, unique password", placeholder)
		}
	}

	return nil
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + ('a' - 'A')
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// containsString checks if s contains substr (case-sensitive)
func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
