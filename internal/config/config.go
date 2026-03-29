package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	APIKey      string `yaml:"api_key"`
	DefaultTeam string `yaml:"default_team"`
}

// Load reads configuration from environment variables and config file.
// Priority: LAZYLINEAR_API_KEY env var > ~/.config/lazylinear/config.yaml
func Load() (*Config, error) {
	cfg := &Config{}

	// Try loading from config file first as the base.
	configPath := defaultConfigPath()
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", configPath, err)
		}
	}

	// Environment variable overrides the config file API key.
	if envKey := os.Getenv("LAZYLINEAR_API_KEY"); envKey != "" {
		cfg.APIKey = envKey
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf(
			"no API key found\n\n" +
				"Set your Linear API key using one of:\n" +
				"  1. Environment variable: export LAZYLINEAR_API_KEY=lin_api_...\n" +
				"  2. Config file: ~/.config/lazylinear/config.yaml\n\n" +
				"Config file format:\n" +
				"  api_key: lin_api_...\n" +
				"  default_team: <team-key>\n\n" +
				"Get your API key at: https://linear.app/settings/api",
		)
	}

	return cfg, nil
}

func defaultConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "lazylinear", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lazylinear", "config.yaml")
}
