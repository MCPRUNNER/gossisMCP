package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != "8086" {
		t.Fatalf("expected default port 8086, got %s", cfg.Server.Port)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("expected default log level info, got %s", cfg.Logging.Level)
	}
	if cfg.Plugins.PluginDir != "./plugins" {
		t.Fatalf("expected default plugin directory ./plugins, got %s", cfg.Plugins.PluginDir)
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = "7000"
	cfg.Packages.Directory = t.TempDir()
	cfg.Logging.Level = "warn"
	cfg.Logging.Format = "json"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected config to be valid, got error %v", err)
	}

	cfg.Server.Port = "invalid"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected invalid port to return error")
	}
}

func TestMergeConfigs(t *testing.T) {
	base := DefaultConfig()
	override := Config{
		Server: ServerConfig{
			HTTPMode: true,
			Port:     "1234",
		},
		Packages: PackageConfig{Directory: "packages"},
		Logging:  LoggingConfig{Level: "debug", Format: "json"},
	}
	merged := mergeConfigs(base, override)
	if !merged.Server.HTTPMode || merged.Server.Port != "1234" {
		t.Fatalf("expected server overrides to apply, got %+v", merged.Server)
	}
	if merged.Packages.Directory != "packages" {
		t.Fatalf("expected package directory override, got %s", merged.Packages.Directory)
	}
	if merged.Logging.Level != "debug" || merged.Logging.Format != "json" {
		t.Fatalf("expected logging overrides, got %+v", merged.Logging)
	}
}

func TestDefaultPluginConfig(t *testing.T) {
	cfg := DefaultPluginConfig()
	if cfg.PluginDir != "./plugins" {
		t.Fatalf("unexpected plugin dir %s", cfg.PluginDir)
	}
	if len(cfg.EnabledPlugins) == 0 {
		t.Fatal("expected at least one enabled plugin")
	}
	if !cfg.AutoUpdate {
		t.Fatal("expected auto update enabled by default")
	}
}

func TestLoadConfigFromJSON(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "config.json")
	escapedDir := strings.ReplaceAll(tempDir, "\\", "\\\\")
	contents := `{
        "server": {"http_mode": true, "port": "9090"},
        "packages": {"directory": "` + escapedDir + `"},
        "logging": {"level": "error", "format": "text"}
    }`
	if err := os.WriteFile(filePath, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	t.Setenv("GOSSIS_LOG_LEVEL", "warn")
	cfg, err := LoadConfig(filePath)
	if err != nil {
		t.Fatalf("expected config to load, got error %v", err)
	}
	if !cfg.Server.HTTPMode || cfg.Server.Port != "9090" {
		t.Fatalf("expected server values from file, got %+v", cfg.Server)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected env override for log level, got %s", cfg.Logging.Level)
	}
	if cfg.Packages.Directory != tempDir {
		t.Fatalf("expected package directory %s, got %s", tempDir, cfg.Packages.Directory)
	}
}

func TestConfigureLogging(t *testing.T) {
	originalFlags := log.Flags()
	t.Cleanup(func() { log.SetFlags(originalFlags) })

	ConfigureLogging(LoggingConfig{Level: "debug"})
	if log.Flags()&log.Lmicroseconds == 0 {
		t.Fatalf("expected microseconds flag when log level is debug")
	}

	ConfigureLogging(LoggingConfig{Level: "info"})
	expected := log.LstdFlags | log.Lshortfile
	if log.Flags() != expected {
		t.Fatalf("expected flags %d, got %d", expected, log.Flags())
	}
}
