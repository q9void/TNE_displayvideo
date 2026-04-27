package main

import (
	"flag"
	"os"
	"testing"
	"time"
)

func TestParseConfig_Defaults(t *testing.T) {
	// Clear all environment variables
	clearEnvVars(t)

	// Reset flags before each test
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	if cfg.Port != "8000" {
		t.Errorf("Expected default port '8000', got '%s'", cfg.Port)
	}

	if cfg.Timeout != 2500*time.Millisecond {
		t.Errorf("Expected default timeout 2500ms, got %v", cfg.Timeout)
	}

	if cfg.IDRUrl != "http://localhost:5050" {
		t.Errorf("Expected default IDR URL 'http://localhost:5050', got '%s'", cfg.IDRUrl)
	}

	if !cfg.IDREnabled {
		t.Error("Expected IDR to be enabled by default")
	}

	if cfg.CurrencyConversionEnabled != true {
		t.Error("Expected currency conversion to be enabled by default")
	}

	if cfg.DefaultCurrency != "USD" {
		t.Errorf("Expected default currency 'USD', got '%s'", cfg.DefaultCurrency)
	}

	if cfg.HostURL != "https://ads.thenexusengine.com" {
		t.Errorf("Expected default host URL 'https://ads.thenexusengine.com', got '%s'", cfg.HostURL)
	}

	if cfg.DatabaseConfig != nil {
		t.Error("Expected no database config when DB_HOST is not set")
	}

	if cfg.RedisURL != "" {
		t.Error("Expected empty Redis URL when REDIS_URL is not set")
	}
}

