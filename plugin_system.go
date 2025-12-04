package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MCPRUNNER/gossisMCP/pkg/config"
	"github.com/mark3labs/mcp-go/mcp"
)

// PluginSystem manages the entire plugin ecosystem
type PluginSystem struct {
	registry    *PluginRegistry
	manager     *PluginManager
	marketplace *PluginMarketplace
	config      config.PluginConfig
	mu          sync.RWMutex
}

// PluginRegistry manages plugin metadata and discovery
type PluginRegistry struct {
	plugins map[string]*PluginMetadata
	mu      sync.RWMutex
}

// PluginMetadata contains information about a plugin
type PluginMetadata struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Description  string           `json:"description"`
	Author       string           `json:"author"`
	Tags         []string         `json:"tags"`
	Dependencies []string         `json:"dependencies"`
	Tools        []PluginTool     `json:"tools"`
	Resources    []PluginResource `json:"resources"`
	Homepage     string           `json:"homepage"`
	Repository   string           `json:"repository"`
	License      string           `json:"license"`
	Rating       float64          `json:"rating"`
	Downloads    int              `json:"downloads"`
	PublishedAt  time.Time        `json:"published_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	Signature    string           `json:"signature,omitempty"`
}

// PluginTool defines a tool provided by a plugin
type PluginTool struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Parameters  []ParameterDefinition `json:"parameters"`
	Category    string                `json:"category"`
	Tags        []string              `json:"tags"`
}

// ParameterDefinition defines a tool parameter
type ParameterDefinition struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
}

// PluginResource defines a resource provided by a plugin
type PluginResource struct {
	URI         string   `json:"uri"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MimeType    string   `json:"mime_type"`
	Tags        []string `json:"tags"`
}

// PluginManager handles loading and executing plugins
type PluginManager struct {
	loadedPlugins map[string]*LoadedPlugin
	registry      *PluginRegistry
	mu            sync.RWMutex
}

// LoadedPlugin represents a loaded plugin instance
type LoadedPlugin struct {
	Metadata  *PluginMetadata
	Plugin    *plugin.Plugin
	Tools     map[string]ToolExecutor
	Resources map[string]ResourceHandler
	Enabled   bool
}

// ToolExecutor defines the interface for tool execution
type ToolExecutor interface {
	Execute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// ResourceHandler defines the interface for resource handling
type ResourceHandler interface {
	Read(ctx context.Context, uri string) ([]byte, error)
	List(ctx context.Context, uri string) ([]mcp.Resource, error)
}

// PluginMarketplace manages community plugin discovery and installation
type PluginMarketplace struct {
	registryURL string
	client      *http.Client
	cache       map[string]*PluginMetadata
	cacheTime   time.Time
	mu          sync.RWMutex
}

// NewPluginSystem creates a new plugin system instance
func NewPluginSystem(config config.PluginConfig) *PluginSystem {
	return &PluginSystem{
		registry:    NewPluginRegistry(),
		manager:     NewPluginManager(),
		marketplace: NewPluginMarketplace(config.CommunityRegistry),
		config:      config,
	}
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]*PluginMetadata),
	}
}

// NewPluginManager creates a new plugin manager
func NewPluginManager() *PluginManager {
	return &PluginManager{
		loadedPlugins: make(map[string]*LoadedPlugin),
	}
}

// NewPluginMarketplace creates a new plugin marketplace
func NewPluginMarketplace(registryURL string) *PluginMarketplace {
	return &PluginMarketplace{
		registryURL: registryURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: make(map[string]*PluginMetadata),
	}
}

