package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfig tests the default configuration values
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test server config defaults
	assert.False(t, config.Server.HTTPMode)
	assert.Equal(t, "8086", config.Server.Port)

	// Test package config defaults
	assert.Empty(t, config.Packages.Directory)

	// Test logging config defaults
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)

	// Test plugin config defaults
	assert.Equal(t, "./plugins", config.Plugins.PluginDir)
	assert.Equal(t, []string{"ssis-core-analysis"}, config.Plugins.EnabledPlugins)
	assert.Equal(t, "https://registry.gossismcp.com", config.Plugins.CommunityRegistry)
	assert.True(t, config.Plugins.AutoUpdate)
	assert.False(t, config.Plugins.Security.AllowNetworkAccess)
	assert.Empty(t, config.Plugins.Security.AllowedDomains)
	assert.False(t, config.Plugins.Security.SignatureRequired)
	assert.Equal(t, []string{"gossisMCP"}, config.Plugins.Security.TrustedPublishers)
}

// TestLoadConfigFromJSON tests loading configuration from JSON file
func TestLoadConfigFromJSON(t *testing.T) {
	// Create temporary JSON config file
	jsonConfig := `{
		"server": {
			"http_mode": true,
			"port": "9090"
		},
		"packages": {
			"directory": ""
		},
		"logging": {
			"level": "debug",
			"format": "json"
		},
		"plugins": {
			"plugin_dir": "/tmp/plugins",
			"enabled_plugins": ["test-plugin"],
			"community_registry": "https://test-registry.com",
			"auto_update": false,
			"security": {
				"allow_network_access": true,
				"allowed_domains": ["example.com"],
				"signature_required": false,
				"trusted_publishers": ["test-publisher"]
			}
		}
	}`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	err := os.WriteFile(configFile, []byte(jsonConfig), 0644)
	require.NoError(t, err)

	// Test loading config
	config, err := LoadConfig(configFile)
	require.NoError(t, err)

	// Verify loaded values
	assert.True(t, config.Server.HTTPMode)
	assert.Equal(t, "9090", config.Server.Port)
	assert.Equal(t, "", config.Packages.Directory)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "/tmp/plugins", config.Plugins.PluginDir)
	assert.Equal(t, []string{"test-plugin"}, config.Plugins.EnabledPlugins)
	assert.Equal(t, "https://test-registry.com", config.Plugins.CommunityRegistry)
	assert.False(t, config.Plugins.AutoUpdate)
	assert.True(t, config.Plugins.Security.AllowNetworkAccess)
	assert.Equal(t, []string{"example.com"}, config.Plugins.Security.AllowedDomains)
	assert.False(t, config.Plugins.Security.SignatureRequired)
	assert.Equal(t, []string{"test-publisher"}, config.Plugins.Security.TrustedPublishers)
}

// TestLoadConfigFromYAML tests loading configuration from YAML file
func TestLoadConfigFromYAML(t *testing.T) {
	// Create temporary YAML config file
	yamlConfig := `server:
  http_mode: true
  port: "9090"
packages:
  directory: ""
logging:
  level: "debug"
  format: "json"
plugins:
  plugin_dir: "/tmp/plugins"
  enabled_plugins:
    - "test-plugin"
  community_registry: "https://test-registry.com"
  auto_update: false
  security:
    allow_network_access: true
    allowed_domains:
      - "example.com"
    signature_required: false
    trusted_publishers:
      - "test-publisher"`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(yamlConfig), 0644)
	require.NoError(t, err)

	// Test loading config
	config, err := LoadConfig(configFile)
	require.NoError(t, err)

	// Verify loaded values
	assert.True(t, config.Server.HTTPMode)
	assert.Equal(t, "9090", config.Server.Port)
	assert.Equal(t, "", config.Packages.Directory)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, "/tmp/plugins", config.Plugins.PluginDir)
	assert.Equal(t, []string{"test-plugin"}, config.Plugins.EnabledPlugins)
	assert.Equal(t, "https://test-registry.com", config.Plugins.CommunityRegistry)
	assert.False(t, config.Plugins.AutoUpdate)
	assert.True(t, config.Plugins.Security.AllowNetworkAccess)
	assert.Equal(t, []string{"example.com"}, config.Plugins.Security.AllowedDomains)
	assert.False(t, config.Plugins.Security.SignatureRequired)
	assert.Equal(t, []string{"test-publisher"}, config.Plugins.Security.TrustedPublishers)
	assert.False(t, config.Plugins.Security.SignatureRequired)
	assert.Equal(t, []string{"test-publisher"}, config.Plugins.Security.TrustedPublishers)
}

// TestLoadConfigInvalidFile tests loading configuration from non-existent file
func TestLoadConfigInvalidFile(t *testing.T) {
	_, err := LoadConfig("/non/existent/file.json")
	assert.Error(t, err)
}

// TestLoadConfigInvalidJSON tests loading invalid JSON configuration (should fall back to defaults)
func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configFile, []byte(`{"invalid": json}`), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configFile)
	// Should succeed with default values
	assert.NoError(t, err)
	assert.Equal(t, DefaultConfig(), config)
}

// TestLoadConfigInvalidYAML tests loading invalid YAML configuration
func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	err := os.WriteFile(configFile, []byte(`invalid: yaml: content: [`), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configFile)
	assert.Error(t, err)
}

// TestLoadConfigUnsupportedFormat tests loading configuration with unsupported file extension
func TestLoadConfigUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.txt")

	err := os.WriteFile(configFile, []byte("some content"), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config as JSON or YAML")
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	// Test valid config
	config := DefaultConfig()
	err := validateConfig(config)
	assert.NoError(t, err)

	// Test invalid port
	config.Server.Port = "invalid"
	err = validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid server port")

	// Test invalid log level
	config = DefaultConfig()
	config.Logging.Level = "invalid"
	err = validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")

	// Test invalid log format
	config = DefaultConfig()
	config.Logging.Format = "invalid"
	err = validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log format")
}

// TestEnvironmentVariableOverrides tests that environment variables override config file values
func TestEnvironmentVariableOverrides(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"GOSSIS_HTTP_PORT":     "9999",
		"GOSSIS_PKG_DIRECTORY": "",
		"GOSSIS_LOG_LEVEL":     "debug",
		"GOSSIS_LOG_FORMAT":    "json",
	}

	// Save original values
	originalEnv := make(map[string]string)
	for key := range envVars {
		originalEnv[key] = os.Getenv(key)
	}

	// Set test values
	for key, value := range envVars {
		os.Setenv(key, value)
	}

	// Restore original values after test
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Create config file with different values
	jsonConfig := `{
		"server": {
			"http_mode": false,
			"port": "8086"
		},
		"packages": {
			"directory": ""
		},
		"logging": {
			"level": "info",
			"format": "text"
		}
	}`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	err := os.WriteFile(configFile, []byte(jsonConfig), 0644)
	require.NoError(t, err)

	// Load config - environment variables should override
	config, err := LoadConfig(configFile)
	require.NoError(t, err)

	// Verify environment variable overrides
	assert.Equal(t, "9999", config.Server.Port)
	assert.Equal(t, "", config.Packages.Directory)
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
}
