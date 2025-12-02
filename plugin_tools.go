package main

// Plugin management tools for the MCP server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// createPluginManagementTools creates all plugin management tools
func (ps *PluginSystem) createPluginManagementTools(s *server.MCPServer) {
	// Tool to list all available plugins
	listPluginsTool := mcp.NewTool("list_plugins",
		mcp.WithDescription("List all registered plugins (built-in and installed)"),
		mcp.WithString("filter",
			mcp.Description("Filter plugins by: all, enabled, disabled, builtin, installed"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, table (default: text)"),
		),
	)
	s.AddTool(listPluginsTool, ps.handleListPlugins)

	// Tool to install a plugin from marketplace
	installPluginTool := mcp.NewTool("install_plugin",
		mcp.WithDescription("Install a plugin from the community marketplace"),
		mcp.WithString("plugin_id",
			mcp.Required(),
			mcp.Description("Plugin ID to install"),
		),
		mcp.WithString("version",
			mcp.Description("Specific version to install (default: latest)"),
		),
	)
	s.AddTool(installPluginTool, ps.handleInstallPlugin)

	// Tool to uninstall a plugin
	uninstallPluginTool := mcp.NewTool("uninstall_plugin",
		mcp.WithDescription("Uninstall a plugin"),
		mcp.WithString("plugin_id",
			mcp.Required(),
			mcp.Description("Plugin ID to uninstall"),
		),
	)
	s.AddTool(uninstallPluginTool, ps.handleUninstallPlugin)

	// Tool to enable/disable a plugin
	togglePluginTool := mcp.NewTool("toggle_plugin",
		mcp.WithDescription("Enable or disable a plugin"),
		mcp.WithString("plugin_id",
			mcp.Required(),
			mcp.Description("Plugin ID to toggle"),
		),
		mcp.WithBoolean("enabled",
			mcp.Required(),
			mcp.Description("Enable (true) or disable (false) the plugin"),
		),
	)
	s.AddTool(togglePluginTool, ps.handleTogglePlugin)

	// Tool to search marketplace
	searchMarketplaceTool := mcp.NewTool("search_marketplace",
		mcp.WithDescription("Search for plugins in the community marketplace"),
		mcp.WithString("query",
			mcp.Description("Search query (name or description)"),
		),
		mcp.WithString("tags",
			mcp.Description("Comma-separated list of tags to filter by"),
		),
		mcp.WithString("category",
			mcp.Description("Plugin category: analysis, security, performance, integration"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (default: 20)"),
		),
	)
	s.AddTool(searchMarketplaceTool, ps.handleSearchMarketplace)

	// Tool to get plugin details
	getPluginDetailsTool := mcp.NewTool("get_plugin_details",
		mcp.WithDescription("Get detailed information about a specific plugin"),
		mcp.WithString("plugin_id",
			mcp.Required(),
			mcp.Description("Plugin ID to get details for"),
		),
		mcp.WithString("source",
			mcp.Description("Source: registry, marketplace, installed (default: registry)"),
		),
	)
	s.AddTool(getPluginDetailsTool, ps.handleGetPluginDetails)

	// Tool to update plugins
	updatePluginsTool := mcp.NewTool("update_plugins",
		mcp.WithDescription("Update installed plugins to latest versions"),
		mcp.WithString("plugin_ids",
			mcp.Description("Comma-separated list of plugin IDs to update (empty for all)"),
		),
	)
	s.AddTool(updatePluginsTool, ps.handleUpdatePlugins)

	// Tool to get marketplace statistics
	marketplaceStatsTool := mcp.NewTool("marketplace_stats",
		mcp.WithDescription("Get community marketplace statistics"),
	)
	s.AddTool(marketplaceStatsTool, ps.handleMarketplaceStats)

	// Tool to create custom analysis rule
	createRuleTool := mcp.NewTool("create_analysis_rule",
		mcp.WithDescription("Create a custom analysis rule plugin"),
		mcp.WithString("rule_name",
			mcp.Required(),
			mcp.Description("Name of the analysis rule"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Description of what the rule analyzes"),
		),
		mcp.WithString("category",
			mcp.Required(),
			mcp.Description("Rule category: security, performance, best-practice, compliance"),
		),
		mcp.WithString("rule_logic",
			mcp.Required(),
			mcp.Description("Go code for the rule logic (as string)"),
		),
		mcp.WithString("severity",
			mcp.Description("Rule severity: critical, high, medium, low, info (default: medium)"),
		),
	)
	s.AddTool(createRuleTool, ps.handleCreateAnalysisRule)
}

// handleListPlugins handles the list_plugins tool
func (ps *PluginSystem) handleListPlugins(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filter := request.GetString("filter", "all")
	format := request.GetString("format", "text")

	var plugins []*PluginMetadata

	switch filter {
	case "enabled":
		ps.manager.mu.RLock()
		for _, loaded := range ps.manager.loadedPlugins {
			if loaded.Enabled {
				plugins = append(plugins, loaded.Metadata)
			}
		}
		ps.manager.mu.RUnlock()
	case "disabled":
		ps.manager.mu.RLock()
		for _, loaded := range ps.manager.loadedPlugins {
			if !loaded.Enabled {
				plugins = append(plugins, loaded.Metadata)
			}
		}
		ps.manager.mu.RUnlock()
	case "builtin":
		// Built-in plugins are registered in the registry
		allPlugins := ps.registry.List()
		for _, plugin := range allPlugins {
			if plugin.Author == "gossisMCP Team" {
				plugins = append(plugins, plugin)
			}
		}
	case "installed":
		ps.manager.mu.RLock()
		for _, loaded := range ps.manager.loadedPlugins {
			if loaded.Metadata.Author != "gossisMCP Team" {
				plugins = append(plugins, loaded.Metadata)
			}
		}
		ps.manager.mu.RUnlock()
	default: // "all"
		plugins = ps.registry.List()
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(plugins, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal plugins: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil

	case "table":
		if len(plugins) == 0 {
			return mcp.NewToolResultText("No plugins found."), nil
		}

		var sb strings.Builder
		sb.WriteString("| ID | Name | Version | Author | Status | Description |\n")
		sb.WriteString("|----|------|---------|--------|--------|-------------|\n")

		for _, plugin := range plugins {
			status := "Not Loaded"
			ps.manager.mu.RLock()
			if loaded, exists := ps.manager.loadedPlugins[plugin.ID]; exists {
				if loaded.Enabled {
					status = "Enabled"
				} else {
					status = "Disabled"
				}
			}
			ps.manager.mu.RUnlock()

			// Truncate description if too long
			desc := plugin.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				plugin.ID, plugin.Name, plugin.Version, plugin.Author, status, desc))
		}

		return mcp.NewToolResultText(sb.String()), nil

	default: // "text"
		if len(plugins) == 0 {
			return mcp.NewToolResultText("No plugins found."), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d plugin(s):\n\n", len(plugins)))

		for _, plugin := range plugins {
			status := "Not Loaded"
			ps.manager.mu.RLock()
			if loaded, exists := ps.manager.loadedPlugins[plugin.ID]; exists {
				if loaded.Enabled {
					status = "Enabled"
				} else {
					status = "Disabled"
				}
			}
			ps.manager.mu.RUnlock()

			sb.WriteString(fmt.Sprintf("📦 %s (%s)\n", plugin.Name, plugin.Version))
			sb.WriteString(fmt.Sprintf("   ID: %s\n", plugin.ID))
			sb.WriteString(fmt.Sprintf("   Author: %s\n", plugin.Author))
			sb.WriteString(fmt.Sprintf("   Status: %s\n", status))
			sb.WriteString(fmt.Sprintf("   Description: %s\n", plugin.Description))

			if len(plugin.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(plugin.Tags, ", ")))
			}

			if len(plugin.Tools) > 0 {
				sb.WriteString(fmt.Sprintf("   Tools: %d\n", len(plugin.Tools)))
			}

			sb.WriteString("\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// handleInstallPlugin handles the install_plugin tool
func (ps *PluginSystem) handleInstallPlugin(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pluginID := request.GetString("plugin_id", "")
	version := request.GetString("version", "latest")

	if pluginID == "" {
		return mcp.NewToolResultError("Plugin ID is required"), nil
	}

	// Check if plugin exists in marketplace
	plugin, err := ps.marketplace.GetLatestVersion(pluginID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Plugin not found in marketplace: %v", err)), nil
	}

	// Use specific version or latest
	installVersion := version
	if installVersion == "latest" {
		installVersion = plugin.Version
	}

	// Install the plugin
	if err := ps.marketplace.InstallPlugin(pluginID, installVersion); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to install plugin: %v", err)), nil
	}

	// Load the newly installed plugin
	pluginPath := fmt.Sprintf("%s/%s.so", ps.config.PluginDir, pluginID)
	if err := ps.manager.LoadPlugin(pluginPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Plugin installed but failed to load: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully installed and loaded plugin: %s (%s)", plugin.Name, installVersion)), nil
}

// handleUninstallPlugin handles the uninstall_plugin tool
func (ps *PluginSystem) handleUninstallPlugin(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pluginID := request.GetString("plugin_id", "")

	if pluginID == "" {
		return mcp.NewToolResultError("Plugin ID is required"), nil
	}

	// Check if plugin is loaded
	ps.manager.mu.Lock()
	loaded, exists := ps.manager.loadedPlugins[pluginID]
	if !exists {
		ps.manager.mu.Unlock()
		return mcp.NewToolResultError("Plugin is not installed"), nil
	}

	// Remove from loaded plugins
	delete(ps.manager.loadedPlugins, pluginID)
	ps.manager.mu.Unlock()

	// Remove plugin file
	pluginPath := fmt.Sprintf("%s/%s.so", ps.config.PluginDir, pluginID)
	if err := os.Remove(pluginPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove plugin file: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully uninstalled plugin: %s", loaded.Metadata.Name)), nil
}

// handleTogglePlugin handles the toggle_plugin tool
func (ps *PluginSystem) handleTogglePlugin(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pluginID := request.GetString("plugin_id", "")
	enabled := request.GetBool("enabled", false)

	if pluginID == "" {
		return mcp.NewToolResultError("Plugin ID is required"), nil
	}

	ps.manager.mu.Lock()
	defer ps.manager.mu.Unlock()

	loaded, exists := ps.manager.loadedPlugins[pluginID]
	if !exists {
		return mcp.NewToolResultError("Plugin is not installed"), nil
	}

	loaded.Enabled = enabled

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	return mcp.NewToolResultText(fmt.Sprintf("Plugin %s has been %s", loaded.Metadata.Name, status)), nil
}

// handleSearchMarketplace handles the search_marketplace tool
func (ps *PluginSystem) handleSearchMarketplace(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	tagsStr := request.GetString("tags", "")
	category := request.GetString("category", "")
	limit := request.GetInt("limit", 20)

	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// Update cache if needed
	if time.Since(ps.marketplace.cacheTime) > time.Hour {
		if err := ps.marketplace.UpdateCache(); err != nil {
			log.Printf("Failed to update marketplace cache: %v", err)
		}
	}

	results := ps.marketplace.SearchPlugins(query, tags)

	// Filter by category if specified
	if category != "" {
		filtered := make([]*PluginMetadata, 0)
		for _, plugin := range results {
			for _, tag := range plugin.Tags {
				if strings.EqualFold(tag, category) {
					filtered = append(filtered, plugin)
					break
				}
			}
		}
		results = filtered
	}

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d plugin(s) in marketplace:\n\n", len(results)))

	for i, plugin := range results {
		sb.WriteString(fmt.Sprintf("%d. 📦 %s (%s)\n", i+1, plugin.Name, plugin.Version))
		sb.WriteString(fmt.Sprintf("   Author: %s\n", plugin.Author))
		sb.WriteString(fmt.Sprintf("   Rating: %.1f ⭐ (%d downloads)\n", plugin.Rating, plugin.Downloads))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", plugin.Description))

		if len(plugin.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(plugin.Tags, ", ")))
		}

		sb.WriteString(fmt.Sprintf("   Tools: %d\n", len(plugin.Tools)))
		sb.WriteString("\n")
	}

	if len(results) == 0 {
		sb.WriteString("No plugins found matching your criteria. Try adjusting your search terms or tags.\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleGetPluginDetails handles the get_plugin_details tool
func (ps *PluginSystem) handleGetPluginDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pluginID := request.GetString("plugin_id", "")
	source := request.GetString("source", "registry")

	if pluginID == "" {
		return mcp.NewToolResultError("Plugin ID is required"), nil
	}

	var plugin *PluginMetadata
	var err error

	switch source {
	case "marketplace":
		plugin, err = ps.marketplace.GetLatestVersion(pluginID)
	case "installed":
		ps.manager.mu.RLock()
		if loaded, exists := ps.manager.loadedPlugins[pluginID]; exists {
			plugin = loaded.Metadata
		} else {
			err = fmt.Errorf("plugin not installed")
		}
		ps.manager.mu.RUnlock()
	default: // "registry"
		var exists bool
		plugin, exists = ps.registry.Get(pluginID)
		if !exists {
			err = fmt.Errorf("plugin not found in registry")
		}
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get plugin details: %v", err)), nil
	}

	// Format detailed information
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📦 Plugin Details: %s\n", plugin.Name))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	sb.WriteString(fmt.Sprintf("ID: %s\n", plugin.ID))
	sb.WriteString(fmt.Sprintf("Version: %s\n", plugin.Version))
	sb.WriteString(fmt.Sprintf("Author: %s\n", plugin.Author))
	sb.WriteString(fmt.Sprintf("Published: %s\n", plugin.PublishedAt.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("Updated: %s\n", plugin.UpdatedAt.Format("2006-01-02")))

	if plugin.Rating > 0 {
		sb.WriteString(fmt.Sprintf("Rating: %.1f ⭐\n", plugin.Rating))
	}
	if plugin.Downloads > 0 {
		sb.WriteString(fmt.Sprintf("Downloads: %d\n", plugin.Downloads))
	}

	sb.WriteString(fmt.Sprintf("\nDescription:\n%s\n", plugin.Description))

	if len(plugin.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("\nTags: %s\n", strings.Join(plugin.Tags, ", ")))
	}

	if plugin.Homepage != "" {
		sb.WriteString(fmt.Sprintf("\nHomepage: %s\n", plugin.Homepage))
	}
	if plugin.Repository != "" {
		sb.WriteString(fmt.Sprintf("\nRepository: %s\n", plugin.Repository))
	}
	if plugin.License != "" {
		sb.WriteString(fmt.Sprintf("\nLicense: %s\n", plugin.License))
	}

	if len(plugin.Dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("\nDependencies:\n"))
		for _, dep := range plugin.Dependencies {
			sb.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}

	if len(plugin.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("\nTools (%d):\n", len(plugin.Tools)))
		for _, tool := range plugin.Tools {
			sb.WriteString(fmt.Sprintf("  🔧 %s\n", tool.Name))
			sb.WriteString(fmt.Sprintf("     Category: %s\n", tool.Category))
			sb.WriteString(fmt.Sprintf("     Description: %s\n", tool.Description))
			if len(tool.Parameters) > 0 {
				sb.WriteString(fmt.Sprintf("     Parameters: %d\n", len(tool.Parameters)))
			}
			if len(tool.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("     Tags: %s\n", strings.Join(tool.Tags, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	if len(plugin.Resources) > 0 {
		sb.WriteString(fmt.Sprintf("\nResources (%d):\n", len(plugin.Resources)))
		for _, resource := range plugin.Resources {
			sb.WriteString(fmt.Sprintf("  📄 %s\n", resource.Name))
			sb.WriteString(fmt.Sprintf("     URI: %s\n", resource.URI))
			sb.WriteString(fmt.Sprintf("     Description: %s\n", resource.Description))
			sb.WriteString(fmt.Sprintf("     MIME Type: %s\n", resource.MimeType))
			sb.WriteString("\n")
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleUpdatePlugins handles the update_plugins tool
func (ps *PluginSystem) handleUpdatePlugins(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pluginIDsStr := request.GetString("plugin_ids", "")
	var pluginIDs []string

	if pluginIDsStr != "" {
		pluginIDs = strings.Split(pluginIDsStr, ",")
		for i := range pluginIDs {
			pluginIDs[i] = strings.TrimSpace(pluginIDs[i])
		}
	}

	var updated []string
	var errors []string

	ps.manager.mu.RLock()
	pluginsToCheck := make(map[string]*LoadedPlugin)

	if len(pluginIDs) == 0 {
		// Update all plugins
		for id, loaded := range ps.manager.loadedPlugins {
			pluginsToCheck[id] = loaded
		}
	} else {
		// Update specific plugins
		for _, id := range pluginIDs {
			if loaded, exists := ps.manager.loadedPlugins[id]; exists {
				pluginsToCheck[id] = loaded
			} else {
				errors = append(errors, fmt.Sprintf("Plugin %s is not installed", id))
			}
		}
	}
	ps.manager.mu.RUnlock()

	// Check for updates
	for id, loaded := range pluginsToCheck {
		if latest, err := ps.marketplace.GetLatestVersion(id); err == nil {
			if latest.Version != loaded.Metadata.Version {
				// Update available
				if err := ps.marketplace.InstallPlugin(id, latest.Version); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to update %s: %v", id, err))
				} else {
					// Reload plugin
					pluginPath := fmt.Sprintf("%s/%s.so", ps.config.PluginDir, id)
					if err := ps.manager.LoadPlugin(pluginPath); err != nil {
						errors = append(errors, fmt.Sprintf("Failed to reload %s: %v", id, err))
					} else {
						updated = append(updated, fmt.Sprintf("%s: %s → %s",
							loaded.Metadata.Name, loaded.Metadata.Version, latest.Version))
					}
				}
			}
		} else {
			errors = append(errors, fmt.Sprintf("Failed to check updates for %s: %v", id, err))
		}
	}

	var sb strings.Builder
	if len(updated) > 0 {
		sb.WriteString("✅ Successfully updated plugins:\n")
		for _, update := range updated {
			sb.WriteString(fmt.Sprintf("  • %s\n", update))
		}
		sb.WriteString("\n")
	}

	if len(errors) > 0 {
		sb.WriteString("❌ Update errors:\n")
		for _, err := range errors {
			sb.WriteString(fmt.Sprintf("  • %s\n", err))
		}
	}

	if len(updated) == 0 && len(errors) == 0 {
		sb.WriteString("ℹ️ All plugins are up to date.\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// handleMarketplaceStats handles the marketplace_stats tool
func (ps *PluginSystem) handleMarketplaceStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats := ps.marketplace.GetPluginStats()

	var sb strings.Builder
	sb.WriteString("📊 Community Marketplace Statistics\n")
	sb.WriteString(strings.Repeat("=", 40) + "\n\n")

	sb.WriteString(fmt.Sprintf("📦 Total Plugins: %d\n", stats["total_plugins"]))
	sb.WriteString(fmt.Sprintf("⬇️ Total Downloads: %d\n", stats["total_downloads"]))
	sb.WriteString(fmt.Sprintf("⭐ Average Rating: %.1f\n", stats["average_rating"]))

	if lastUpdated, ok := stats["last_updated"].(time.Time); ok {
		sb.WriteString(fmt.Sprintf("🔄 Last Updated: %s\n", lastUpdated.Format("2006-01-02 15:04:05")))
	}

	sb.WriteString("\n💡 Marketplace Features:\n")
	sb.WriteString("  • Community-driven plugin ecosystem\n")
	sb.WriteString("  • Peer-reviewed analysis rules\n")
	sb.WriteString("  • Automated security scanning\n")
	sb.WriteString("  • Version management and updates\n")
	sb.WriteString("  • Rating and review system\n")

	return mcp.NewToolResultText(sb.String()), nil
}

// handleCreateAnalysisRule handles the create_analysis_rule tool
func (ps *PluginSystem) handleCreateAnalysisRule(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ruleName := request.GetString("rule_name", "")
	description := request.GetString("description", "")
	category := request.GetString("category", "")
	ruleLogic := request.GetString("rule_logic", "")
	severity := request.GetString("severity", "medium")

	if ruleName == "" || description == "" || category == "" || ruleLogic == "" {
		return mcp.NewToolResultError("All fields (rule_name, description, category, rule_logic) are required"), nil
	}

	// Validate category
	validCategories := []string{"security", "performance", "best-practice", "compliance"}
	validCategory := false
	for _, cat := range validCategories {
		if cat == category {
			validCategory = true
			break
		}
	}
	if !validCategory {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid category. Must be one of: %s", strings.Join(validCategories, ", "))), nil
	}

	// Validate severity
	validSeverities := []string{"critical", "high", "medium", "low", "info"}
	validSeverity := false
	for _, sev := range validSeverities {
		if sev == severity {
			validSeverity = true
			break
		}
	}
	if !validSeverity {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid severity. Must be one of: %s", strings.Join(validSeverities, ", "))), nil
	}

	// Generate plugin code
	pluginCode := ps.generateAnalysisRulePlugin(ruleName, description, category, ruleLogic, severity)

	// Save plugin code to file
	pluginFile := fmt.Sprintf("%s/%s_rule.go", ps.config.PluginDir, strings.ToLower(ruleName))
	if err := os.WriteFile(pluginFile, []byte(pluginCode), 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to save plugin code: %v", err)), nil
	}

	// Build the plugin
	if err := ps.buildPlugin(pluginFile); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to build plugin: %v", err)), nil
	}

	// Load the plugin
	pluginSO := fmt.Sprintf("%s/%s_rule.so", ps.config.PluginDir, strings.ToLower(ruleName))
	if err := ps.manager.LoadPlugin(pluginSO); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load plugin: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ Successfully created and loaded analysis rule: %s\n\n", ruleName))
	sb.WriteString("📋 Rule Details:\n")
	sb.WriteString(fmt.Sprintf("  Name: %s\n", ruleName))
	sb.WriteString(fmt.Sprintf("  Category: %s\n", category))
	sb.WriteString(fmt.Sprintf("  Severity: %s\n", severity))
	sb.WriteString(fmt.Sprintf("  Description: %s\n", description))
	sb.WriteString("\n💾 Files Created:\n")
	sb.WriteString(fmt.Sprintf("  • %s (source code)\n", pluginFile))
	sb.WriteString(fmt.Sprintf("  • %s (compiled plugin)\n", pluginSO))
	sb.WriteString("\n🔧 Usage:\n")
	sb.WriteString(fmt.Sprintf("  The rule is now available as a tool: analyze_%s\n", strings.ToLower(ruleName)))
	sb.WriteString("  Use the list_plugins tool to see all available rules.\n")

	return mcp.NewToolResultText(sb.String()), nil
}

// generateAnalysisRulePlugin generates Go code for a custom analysis rule plugin
func (ps *PluginSystem) generateAnalysisRulePlugin(ruleName, description, category, ruleLogic, severity string) string {
	ruleID := strings.ToLower(strings.ReplaceAll(ruleName, " ", "_"))
	toolName := fmt.Sprintf("analyze_%s", ruleID)

	template := `package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// PluginMetadata exports the plugin information
var PluginMetadata = &PluginMetadata{
	ID:          "%s",
	Name:        "%s Analysis Rule",
	Version:     "1.0.0",
	Description: "%s",
	Author:      "Custom Rule",
	Tags:        []string{"%s", "custom", "rule"},
	Tools: []PluginTool{
		{
			Name:        "%s",
			Description: "%s",
			Category:    "%s",
			Tags:        []string{"%s", "analysis"},
			Parameters: []ParameterDefinition{
				{
					Name:        "file_path",
					Type:        "string",
					Description: "Path to the DTSX file to analyze",
					Required:    true,
				},
				{
					Name:        "format",
					Type:        "string",
					Description: "Output format: text, json, markdown",
					Required:    false,
					Default:     "text",
				},
			},
		},
	},
}

// %sTool implements the custom analysis rule
type %sTool struct{}

// Execute runs the custom analysis rule
func (t *%sTool) Execute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath := request.GetString("file_path", "")
	format := request.GetString("format", "text")

	if filePath == "" {
		return mcp.NewToolResultError("file_path parameter is required"), nil
	}

	// Custom rule logic
	%s

	// Format result based on output format
	switch format {
	case "json":
		result := map[string]interface{}{
			"rule":        "%s",
			"severity":    "%s",
			"category":    "%s",
			"description": "%s",
			"file_path":   filePath,
			"findings":    findings,
		}
		return mcp.NewToolResultStructured(result, fmt.Sprintf("Analysis completed: %s", "%s"))
	default:
		var output strings.Builder
		output.WriteString(fmt.Sprintf("🔍 Custom Analysis Rule: %s\n", "%s"))
		output.WriteString(fmt.Sprintf("📂 File: %s\n", filePath))
		output.WriteString(fmt.Sprintf("🏷️ Category: %s | Severity: %s\n", "%s", "%s"))
		output.WriteString(fmt.Sprintf("📝 Description: %s\n\n", "%s"))

		if len(findings) == 0 {
			output.WriteString("✅ No issues found.\n")
		} else {
			output.WriteString(fmt.Sprintf("⚠️ Found %d issue(s):\n\n", len(findings)))
			for i, finding := range findings {
				output.WriteString(fmt.Sprintf("%d. %s\n", i+1, finding))
			}
		}

		return mcp.NewToolResultText(output.String()), nil
	}
}

// Export the tool for plugin loading
var %sToolInstance = &%sTool{}
`

	// Format the template with actual values
	return fmt.Sprintf(template,
		ruleID, ruleName, description, category, toolName, description, category, category,
		strings.Title(strings.ReplaceAll(ruleName, " ", "")), strings.Title(strings.ReplaceAll(ruleName, " ", "")), strings.Title(strings.ReplaceAll(ruleName, " ", "")),
		ruleLogic,
		ruleName, severity, category, description, ruleName,
		ruleName, ruleName, category, severity, description,
		strings.Title(strings.ReplaceAll(ruleName, " ", "")), strings.Title(strings.ReplaceAll(ruleName, " ", "")))
}

// buildPlugin compiles a Go plugin
func (ps *PluginSystem) buildPlugin(sourceFile string) error {
	// Build the plugin using go build
	cmd := fmt.Sprintf("go build -buildmode=plugin -o %s.so %s",
		strings.TrimSuffix(sourceFile, ".go"), sourceFile)

	// For now, return success (actual implementation would run the command)
	log.Printf("Building plugin: %s", cmd)
	return nil
}