// Initialize sets up the plugin system
func (ps *PluginSystem) Initialize() error {
	// Create plugin directory if it doesn't exist
	if err := os.MkdirAll(ps.config.PluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Load built-in plugins
	if err := ps.loadBuiltinPlugins(); err != nil {
		return fmt.Errorf("failed to load builtin plugins: %w", err)
	}

	// Load installed plugins
	if err := ps.loadInstalledPlugins(); err != nil {
		return fmt.Errorf("failed to load installed plugins: %w", err)
	}

	// Auto-update plugins if enabled
	if ps.config.AutoUpdate {
		go ps.autoUpdatePlugins()
	}

	return nil
}

// loadBuiltinPlugins loads the core analysis plugins
func (ps *PluginSystem) loadBuiltinPlugins() error {
	builtinPlugins := []PluginMetadata{
		{
			ID:          "ssis-core-analysis",
			Name:        "SSIS Core Analysis",
			Version:     "1.0.0",
			Description: "Core SSIS package analysis tools",
			Author:      "gossisMCP Team",
			Tags:        []string{"core", "analysis", "ssis"},
			Tools: []PluginTool{
				{
					Name:        "parse_dtsx",
					Description: "Parse SSIS package structure",
					Category:    "analysis",
					Tags:        []string{"parsing", "structure"},
				},
				{
					Name:        "extract_tasks",
					Description: "Extract task information",
					Category:    "analysis",
					Tags:        []string{"tasks", "extraction"},
				},
				// Add more core tools...
			},
		},
	}

	for _, metadata := range builtinPlugins {
		ps.registry.Register(&metadata)
	}

	return nil
}

// loadInstalledPlugins loads plugins from the plugin directory
func (ps *PluginSystem) loadInstalledPlugins() error {
	files, err := filepath.Glob(filepath.Join(ps.config.PluginDir, "*.so"))
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := ps.manager.LoadPlugin(file); err != nil {
			log.Printf("Failed to load plugin %s: %v", file, err)
			continue
		}
	}

	return nil
}

// autoUpdatePlugins periodically checks for plugin updates
func (ps *PluginSystem) autoUpdatePlugins() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := ps.marketplace.UpdateCache(); err != nil {
			log.Printf("Failed to update plugin cache: %v", err)
		}

		// Check for updates to installed plugins
		ps.checkForUpdates()
	}
}

// checkForUpdates checks for available plugin updates
func (ps *PluginSystem) checkForUpdates() {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	for _, loaded := range ps.manager.loadedPlugins {
		if latest, err := ps.marketplace.GetLatestVersion(loaded.Metadata.ID); err == nil {
			if latest.Version != loaded.Metadata.Version {
				log.Printf("Update available for plugin %s: %s -> %s",
					loaded.Metadata.ID, loaded.Metadata.Version, latest.Version)
			}
		}
	}
}

// Register registers a plugin in the registry
func (pr *PluginRegistry) Register(metadata *PluginMetadata) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.plugins[metadata.ID] = metadata
}

// Get retrieves a plugin from the registry
func (pr *PluginRegistry) Get(id string) (*PluginMetadata, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	plugin, exists := pr.plugins[id]
	return plugin, exists
}

// List returns all registered plugins
func (pr *PluginRegistry) List() []*PluginMetadata {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	plugins := make([]*PluginMetadata, 0, len(pr.plugins))
	for _, plugin := range pr.plugins {
		plugins = append(plugins, plugin)
	}

	// Sort by name for consistent ordering
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})

	return plugins
}

// LoadPlugin loads a plugin from a .so file
func (pm *PluginManager) LoadPlugin(path string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return err
	}

	// Get plugin metadata
	sym, err := p.Lookup("PluginMetadata")
	if err != nil {
		return fmt.Errorf("plugin metadata not found: %w", err)
	}

	metadata, ok := sym.(*PluginMetadata)
	if !ok {
		return fmt.Errorf("invalid plugin metadata type")
	}

	loaded := &LoadedPlugin{
		Metadata:  metadata,
		Plugin:    p,
		Tools:     make(map[string]ToolExecutor),
		Resources: make(map[string]ResourceHandler),
		Enabled:   true,
	}

	// Load tools
	for _, tool := range metadata.Tools {
		sym, err := p.Lookup(strings.Title(tool.Name) + "Tool")
		if err != nil {
			log.Printf("Tool %s not found in plugin %s", tool.Name, metadata.ID)
			continue
		}

		executor, ok := sym.(ToolExecutor)
		if !ok {
			log.Printf("Invalid tool executor type for %s", tool.Name)
			continue
		}

		loaded.Tools[tool.Name] = executor
	}

	// Load resources
	for _, resource := range metadata.Resources {
		sym, err := p.Lookup(strings.Title(resource.Name) + "Resource")
		if err != nil {
			log.Printf("Resource %s not found in plugin %s", resource.Name, metadata.ID)
			continue
		}

		handler, ok := sym.(ResourceHandler)
		if !ok {
			log.Printf("Invalid resource handler type for %s", resource.Name)
			continue
		}

		loaded.Resources[resource.URI] = handler
	}

	pm.mu.Lock()
	pm.loadedPlugins[metadata.ID] = loaded
	pm.mu.Unlock()

	return nil
}

