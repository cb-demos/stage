package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration
type Config struct {
	// Server configuration
	Port      string
	AssetDir  string
	Host      string

	// Transformation rules: map of placeholder -> replacement value
	// e.g., "FF_SDK_KEY" -> "abc123" means replace "__FF_SDK_KEY__" with "abc123"
	Replacements map[string]string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:         getEnvOrDefault("PORT", "8080"),
		AssetDir:     getEnvOrDefault("ASSET_DIR", "/app/assets"),
		Host:         getEnvOrDefault("HOST", "0.0.0.0"),
		Replacements: make(map[string]string),
	}

	// Parse all STAGE_* environment variables for transformations
	for _, env := range os.Environ() {
		// Split into key=value
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Check if it starts with STAGE_ prefix
		if strings.HasPrefix(key, "STAGE_") {
			// Extract the placeholder name (everything after STAGE_)
			placeholder := strings.TrimPrefix(key, "STAGE_")

			// Validate placeholder name
			if placeholder == "" || strings.TrimSpace(placeholder) == "" {
				slog.Warn("Ignoring invalid STAGE_ variable with empty name", "key", key)
				continue
			}

			cfg.Replacements[placeholder] = value
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}

	// Validate port is a number in valid range
	portNum, err := strconv.Atoi(c.Port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("PORT must be a number between 1 and 65535, got: %s", c.Port)
	}

	if c.AssetDir == "" {
		return fmt.Errorf("ASSET_DIR cannot be empty")
	}

	// Check if asset directory exists
	if _, err := os.Stat(c.AssetDir); os.IsNotExist(err) {
		return fmt.Errorf("asset directory does not exist: %s", c.AssetDir)
	}

	return nil
}

// getEnvOrDefault retrieves an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
