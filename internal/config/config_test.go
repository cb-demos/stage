package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		envVars       map[string]string
		expectError   bool
		expectedPort  string
		expectedHost  string
		expectedDir   string
		expectedReplacements map[string]string
	}{
		{
			name: "default values",
			envVars: map[string]string{
				"ASSET_DIR": tempDir,
			},
			expectError:  false,
			expectedPort: "8080",
			expectedHost: "0.0.0.0",
			expectedDir:  tempDir,
			expectedReplacements: map[string]string{},
		},
		{
			name: "custom port and host",
			envVars: map[string]string{
				"PORT":      "3000",
				"HOST":      "127.0.0.1",
				"ASSET_DIR": tempDir,
			},
			expectError:  false,
			expectedPort: "3000",
			expectedHost: "127.0.0.1",
			expectedDir:  tempDir,
			expectedReplacements: map[string]string{},
		},
		{
			name: "single STAGE_ variable",
			envVars: map[string]string{
				"ASSET_DIR":       tempDir,
				"STAGE_FF_SDK_KEY": "test-key-123",
			},
			expectError:  false,
			expectedPort: "8080",
			expectedHost: "0.0.0.0",
			expectedDir:  tempDir,
			expectedReplacements: map[string]string{
				"FF_SDK_KEY": "test-key-123",
			},
		},
		{
			name: "multiple STAGE_ variables",
			envVars: map[string]string{
				"ASSET_DIR":           tempDir,
				"STAGE_FF_SDK_KEY":    "test-key-123",
				"STAGE_API_ENDPOINT":  "https://api.test.com",
				"STAGE_APP_NAME":      "Test App",
			},
			expectError:  false,
			expectedPort: "8080",
			expectedHost: "0.0.0.0",
			expectedDir:  tempDir,
			expectedReplacements: map[string]string{
				"FF_SDK_KEY":   "test-key-123",
				"API_ENDPOINT": "https://api.test.com",
				"APP_NAME":     "Test App",
			},
		},
		{
			name: "non-STAGE variables ignored",
			envVars: map[string]string{
				"ASSET_DIR":         tempDir,
				"STAGE_FF_SDK_KEY":  "test-key-123",
				"REGULAR_VAR":       "should-be-ignored",
				"PATH":              "/usr/bin",
			},
			expectError:  false,
			expectedPort: "8080",
			expectedHost: "0.0.0.0",
			expectedDir:  tempDir,
			expectedReplacements: map[string]string{
				"FF_SDK_KEY": "test-key-123",
			},
		},
		{
			name: "missing asset directory",
			envVars: map[string]string{
				"ASSET_DIR": "/nonexistent/directory",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Ensure cleanup
			defer clearEnv()

			// Load configuration
			cfg, err := Load()

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Validate configuration values
			if cfg.Port != tt.expectedPort {
				t.Errorf("expected port %s, got %s", tt.expectedPort, cfg.Port)
			}

			if cfg.Host != tt.expectedHost {
				t.Errorf("expected host %s, got %s", tt.expectedHost, cfg.Host)
			}

			if cfg.AssetDir != tt.expectedDir {
				t.Errorf("expected asset dir %s, got %s", tt.expectedDir, cfg.AssetDir)
			}

			// Validate replacements
			if len(cfg.Replacements) != len(tt.expectedReplacements) {
				t.Errorf("expected %d replacements, got %d", len(tt.expectedReplacements), len(cfg.Replacements))
			}

			for key, expectedValue := range tt.expectedReplacements {
				actualValue, exists := cfg.Replacements[key]
				if !exists {
					t.Errorf("expected replacement key %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("for key %s, expected value %s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Port:         "8080",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: false,
		},
		{
			name: "valid config with port 1",
			config: &Config{
				Port:         "1",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: false,
		},
		{
			name: "valid config with port 65535",
			config: &Config{
				Port:         "65535",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: false,
		},
		{
			name: "empty port",
			config: &Config{
				Port:         "",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "invalid port - zero",
			config: &Config{
				Port:         "0",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "invalid port - negative",
			config: &Config{
				Port:         "-1",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "invalid port - too large",
			config: &Config{
				Port:         "65536",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "invalid port - way too large",
			config: &Config{
				Port:         "99999",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "invalid port - not a number",
			config: &Config{
				Port:         "abc",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "invalid port - alphanumeric",
			config: &Config{
				Port:         "8080abc",
				AssetDir:     tempDir,
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "empty asset dir",
			config: &Config{
				Port:         "8080",
				AssetDir:     "",
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
		{
			name: "nonexistent asset dir",
			config: &Config{
				Port:         "8080",
				AssetDir:     "/nonexistent/path",
				Host:         "0.0.0.0",
				Replacements: map[string]string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		setEnv       bool
		expected     string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			setEnv:       true,
			expected:     "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR",
			defaultValue: "default",
			setEnv:       false,
			expected:     "default",
		},
		{
			name:         "env var set to empty string",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			setEnv:       true,
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv(tt.key)

			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
			}

			defer os.Unsetenv(tt.key)

			result := getEnvOrDefault(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// clearEnv removes all test-related environment variables
func clearEnv() {
	testVars := []string{
		"PORT", "HOST", "ASSET_DIR",
		"STAGE_FF_SDK_KEY", "STAGE_API_ENDPOINT", "STAGE_APP_NAME",
		"REGULAR_VAR",
	}
	for _, v := range testVars {
		os.Unsetenv(v)
	}
}