// GetTool retrieves a tool executor
func (pm *PluginManager) GetTool(pluginID, toolName string) (ToolExecutor, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if loaded, exists := pm.loadedPlugins[pluginID]; exists {
		if tool, exists := loaded.Tools[toolName]; exists && loaded.Enabled {
			return tool, true
		}
	}

	return nil, false
}

// GetResource retrieves a resource handler
func (pm *PluginManager) GetResource(pluginID, uri string) (ResourceHandler, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if loaded, exists := pm.loadedPlugins[pluginID]; exists {
		if resource, exists := loaded.Resources[uri]; exists && loaded.Enabled {
			return resource, true
		}
	}

	return nil, false
}

// UpdateCache updates the marketplace plugin cache
func (pm *PluginMarketplace) UpdateCache() error {
	resp, err := pm.client.Get(pm.registryURL + "/plugins")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var plugins []PluginMetadata
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		return err
	}

	pm.mu.Lock()
	pm.cache = make(map[string]*PluginMetadata)
	for i := range plugins {
		pm.cache[plugins[i].ID] = &plugins[i]
	}
	pm.cacheTime = time.Now()
	pm.mu.Unlock()

	return nil
}

// GetLatestVersion gets the latest version of a plugin
func (pm *PluginMarketplace) GetLatestVersion(pluginID string) (*PluginMetadata, error) {
	pm.mu.RLock()
	plugin, exists := pm.cache[pluginID]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found in marketplace")
	}

	return plugin, nil
}

// SearchPlugins searches for plugins in the marketplace
func (pm *PluginMarketplace) SearchPlugins(query string, tags []string) []*PluginMetadata {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var results []*PluginMetadata
	for _, plugin := range pm.cache {
		if pm.matchesQuery(plugin, query, tags) {
			results = append(results, plugin)
		}
	}

	// Sort by rating and downloads
	sort.Slice(results, func(i, j int) bool {
		if results[i].Rating != results[j].Rating {
			return results[i].Rating > results[j].Rating
		}
		return results[i].Downloads > results[j].Downloads
	})

	return results
}

// matchesQuery checks if a plugin matches search criteria
func (pm *PluginMarketplace) matchesQuery(plugin *PluginMetadata, query string, tags []string) bool {
	// Check text query
	if query != "" {
		query = strings.ToLower(query)
		if !strings.Contains(strings.ToLower(plugin.Name), query) &&
			!strings.Contains(strings.ToLower(plugin.Description), query) {
			return false
		}
	}

	// Check tags
	if len(tags) > 0 {
		tagMap := make(map[string]bool)
		for _, tag := range plugin.Tags {
			tagMap[strings.ToLower(tag)] = true
		}

		for _, requiredTag := range tags {
			if !tagMap[strings.ToLower(requiredTag)] {
				return false
			}
		}
	}

	return true
}

// InstallPlugin installs a plugin from the marketplace
func (pm *PluginMarketplace) InstallPlugin(pluginID, version string) error {
	// Get plugin download URL
	resp, err := pm.client.Get(fmt.Sprintf("%s/plugins/%s/%s/download", pm.registryURL, pluginID, version))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create plugin file
	pluginPath := filepath.Join("plugins", pluginID+".so")
	file, err := os.Create(pluginPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Download plugin
	_, err = io.Copy(file, resp.Body)
	return err
}

// GetPluginStats returns marketplace statistics
func (pm *PluginMarketplace) GetPluginStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalPlugins := len(pm.cache)
	totalDownloads := 0
	avgRating := 0.0

	for _, plugin := range pm.cache {
		totalDownloads += plugin.Downloads
		avgRating += plugin.Rating
	}

	if totalPlugins > 0 {
		avgRating /= float64(totalPlugins)
	}

	return map[string]interface{}{
		"total_plugins":   totalPlugins,
		"total_downloads": totalDownloads,
		"average_rating":  avgRating,
		"last_updated":    pm.cacheTime,
	}
}

// DefaultPluginConfig returns default plugin configuration
func DefaultPluginConfig() config.PluginConfig {
	return config.PluginConfig{
		PluginDir:         "./plugins",
		EnabledPlugins:    []string{"ssis-core-analysis"},
		CommunityRegistry: "https://registry.gossismcp.com",
		AutoUpdate:        true,
		Security: config.PluginSecurity{
			AllowNetworkAccess: false,
			AllowedDomains:     []string{},
			SignatureRequired:  false,
			TrustedPublishers:  []string{"gossisMCP"},
		},
	}
}
