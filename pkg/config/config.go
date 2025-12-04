package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig  `json:"server" yaml:"server"`
	Packages PackageConfig `json:"packages" yaml:"packages"`
	Logging  LoggingConfig `json:"logging" yaml:"logging"`
	Plugins  PluginConfig  `json:"plugins" yaml:"plugins"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	HTTPMode bool   `json:"http_mode" yaml:"http_mode"`
	Port     string `json:"port" yaml:"port"`
}

// PackageConfig holds package directory configuration
type PackageConfig struct {
	Directory   string `json:"directory" yaml:"directory"`
	ExcludeFile string `json:"exclude_file" yaml:"exclude_file"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `json:"level" yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

// PluginConfig holds plugin system configuration
type PluginConfig struct {
	PluginDir         string         `json:"plugin_dir" yaml:"plugin_dir"`
	EnabledPlugins    []string       `json:"enabled_plugins" yaml:"enabled_plugins"`
	CommunityRegistry string         `json:"community_registry" yaml:"community_registry"`
	AutoUpdate        bool           `json:"auto_update" yaml:"auto_update"`
	Security          PluginSecurity `json:"security" yaml:"security"`
}

// PluginSecurity defines security settings for plugins
type PluginSecurity struct {
	AllowNetworkAccess bool     `json:"allow_network_access" yaml:"allow_network_access"`
	AllowedDomains     []string `json:"allowed_domains" yaml:"allowed_domains"`
	SignatureRequired  bool     `json:"signature_required" yaml:"signature_required"`
	TrustedPublishers  []string `json:"trusted_publishers" yaml:"trusted_publishers"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			HTTPMode: false,
			Port:     "8086",
		},
		Packages: PackageConfig{
			Directory: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Plugins: DefaultPluginConfig(),
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (Config, error) {
	config := DefaultConfig()

	// Load from config file if specified
	if configPath != "" {
		if err := loadConfigFile(configPath, &config); err != nil {
			return config, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Load environment-specific overrides
	if err := loadEnvironmentConfig(&config); err != nil {
		return config, fmt.Errorf("failed to load environment config: %w", err)
	}

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		return config, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadConfigFile loads configuration from JSON or YAML file
func loadConfigFile(configPath string, config *Config) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Try JSON first
	if err := json.Unmarshal(data, config); err != nil {
		// Try YAML
		if yamlErr := yaml.Unmarshal(data, config); yamlErr != nil {
			return fmt.Errorf("failed to parse config as JSON or YAML: %v, %v", err, yamlErr)
		}
	}

	return nil
}

// loadEnvironmentConfig loads configuration overrides from environment variables
func loadEnvironmentConfig(config *Config) error {
	// Server configuration
	if port := os.Getenv("GOSSIS_HTTP_PORT"); port != "" {
		config.Server.Port = port
	}

	// Package directory
	if pkgDir := os.Getenv("GOSSIS_PKG_DIRECTORY"); pkgDir != "" {
		config.Packages.Directory = pkgDir
	}

	// Logging configuration
	if logLevel := os.Getenv("GOSSIS_LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("GOSSIS_LOG_FORMAT"); logFormat != "" {
		config.Logging.Format = logFormat
	}

	return nil
}

// ValidateConfig validates the configuration
func ValidateConfig(config Config) error {
	// Validate port
	if config.Server.Port != "" {
		if port, err := strconv.Atoi(config.Server.Port); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid server port: %s", config.Server.Port)
		}
	}

	// Validate package directory if specified
	if config.Packages.Directory != "" {
		if _, err := os.Stat(config.Packages.Directory); os.IsNotExist(err) {
			return fmt.Errorf("package directory does not exist: %s", config.Packages.Directory)
		}
	}

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(config.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", config.Logging.Level)
	}

	// Validate log format
	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[strings.ToLower(config.Logging.Format)] {
		return fmt.Errorf("invalid log format: %s", config.Logging.Format)
	}

	return nil
}

// mergeConfigs merges two configurations (for future use with multiple config files)
func mergeConfigs(base, override Config) Config {
	result := base

	// Merge server config
	if override.Server.Port != "" {
		result.Server.Port = override.Server.Port
	}
	if override.Server.HTTPMode {
		result.Server.HTTPMode = override.Server.HTTPMode
	}

	// Merge package config
	if override.Packages.Directory != "" {
		result.Packages.Directory = override.Packages.Directory
	}
	if override.Packages.ExcludeFile != "" {
		result.Packages.ExcludeFile = override.Packages.ExcludeFile
	}

	// Merge logging config
	if override.Logging.Level != "" {
		result.Logging.Level = override.Logging.Level
	}
	if override.Logging.Format != "" {
		result.Logging.Format = override.Logging.Format
	}

	return result
}

// DefaultPluginConfig returns default plugin configuration
func DefaultPluginConfig() PluginConfig {
	return PluginConfig{
		PluginDir:         "./plugins",
		EnabledPlugins:    []string{"ssis-core-analysis"},
		CommunityRegistry: "https://registry.gossismcp.com",
		AutoUpdate:        true,
		Security: PluginSecurity{
			AllowNetworkAccess: false,
			AllowedDomains:     []string{},
			SignatureRequired:  false,
			TrustedPublishers:  []string{"gossisMCP"},
		},
	}
}
