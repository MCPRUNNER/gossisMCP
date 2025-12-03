package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ExamplePlugin demonstrates a basic gossisMCP plugin
type ExamplePlugin struct{}

// PluginInfo provides plugin information
var PluginInfo = map[string]interface{}{
	"id":          "example-ssis-plugin",
	"name":        "Example SSIS Plugin",
	"version":     "1.0.0",
	"description": "Example plugin demonstrating gossisMCP plugin development",
	"author":      "gossisMCP Team",
	"category":    "Analysis",
	"tags":        []string{"example", "analysis", "tutorial"},
	"tools": []map[string]interface{}{
		{
			"name":        "detect_hardcoded_connections",
			"description": "Detect hardcoded connection strings in DTSX files",
			"parameters": []map[string]interface{}{
				{
					"name":        "file_path",
					"type":        "string",
					"description": "Path to the DTSX file to analyze",
					"required":    true,
				},
				{
					"name":        "severity",
					"type":        "string",
					"description": "Severity level for findings",
					"required":    false,
					"default":     "warning",
					"enum":        []string{"info", "warning", "error"},
				},
			},
			"category": "security",
			"tags":     []string{"connections", "hardcoded", "security"},
		},
		{
			"name":        "analyze_variable_usage",
			"description": "Analyze SSIS variable usage patterns",
			"parameters": []map[string]interface{}{
				{
					"name":        "file_path",
					"type":        "string",
					"description": "Path to the DTSX file to analyze",
					"required":    true,
				},
			},
			"category": "analysis",
			"tags":     []string{"variables", "usage"},
		},
	},
}

// Metadata returns plugin metadata
func (p *ExamplePlugin) Metadata() map[string]interface{} {
	return PluginInfo
}

// Tools returns the tools provided by this plugin
func (p *ExamplePlugin) Tools() []map[string]interface{} {
	return PluginInfo["tools"].([]map[string]interface{})
}

// ExecuteTool executes a tool
func (p *ExamplePlugin) ExecuteTool(ctx context.Context, name string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	switch name {
	case "detect_hardcoded_connections":
		return p.detectHardcodedConnections(ctx, request)
	case "analyze_variable_usage":
		return p.analyzeVariableUsage(ctx, request)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// detectHardcodedConnections detects hardcoded connection strings
func (p *ExamplePlugin) detectHardcodedConnections(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath := request.GetString("file_path", "")
	severity := request.GetString("severity", "warning")

	// Read file content (in real implementation, use proper file reading)
	content := `
<DTS:ConnectionManager DTS:refId="Package.ConnectionManagers[ConnectionString]">
  <DTS:Property DTS:Name="ConnectionString">Server=myServer;Database=myDB;User Id=myUser;Password=myPassword;</DTS:Property>
</DTS:ConnectionManager>
`

	// Simple regex to detect potential hardcoded passwords
	passwordRegex := regexp.MustCompile(`Password\s*=\s*[^;]+`)
	matches := passwordRegex.FindAllString(content, -1)

	findings := make([]string, 0, len(matches))
	for _, match := range matches {
		findings = append(findings, fmt.Sprintf("[%s] Hardcoded password detected: %s", severity, match))
	}

	result := map[string]interface{}{
		"file_analyzed":  filePath,
		"severity":       severity,
		"findings":       findings,
		"total_findings": len(findings),
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hardcoded connection analysis result: %d findings found\nDetails: %+v", len(findings), result)), nil
}

// analyzeVariableUsage analyzes SSIS variable usage
func (p *ExamplePlugin) analyzeVariableUsage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath := request.GetString("file_path", "")

	// Mock DTSX content with variables
	content := `
<DTS:Variable DTS:refId="Package.Variables[User::SourcePath]">
  <DTS:VariableValue DTS:DataType="8">C:\Data\Source</DTS:VariableValue>
</DTS:Variable>
<DTS:Variable DTS:refId="Package.Variables[User::UnusedVar]">
  <DTS:VariableValue DTS:DataType="8">Unused Value</DTS:VariableValue>
</DTS:Variable>
<DTS:Property DTS:Expression="@[User::SourcePath] + "\input.txt""</DTS:Property>
`

	// Extract variables
	varRegex := regexp.MustCompile(`Package\.Variables\[([^\]]+)\]`)
	varMatches := varRegex.FindAllStringSubmatch(content, -1)

	variables := make([]string, 0, len(varMatches))
	for _, match := range varMatches {
		if len(match) > 1 {
			variables = append(variables, match[1])
		}
	}

	// Check usage
	used := make([]string, 0)
	unused := make([]string, 0)

	for _, variable := range variables {
		if strings.Contains(content, "@["+variable+"]") {
			used = append(used, variable)
		} else {
			unused = append(unused, variable)
		}
	}

	result := map[string]interface{}{
		"file_analyzed":    filePath,
		"total_variables":  len(variables),
		"used_variables":   used,
		"unused_variables": unused,
		"usage_ratio":      fmt.Sprintf("%.1f%%", float64(len(used))/float64(len(variables))*100),
	}

	return mcp.NewToolResultText(fmt.Sprintf("Variable usage analysis: %d total, %d used, %d unused\nDetails: %+v", len(variables), len(used), len(unused), result)), nil
}

// Plugin instance - this is what the plugin system looks for
var Plugin = &ExamplePlugin{}