func TestParseConfig_EnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		validate func(*testing.T, *ServerConfig)
	}{
		{
			name: "Custom port",
			envVars: map[string]string{
				"PBS_PORT": "9000",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.Port != "9000" {
					t.Errorf("Expected port '9000', got '%s'", cfg.Port)
				}
			},
		},
		{
			name: "Custom IDR URL",
			envVars: map[string]string{
				"IDR_URL": "http://idr.example.com:8080",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.IDRUrl != "http://idr.example.com:8080" {
					t.Errorf("Expected IDR URL 'http://idr.example.com:8080', got '%s'", cfg.IDRUrl)
				}
			},
		},
		{
			name: "IDR disabled",
			envVars: map[string]string{
				"IDR_ENABLED": "false",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.IDREnabled {
					t.Error("Expected IDR to be disabled")
				}
			},
		},
		{
			name: "IDR API key",
			envVars: map[string]string{
				"IDR_API_KEY": "secret-key-123",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.IDRAPIKey != "secret-key-123" {
					t.Errorf("Expected IDR API key 'secret-key-123', got '%s'", cfg.IDRAPIKey)
				}
			},
		},
		{
			name: "Redis URL",
			envVars: map[string]string{
				"REDIS_URL": "redis://localhost:6379/0",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.RedisURL != "redis://localhost:6379/0" {
					t.Errorf("Expected Redis URL 'redis://localhost:6379/0', got '%s'", cfg.RedisURL)
				}
			},
		},
		{
			name: "Currency conversion disabled",
			envVars: map[string]string{
				"CURRENCY_CONVERSION_ENABLED": "false",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.CurrencyConversionEnabled {
					t.Error("Expected currency conversion to be disabled")
				}
			},
		},
		{
			name: "GDPR enforcement disabled",
			envVars: map[string]string{
				"PBS_DISABLE_GDPR_ENFORCEMENT": "true",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if !cfg.DisableGDPREnforcement {
					t.Error("Expected GDPR enforcement to be disabled")
				}
			},
		},
		{
			name: "Custom host URL",
			envVars: map[string]string{
				"PBS_HOST_URL": "https://custom.example.com",
			},
			validate: func(t *testing.T, cfg *ServerConfig) {
				if cfg.HostURL != "https://custom.example.com" {
					t.Errorf("Expected host URL 'https://custom.example.com', got '%s'", cfg.HostURL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set environment variables
			clearEnvVars(t)
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Reset flags
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			cfg := ParseConfig()
			tt.validate(t, cfg)
		})
	}
}

func TestParseConfig_DatabaseConfig(t *testing.T) {
	clearEnvVars(t)

	// Set database environment variables
	t.Setenv("DB_HOST", "postgres.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "testuser")
	t.Setenv("DB_PASSWORD", "testpass")
	t.Setenv("DB_NAME", "testdb")
	t.Setenv("DB_SSL_MODE", "require")

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	if cfg.DatabaseConfig == nil {
		t.Fatal("Expected database config to be set")
	}

	dbCfg := cfg.DatabaseConfig

	if dbCfg.Host != "postgres.example.com" {
		t.Errorf("Expected DB host 'postgres.example.com', got '%s'", dbCfg.Host)
	}

	if dbCfg.Port != "5433" {
		t.Errorf("Expected DB port '5433', got '%s'", dbCfg.Port)
	}

	if dbCfg.User != "testuser" {
		t.Errorf("Expected DB user 'testuser', got '%s'", dbCfg.User)
	}

	if dbCfg.Password != "testpass" {
		t.Errorf("Expected DB password 'testpass', got '%s'", dbCfg.Password)
	}

	if dbCfg.Name != "testdb" {
		t.Errorf("Expected DB name 'testdb', got '%s'", dbCfg.Name)
	}

	if dbCfg.SSLMode != "require" {
		t.Errorf("Expected DB SSL mode 'require', got '%s'", dbCfg.SSLMode)
	}
}

func TestParseConfig_DatabaseConfig_NotSet(t *testing.T) {
	clearEnvVars(t)

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	if cfg.DatabaseConfig != nil {
		t.Error("Expected no database config when DB_HOST is not set")
	}
}

func TestParseConfig_DatabaseConfig_Defaults(t *testing.T) {
	clearEnvVars(t)

	// Set only DB_HOST, use defaults for the rest
	t.Setenv("DB_HOST", "localhost")

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	if cfg.DatabaseConfig == nil {
		t.Fatal("Expected database config to be set")
	}

	dbCfg := cfg.DatabaseConfig

	if dbCfg.Host != "localhost" {
		t.Errorf("Expected DB host 'localhost', got '%s'", dbCfg.Host)
	}

	if dbCfg.Port != "5432" {
		t.Errorf("Expected default DB port '5432', got '%s'", dbCfg.Port)
	}

	if dbCfg.User != "catalyst" {
		t.Errorf("Expected default DB user 'catalyst', got '%s'", dbCfg.User)
	}

	if dbCfg.Password != "" {
		t.Errorf("Expected default DB password '', got '%s'", dbCfg.Password)
	}

	if dbCfg.Name != "catalyst" {
		t.Errorf("Expected default DB name 'catalyst', got '%s'", dbCfg.Name)
	}

	if dbCfg.SSLMode != "disable" {
		t.Errorf("Expected default DB SSL mode 'disable', got '%s'", dbCfg.SSLMode)
	}
}

func TestToExchangeConfig(t *testing.T) {
	// EventRecordEnabled is wired from the IDR_EVENT_RECORD_ENABLED env var
	// (see ToExchangeConfig in config.go). Set it for the duration of the test.
	t.Setenv("IDR_EVENT_RECORD_ENABLED", "true")

	cfg := &ServerConfig{
		Port:                      "8000",
		Timeout:                   2000 * time.Millisecond,
		IDREnabled:                true,
		IDRUrl:                    "http://idr.example.com",
		IDRAPIKey:                 "test-api-key",
		CurrencyConversionEnabled: true,
		DefaultCurrency:           "EUR",
	}

	exCfg := cfg.ToExchangeConfig()

	if exCfg.DefaultTimeout != 2000*time.Millisecond {
		t.Errorf("Expected timeout 2000ms, got %v", exCfg.DefaultTimeout)
	}

	if exCfg.MaxBidders != 50 {
		t.Errorf("Expected max bidders 50, got %d", exCfg.MaxBidders)
	}

	if !exCfg.IDREnabled {
		t.Error("Expected IDR to be enabled")
	}

	if exCfg.IDRServiceURL != "http://idr.example.com" {
		t.Errorf("Expected IDR URL 'http://idr.example.com', got '%s'", exCfg.IDRServiceURL)
	}

	if exCfg.IDRAPIKey != "test-api-key" {
		t.Errorf("Expected IDR API key 'test-api-key', got '%s'", exCfg.IDRAPIKey)
	}

	if !exCfg.EventRecordEnabled {
		t.Error("Expected event recording to be enabled")
	}

	if exCfg.EventBufferSize != 100 {
		t.Errorf("Expected event buffer size 100, got %d", exCfg.EventBufferSize)
	}

	if !exCfg.CurrencyConv {
		t.Error("Expected currency conversion to be enabled")
	}

	if exCfg.DefaultCurrency != "EUR" {
		t.Errorf("Expected default currency 'EUR', got '%s'", exCfg.DefaultCurrency)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		setValue     bool
		defaultValue string
		expected     string
	}{
		{
			name:         "With value",
			key:          "TEST_VAR",
			value:        "test_value",
			setValue:     true,
			defaultValue: "default",
			expected:     "test_value",
		},
		{
			name:         "Without value",
			key:          "MISSING_VAR",
			setValue:     false,
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "Empty string",
			key:          "EMPTY_VAR",
			value:        "",
			setValue:     true,
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue {
				t.Setenv(tt.key, tt.value)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetEnvBoolOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		setValue     bool
		defaultValue bool
		expected     bool
	}{
		{
			name:         "true",
			value:        "true",
			setValue:     true,
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "1",
			value:        "1",
			setValue:     true,
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "yes",
			value:        "yes",
			setValue:     true,
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "false",
			value:        "false",
			setValue:     true,
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "0",
			value:        "0",
			setValue:     true,
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "no",
			value:        "no",
			setValue:     true,
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "Empty uses default false",
			value:        "",
			setValue:     false,
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "Empty uses default true",
			value:        "",
			setValue:     false,
			defaultValue: true,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_VAR"
			if tt.setValue {
				t.Setenv(key, tt.value)
			} else {
				os.Unsetenv(key)
			}

			result := getEnvBoolOrDefault(key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function to clear relevant environment variables
func clearEnvVars(t *testing.T) {
	t.Helper()

	envVars := []string{
		"PBS_PORT",
		"IDR_URL",
		"IDR_ENABLED",
		"IDR_API_KEY",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSL_MODE",
		"REDIS_URL",
		"CURRENCY_CONVERSION_ENABLED",
		"PBS_DISABLE_GDPR_ENFORCEMENT",
		"PBS_HOST_URL",
	}

	for _, key := range envVars {
		os.Unsetenv(key)
	}
}

func TestParseConfig_AllEnvironmentVariables(t *testing.T) {
	clearEnvVars(t)

	// Set all possible environment variables
	t.Setenv("PBS_PORT", "9090")
	t.Setenv("IDR_URL", "http://idr.custom.com:9000")
	t.Setenv("IDR_ENABLED", "false")
	t.Setenv("IDR_API_KEY", "super-secret-key")
	t.Setenv("REDIS_URL", "redis://redis.example.com:6380/1")
	t.Setenv("CURRENCY_CONVERSION_ENABLED", "false")
	t.Setenv("PBS_DISABLE_GDPR_ENFORCEMENT", "true")
	t.Setenv("PBS_HOST_URL", "https://custom-host.com")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "customuser")
	t.Setenv("DB_PASSWORD", "custompass")
	t.Setenv("DB_NAME", "customdb")
	t.Setenv("DB_SSL_MODE", "require")

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	// Verify all values
	if cfg.Port != "9090" {
		t.Errorf("Expected port '9090', got '%s'", cfg.Port)
	}

	if cfg.IDRUrl != "http://idr.custom.com:9000" {
		t.Errorf("Expected IDR URL, got '%s'", cfg.IDRUrl)
	}

	if cfg.IDREnabled {
		t.Error("Expected IDR to be disabled")
	}

	if cfg.IDRAPIKey != "super-secret-key" {
		t.Errorf("Expected IDR API key, got '%s'", cfg.IDRAPIKey)
	}

	if cfg.RedisURL != "redis://redis.example.com:6380/1" {
		t.Errorf("Expected Redis URL, got '%s'", cfg.RedisURL)
	}

	if cfg.CurrencyConversionEnabled {
		t.Error("Expected currency conversion to be disabled")
	}

	if !cfg.DisableGDPREnforcement {
		t.Error("Expected GDPR enforcement to be disabled")
	}

	if cfg.HostURL != "https://custom-host.com" {
		t.Errorf("Expected host URL, got '%s'", cfg.HostURL)
	}

	if cfg.DatabaseConfig == nil {
		t.Fatal("Expected database config to be set")
	}

	dbCfg := cfg.DatabaseConfig
	if dbCfg.Host != "db.example.com" {
		t.Errorf("Expected DB host, got '%s'", dbCfg.Host)
	}

	if dbCfg.Port != "5433" {
		t.Errorf("Expected DB port, got '%s'", dbCfg.Port)
	}

	if dbCfg.User != "customuser" {
		t.Errorf("Expected DB user, got '%s'", dbCfg.User)
	}

	if dbCfg.Password != "custompass" {
		t.Errorf("Expected DB password, got '%s'", dbCfg.Password)
	}

	if dbCfg.Name != "customdb" {
		t.Errorf("Expected DB name, got '%s'", dbCfg.Name)
	}

	if dbCfg.SSLMode != "require" {
		t.Errorf("Expected DB SSL mode, got '%s'", dbCfg.SSLMode)
	}
}

func TestParseConfig_MixedFlagsAndEnv(t *testing.T) {
	clearEnvVars(t)

	// Set some env vars
	t.Setenv("PBS_PORT", "7777")
	t.Setenv("IDR_API_KEY", "env-key")

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	// Port should come from env
	if cfg.Port != "7777" {
		t.Errorf("Expected port from env, got '%s'", cfg.Port)
	}

	// API key should come from env
	if cfg.IDRAPIKey != "env-key" {
		t.Errorf("Expected API key from env, got '%s'", cfg.IDRAPIKey)
	}
}

func TestGetEnvBoolOrDefault_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		setValue bool
		defVal   bool
		expected bool
	}{
		{
			name:     "TRUE uppercase",
			value:    "TRUE",
			setValue: true,
			defVal:   false,
			expected: false, // Only lowercase "true" is accepted
		},
		{
			name:     "Yes uppercase",
			value:    "YES",
			setValue: true,
			defVal:   false,
			expected: false, // Only lowercase "yes" is accepted
		},
		{
			name:     "Random string",
			value:    "random",
			setValue: true,
			defVal:   true,
			expected: false,
		},
		{
			name:     "Number 2",
			value:    "2",
			setValue: true,
			defVal:   false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_EDGE"
			if tt.setValue {
				t.Setenv(key, tt.value)
			} else {
				os.Unsetenv(key)
			}

			result := getEnvBoolOrDefault(key, tt.defVal)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v for value '%s'", tt.expected, result, tt.value)
			}
		})
	}
}

func TestServerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				IDREnabled:      false,
			},
			wantErr: false,
		},
		{
			name: "valid config with IDR enabled",
			config: &ServerConfig{
				Port:            "8080",
				Timeout:         2 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				IDREnabled:      true,
				IDRUrl:          "http://localhost:5050",
				IDRAPIKey:       "test-key",
			},
			wantErr: false,
		},
		{
			name: "valid config with database",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				DatabaseConfig: &DatabaseConfig{
					Host:            "localhost",
					Port:            "5432",
					User:            "userxyz9876543",
					Password:        "S3cur3P@ssw0rd!9876XYZ",
					Name:            "testdb",
					SSLMode:         "disable",
					MaxConnections:  100,
					MaxIdleConns:    10,
					ConnMaxLifetime: 3600 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "empty port",
			config: &ServerConfig{
				Port:            "",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "port is required",
		},
		{
			name: "non-numeric port",
			config: &ServerConfig{
				Port:            "abc",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "port must be numeric",
		},
		{
			name: "port below valid range",
			config: &ServerConfig{
				Port:            "0",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "port must be in range 1-65535",
		},
		{
			name: "port above valid range",
			config: &ServerConfig{
				Port:            "65536",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "port must be in range 1-65535",
		},
		{
			name: "negative timeout",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         -1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "zero timeout",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         0,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "timeout too large",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         31 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "timeout must be less than 30s",
		},
		{
			name: "IDR enabled without URL",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				IDREnabled:      true,
				IDRAPIKey:       "test-key",
			},
			wantErr: true,
			errMsg:  "IDR URL is required when IDR is enabled",
		},
		{
			name: "IDR enabled without API key",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				IDREnabled:      true,
				IDRUrl:          "http://localhost:5050",
			},
			wantErr: true,
			errMsg:  "IDR API key is required when IDR is enabled",
		},
		{
			name: "empty host URL",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "",
				DefaultCurrency: "USD",
			},
			wantErr: true,
			errMsg:  "host URL is required",
		},
		{
			name: "empty default currency",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "",
			},
			wantErr: true,
			errMsg:  "default currency is required",
		},
		{
			name: "invalid database config",
			config: &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				DatabaseConfig: &DatabaseConfig{
					Host:     "",
					Port:     "5432",
					User:     "userxyz9876543",
					Password: "S3cur3P@ssw0rd!9876XYZ",
					Name:     "testdb",
					SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,				},
			},
			wantErr: true,
			errMsg:  "host is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestDatabaseConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *DatabaseConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: false,
		},
		{
			name: "valid config with SSL require",
			config: &DatabaseConfig{
				Host:     "db.example.com",
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "require",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: false,
		},
		{
			name: "valid config with SSL verify-ca",
			config: &DatabaseConfig{
				Host:     "db.example.com",
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "verify-ca",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: false,
		},
		{
			name: "valid config with SSL verify-full",
			config: &DatabaseConfig{
				Host:     "db.example.com",
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "verify-full",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: &DatabaseConfig{
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "host is required",
		},
		{
			name: "missing port",
			config: &DatabaseConfig{
				Host:     "localhost",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "port is required",
		},
		{
			name: "non-numeric port",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "abc",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "port must be numeric",
		},
		{
			name: "port out of range - too low",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "0",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "port must be in range 1-65535",
		},
		{
			name: "port out of range - too high",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "70000",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "port must be in range 1-65535",
		},
		{
			name: "missing user",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "user is required",
		},
		{
			name: "missing password",
			config: &DatabaseConfig{
				Host:    "localhost",
				Port:    "5432",
				User:    "testuser",
				Name:    "testdb",
				SSLMode: "disable",
			},
			wantErr: true,
			errMsg:  "password is required",
		},
		{
			name: "missing database name",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				SSLMode:  "disable",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "database name is required",
		},
		{
			name: "invalid SSL mode",
			config: &DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "userxyz9876543",
				Password: "S3cur3P@ssw0rd!9876XYZ",
				Name:     "testdb",
				SSLMode:  "invalid",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,			},
			wantErr: true,
			errMsg:  "invalid SSL mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

// containsHelper is a helper function for contains
func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// SECURITY VALIDATION TESTS

func TestDatabaseConfigValidate_PasswordSecurity(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "placeholder password - changeme",
			password: "changeme",
			wantErr:  true,
			errMsg:   "password must be at least 16 characters long",
		},
		{
			name:     "placeholder password - changeme 16+ chars",
			password: "changeme12345678",
			wantErr:  true,
			errMsg:   "password contains placeholder text 'changeme'",
		},
		{
			name:     "placeholder password - CHANGEME uppercase",
			password: "MyPassword_CHANGEME_123",
			wantErr:  true,
			errMsg:   "password contains placeholder text 'changeme'",
		},
		{
			name:     "placeholder password - change_me",
			password: "change_me",
			wantErr:  true,
			errMsg:   "password must be at least 16 characters long",
		},
		{
			name:     "placeholder password - password",
			password: "password123456789",
			wantErr:  true,
			errMsg:   "password contains placeholder text 'password'",
		},
		{
			name:     "too short - 15 chars",
			password: "SecurePass12345",
			wantErr:  true,
			errMsg:   "password must be at least 16 characters long",
		},
		{
			name:     "too short - 10 chars",
			password: "Short12345",
			wantErr:  true,
			errMsg:   "password must be at least 16 characters long",
		},
		{
			name:     "placeholder - admin",
			password: "admin1234567890123",
			wantErr:  true,
			errMsg:   "password contains placeholder text 'admin'",
		},
		{
			name:     "placeholder - test",
			password: "test12345678901234",
			wantErr:  true,
			errMsg:   "password contains placeholder text 'test'",
		},
		{
			name:     "placeholder - demo",
			password: "demo12345678901234",
			wantErr:  true,
			errMsg:   "password contains placeholder text 'demo'",
		},
		{
			name:     "valid strong password - 16 chars",
			password: "xK9$mP2#vL5@nQ8!",
			wantErr:  false,
		},
		{
			name:     "valid strong password - 20 chars",
			password: "xK9$mP2#vL5@nQ8!wR7%",
			wantErr:  false,
		},
		{
			name:     "valid strong password - 32 chars",
			password: "xK9$mP2#vL5@nQ8!wR7%tY4&uI3*",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &DatabaseConfig{
				Host:            "localhost",
				Port:            "5432",
				User:            "testuser",
				Password:        tt.password,
				Name:            "testdb",
				SSLMode:         "require",
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,
			}

			err := config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestDatabaseConfigValidate_SSLModeProduction(t *testing.T) {
	tests := []struct {
		name        string
		sslMode     string
		setEnv      bool
		envValue    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "production with SSL disable - should fail",
			sslMode:     "disable",
			setEnv:      true,
			envValue:    "production",
			wantErr:     true,
			errContains: "SSL mode 'disable' is not allowed in production",
		},
		{
			name:        "production with SSL require - should pass",
			sslMode:     "require",
			setEnv:      true,
			envValue:    "production",
			wantErr:     false,
		},
		{
			name:        "prod (short) with SSL disable - should fail",
			sslMode:     "disable",
			setEnv:      true,
			envValue:    "prod",
			wantErr:     true,
			errContains: "SSL mode 'disable' is not allowed in production",
		},
		{
			name:     "non-production with SSL disable - should pass",
			sslMode:  "disable",
			setEnv:   true,
			envValue: "development",
			wantErr:  false,
		},
		{
			name:     "no environment set with SSL disable - should pass",
			sslMode:  "disable",
			setEnv:   false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment if needed
			if tt.setEnv {
				t.Setenv("ENVIRONMENT", tt.envValue)
			} else {
				os.Unsetenv("ENVIRONMENT")
				os.Unsetenv("ENV")
			}

			config := &DatabaseConfig{
				Host:            "localhost",
				Port:            "5432",
				User:            "userxyz9876543",
				Password:        "S3cur3P@ssw0rd!9876XYZ",
				Name:            "testdb",
				SSLMode:         tt.sslMode,
				MaxConnections:  100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600 * time.Second,
			}

			err := config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestDatabaseConfigValidate_ConnectionPoolBounds(t *testing.T) {
	tests := []struct {
		name           string
		maxConnections int
		maxIdleConns   int
		wantErr        bool
		errContains    string
	}{
		{
			name:           "max connections zero - should fail",
			maxConnections: 0,
			maxIdleConns:   0,
			wantErr:        true,
			errContains:    "max connections must be at least 1",
		},
		{
			name:           "max connections negative - should fail",
			maxConnections: -1,
			maxIdleConns:   0,
			wantErr:        true,
			errContains:    "max connections must be at least 1",
		},
		{
			name:           "max connections exceeds 1000 - should fail",
			maxConnections: 1001,
			maxIdleConns:   10,
			wantErr:        true,
			errContains:    "max connections must not exceed 1000",
		},
		{
			name:           "max connections 1000 - should pass",
			maxConnections: 1000,
			maxIdleConns:   100,
			wantErr:        false,
		},
		{
			name:           "max idle negative - should fail",
			maxConnections: 100,
			maxIdleConns:   -1,
			wantErr:        true,
			errContains:    "max idle connections must be non-negative",
		},
		{
			name:           "max idle exceeds max connections - should fail",
			maxConnections: 100,
			maxIdleConns:   101,
			wantErr:        true,
			errContains:    "max idle connections (101) cannot exceed max connections (100)",
		},
		{
			name:           "valid pool configuration",
			maxConnections: 100,
			maxIdleConns:   10,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &DatabaseConfig{
				Host:            "localhost",
				Port:            "5432",
				User:            "userxyz9876543",
				Password:        "S3cur3P@ssw0rd!9876XYZ",
				Name:            "testdb",
				SSLMode:         "require",
				MaxConnections:  tt.maxConnections,
				MaxIdleConns:    tt.maxIdleConns,
				ConnMaxLifetime: 3600 * time.Second,
			}

			err := config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestServerConfigValidate_CORSProduction(t *testing.T) {
	tests := []struct {
		name        string
		corsOrigins []string
		setEnv      bool
		envValue    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "production with wildcard CORS - should fail",
			corsOrigins: []string{"*"},
			setEnv:      true,
			envValue:    "production",
			wantErr:     true,
			errContains: "CORS wildcard '*' is not allowed in production",
		},
		{
			name:        "production with empty CORS - should fail",
			corsOrigins: []string{},
			setEnv:      true,
			envValue:    "production",
			wantErr:     true,
			errContains: "CORS origins must be explicitly configured in production",
		},
		{
			name:        "production with explicit origins - should pass",
			corsOrigins: []string{"https://example.com", "https://app.example.com"},
			setEnv:      true,
			envValue:    "production",
			wantErr:     false,
		},
		{
			name:        "non-production with wildcard - should pass",
			corsOrigins: []string{"*"},
			setEnv:      true,
			envValue:    "development",
			wantErr:     false,
		},
		{
			name:        "non-production with empty CORS - should pass",
			corsOrigins: []string{},
			setEnv:      true,
			envValue:    "development",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment if needed
			if tt.setEnv {
				t.Setenv("ENVIRONMENT", tt.envValue)
			} else {
				os.Unsetenv("ENVIRONMENT")
				os.Unsetenv("ENV")
			}

			config := &ServerConfig{
				Port:            "8000",
				Timeout:         1 * time.Second,
				HostURL:         "https://example.com",
				DefaultCurrency: "USD",
				CORSOrigins:     tt.corsOrigins,
			}

			err := config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestGetEnvIntOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		setValue     bool
		defaultValue int
		expected     int
	}{
		{
			name:         "with valid integer",
			key:          "TEST_INT",
			value:        "42",
			setValue:     true,
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "without value",
			key:          "MISSING_INT",
			setValue:     false,
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "with invalid integer",
			key:          "INVALID_INT",
			value:        "not-a-number",
			setValue:     true,
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "with negative integer",
			key:          "NEGATIVE_INT",
			value:        "-5",
			setValue:     true,
			defaultValue: 10,
			expected:     -5,
		},
		{
			name:         "with zero",
			key:          "ZERO_INT",
			value:        "0",
			setValue:     true,
			defaultValue: 10,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setValue {
				t.Setenv(tt.key, tt.value)
			} else {
				os.Unsetenv(tt.key)
			}

			result := getEnvIntOrDefault(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name       string
		envVar     string
		envValue   string
		setEnv     bool
		expected   bool
	}{
		{
			name:     "ENVIRONMENT=production",
			envVar:   "ENVIRONMENT",
			envValue: "production",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "ENVIRONMENT=prod",
			envVar:   "ENVIRONMENT",
			envValue: "prod",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "ENV=production",
			envVar:   "ENV",
			envValue: "production",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "ENV=prod",
			envVar:   "ENV",
			envValue: "prod",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "ENVIRONMENT=development",
			envVar:   "ENVIRONMENT",
			envValue: "development",
			setEnv:   true,
			expected: false,
		},
		{
			name:     "ENVIRONMENT=staging",
			envVar:   "ENVIRONMENT",
			envValue: "staging",
			setEnv:   true,
			expected: false,
		},
		{
			name:     "no environment set",
			setEnv:   false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both env vars first
			os.Unsetenv("ENVIRONMENT")
			os.Unsetenv("ENV")

			if tt.setEnv {
				t.Setenv(tt.envVar, tt.envValue)
			}

			result := isProduction()

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseConfig_WithConnectionPoolSettings(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PASSWORD", "SecurePassword123!@#$")
	t.Setenv("DB_MAX_CONNECTIONS", "200")
	t.Setenv("DB_MAX_IDLE_CONNS", "20")
	t.Setenv("DB_CONN_MAX_LIFETIME_SECONDS", "7200")

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	if cfg.DatabaseConfig == nil {
		t.Fatal("Expected database config to be set")
	}

	if cfg.DatabaseConfig.MaxConnections != 200 {
		t.Errorf("Expected max connections 200, got %d", cfg.DatabaseConfig.MaxConnections)
	}

	if cfg.DatabaseConfig.MaxIdleConns != 20 {
		t.Errorf("Expected max idle conns 20, got %d", cfg.DatabaseConfig.MaxIdleConns)
	}

	if cfg.DatabaseConfig.ConnMaxLifetime != 7200*time.Second {
		t.Errorf("Expected conn max lifetime 7200s, got %v", cfg.DatabaseConfig.ConnMaxLifetime)
	}
}

func TestParseConfig_WithCORSOrigins(t *testing.T) {
	clearEnvVars(t)

	t.Setenv("CORS_ORIGINS", "https://example.com,https://app.example.com,https://admin.example.com")

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cfg := ParseConfig()

	expectedOrigins := []string{
		"https://example.com",
		"https://app.example.com",
		"https://admin.example.com",
	}

	if len(cfg.CORSOrigins) != len(expectedOrigins) {
		t.Errorf("Expected %d CORS origins, got %d", len(expectedOrigins), len(cfg.CORSOrigins))
	}

	for i, expected := range expectedOrigins {
		if i >= len(cfg.CORSOrigins) {
			t.Errorf("Missing CORS origin at index %d: expected %s", i, expected)
			continue
		}
		if cfg.CORSOrigins[i] != expected {
			t.Errorf("CORS origin %d: expected %s, got %s", i, expected, cfg.CORSOrigins[i])
		}
	}
}
