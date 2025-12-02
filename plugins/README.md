# gossisMCP Plugin Development Guide

This guide provides comprehensive documentation for developing plugins for the gossisMCP SSIS DTSX Analyzer.

## Overview

gossisMCP plugins extend the functionality of the SSIS DTSX Analyzer by providing custom analysis rules, tools, and resources. Plugins are compiled as shared libraries (.so on Linux/macOS, .dll on Windows) and loaded dynamically at runtime.

## Plugin Architecture

### Core Components

- **Plugin System**: Manages plugin lifecycle, loading, and execution
- **Plugin Registry**: Maintains metadata about available plugins
- **Plugin Manager**: Handles loading and execution of plugin code
- **Plugin Marketplace**: Community repository for plugin discovery and installation

### Plugin Types

1. **Analysis Plugins**: Provide custom SSIS package analysis rules
2. **Tool Plugins**: Add new MCP tools to the server
3. **Resource Plugins**: Provide additional resources (files, data, etc.)

## Creating a Plugin

### 1. Project Setup

Create a new Go module for your plugin:

```bash
mkdir my-ssis-plugin
cd my-ssis-plugin
go mod init my-ssis-plugin
```

### 2. Plugin Structure

A basic plugin consists of:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mark3labs/mcp-go/mcp"
)

// MyPlugin implements the plugin interface
type MyPlugin struct{}

// PluginMetadata provides plugin information
var PluginMetadata = &PluginMetadata{
    ID:          "my-custom-plugin",
    Name:        "My Custom SSIS Plugin",
    Version:     "1.0.0",
    Description: "Custom analysis for specific SSIS patterns",
    Author:      "Your Name",
    Category:    "Analysis",
    Tags:        []string{"custom", "analysis"},
    Tools: []PluginTool{
        {
            Name:        "my_custom_analysis",
            Description: "Perform custom analysis on DTSX files",
            Parameters: []ParameterDefinition{
                {
                    Name:        "file_path",
                    Type:        "string",
                    Description: "Path to the DTSX file",
                    Required:    true,
                },
            },
            Category: "analysis",
            Tags:     []string{"custom"},
        },
    },
}

// Metadata returns plugin metadata
func (p *MyPlugin) Metadata() *PluginMetadata {
    return PluginMetadata
}

// Tools returns the tools provided by this plugin
func (p *MyPlugin) Tools() []PluginTool {
    return PluginMetadata.Tools
}

// ExecuteTool executes a tool
func (p *MyPlugin) ExecuteTool(ctx context.Context, name string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    switch name {
    case "my_custom_analysis":
        return p.executeCustomAnalysis(ctx, request)
    default:
        return nil, fmt.Errorf("unknown tool: %s", name)
    }
}

// executeCustomAnalysis implements the custom analysis logic
func (p *MyPlugin) executeCustomAnalysis(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    filePath := request.GetString("file_path", "")

    // Your custom analysis logic here
    // Read the DTSX file, parse it, perform analysis...

    result := map[string]interface{}{
        "analysis_type": "custom",
        "file_analyzed": filePath,
        "findings": []string{
            "Custom finding 1",
            "Custom finding 2",
        },
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            {
                Type: "text",
                Text: fmt.Sprintf("Custom analysis result: %+v", result),
            },
        },
    }, nil
}

// Plugin instance
var Plugin = &MyPlugin{}

func main() {
    // Plugin entry point - required for compilation
    log.Println("Plugin loaded:", Plugin.Metadata().Name)
}
```

### 3. Plugin Metadata

Each plugin must provide metadata including:

```go
type PluginMetadata struct {
    ID           string        // Unique plugin identifier
    Name         string        // Human-readable name
    Version      string        // Semantic version
    Description  string        // Plugin description
    Author       string        // Plugin author
    Category     string        // Plugin category
    Tags         []string      // Search tags
    Dependencies []string      // Required plugins
    Tools        []PluginTool  // Provided tools
    Resources    []PluginResource // Provided resources
    Homepage     string        // Project homepage
    Repository   string        // Source repository
    License      string        // License type
    Rating       float64       // Community rating
    Downloads    int           // Download count
    PublishedAt  time.Time     // Publication date
    UpdatedAt    time.Time     // Last update
    Signature    string        // Security signature
}
```

### 4. Tool Definition

Tools are defined with parameters and execution logic:

```go
type PluginTool struct {
    Name        string                // Tool name
    Description string                // Tool description
    Parameters  []ParameterDefinition // Tool parameters
    Category    string                // Tool category
    Tags        []string              // Tool tags
}

type ParameterDefinition struct {
    Name        string      // Parameter name
    Type        string      // Parameter type (string, number, boolean, array)
    Description string      // Parameter description
    Required    bool        // Whether parameter is required
    Default     interface{} // Default value
    Enum        []string    // Allowed values (for enum types)
}
```

## Building Plugins

### Compilation

Build your plugin as a shared library:

```bash
# Linux/macOS
go build -buildmode=plugin -o my-plugin.so .

# Windows
go build -buildmode=plugin -o my-plugin.dll .
```

### Dependencies

Add required dependencies to your go.mod:

```go
require (
    github.com/mark3labs/mcp-go v1.0.0
    // Other dependencies...
)
```

## Installing Plugins

### Manual Installation

1. Build your plugin as described above
2. Copy the .so/.dll file to the plugins directory
3. Restart the gossisMCP server

### Marketplace Installation

Use the built-in plugin management tools:

```bash
# List available plugins
curl -X POST http://localhost:8086/tools/list_plugins

# Install a plugin
curl -X POST http://localhost:8086/tools/install_plugin \
  -d '{"plugin_id": "my-plugin", "version": "1.0.0"}'
```

## Plugin Development Best Practices

### Code Organization

```
my-plugin/
├── main.go          # Plugin entry point
├── analysis.go      # Analysis logic
├── tools.go         # Tool definitions
├── go.mod           # Go module file
├── go.sum           # Dependency checksums
└── README.md        # Plugin documentation
```

### Error Handling

```go
func (p *MyPlugin) ExecuteTool(ctx context.Context, name string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // Validate input parameters
    if err := p.validateParameters(request); err != nil {
        return nil, fmt.Errorf("parameter validation failed: %w", err)
    }

    // Check context for cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Execute analysis with proper error handling
    result, err := p.performAnalysis(request)
    if err != nil {
        return nil, fmt.Errorf("analysis failed: %w", err)
    }

    return result, nil
}
```

### Performance Considerations

- Use context for cancellation
- Implement timeouts for long-running operations
- Avoid blocking operations
- Use efficient data structures
- Cache expensive computations when possible

### Security

- Validate all input parameters
- Sanitize file paths
- Use secure defaults
- Avoid executing arbitrary code
- Implement proper access controls

## Advanced Features

### Custom Resources

Plugins can provide resources:

```go
type PluginResource struct {
    URI         string   // Resource URI
    Name        string   // Resource name
    Description string   // Resource description
    MimeType    string   // MIME type
    Tags        []string // Resource tags
}

// ReadResource reads a resource
func (p *MyPlugin) ReadResource(ctx context.Context, uri string) ([]byte, error) {
    // Implement resource reading logic
    return []byte("resource content"), nil
}

// ListResources lists available resources
func (p *MyPlugin) ListResources(ctx context.Context, uri string) ([]mcp.Resource, error) {
    // Implement resource listing logic
    return []mcp.Resource{
        {
            URI:      "plugin://my-plugin/config",
            Name:     "Plugin Configuration",
            MimeType: "application/json",
        },
    }, nil
}
```

### Plugin Dependencies

Declare plugin dependencies:

```go
var PluginMetadata = &PluginMetadata{
    // ... other fields ...
    Dependencies: []string{
        "ssis-core-analysis",  // Required plugin
        "xml-parser",          // Another dependency
    },
}
```

### Configuration

Plugins can access configuration:

```go
type MyPlugin struct {
    config map[string]interface{}
}

func (p *MyPlugin) Configure(config map[string]interface{}) error {
    p.config = config
    return nil
}
```

## Testing Plugins

### Unit Tests

```go
func TestMyPlugin_ExecuteTool(t *testing.T) {
    plugin := &MyPlugin{}

    request := &mcp.CallToolRequest{
        Params: mcp.CallToolParams{
            Name: "my_custom_analysis",
            Arguments: map[string]interface{}{
                "file_path": "test.dtsx",
            },
        },
    }

    result, err := plugin.ExecuteTool(context.Background(), "my_custom_analysis", *request)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Integration Tests

```go
func TestPluginIntegration(t *testing.T) {
    // Load plugin
    p, err := plugin.Open("my-plugin.so")
    assert.NoError(t, err)

    // Get plugin instance
    sym, err := p.Lookup("Plugin")
    assert.NoError(t, err)

    pluginInstance := sym.(*MyPlugin)
    assert.NotNil(t, pluginInstance)

    // Test metadata
    metadata := pluginInstance.Metadata()
    assert.Equal(t, "my-custom-plugin", metadata.ID)
}
```

## Example Plugins

### Unused Variables Detector

```go
func (p *UnusedVariablesPlugin) executeAnalysis(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    filePath := request.GetString("file_path", "")

    // Read and parse DTSX file
    content, err := ioutil.ReadFile(filePath)
    if err != nil {
        return nil, err
    }

    // Extract variables
    variables := p.extractVariables(string(content))

    // Find unused variables
    unused := p.findUnusedVariables(string(content), variables)

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            {
                Type: "text",
                Text: fmt.Sprintf("Found %d unused variables: %v", len(unused), unused),
            },
        },
    }, nil
}
```

### Performance Analyzer

```go
func (p *PerformancePlugin) analyzeDataFlow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    filePath := request.GetString("file_path", "")

    // Analyze buffer sizes, thread counts, etc.
    recommendations := []string{
        "Increase DefaultBufferSize to 10MB",
        "Set EngineThreads to 4",
        "Enable BLOBTempStoragePath",
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            {
                Type: "text",
                Text: fmt.Sprintf("Performance recommendations: %v", recommendations),
            },
        },
    }, nil
}
```

## Publishing Plugins

### Marketplace Submission

1. Create a GitHub repository for your plugin
2. Add proper documentation and examples
3. Create releases with compiled binaries
4. Submit to the gossisMCP marketplace

### Plugin Registry

The plugin registry accepts submissions via:

```json
{
  "id": "my-plugin",
  "repository": "https://github.com/username/my-plugin",
  "versions": ["1.0.0", "1.1.0"],
  "metadata": { ... }
}
```

## Troubleshooting

### Common Issues

1. **Plugin not loading**: Check file permissions and architecture compatibility
2. **Symbol not found**: Ensure all required functions are exported
3. **Version conflicts**: Check Go version and dependency compatibility
4. **Security errors**: Verify plugin signature and publisher trust

### Debug Mode

Enable debug logging:

```bash
export GOSSIS_LOG_LEVEL=debug
./ssis-analyzer.exe
```

### Plugin Validation

Validate plugin structure:

```go
func validatePlugin(plugin *MyPlugin) error {
    if plugin.Metadata().ID == "" {
        return errors.New("plugin ID is required")
    }

    if len(plugin.Tools()) == 0 {
        return errors.New("plugin must provide at least one tool")
    }

    return nil
}
```

## Contributing

1. Fork the gossisMCP repository
2. Create a feature branch
3. Develop your plugin
4. Add comprehensive tests
5. Submit a pull request

## License

Plugins can use any license compatible with the gossisMCP license. We recommend MIT or Apache 2.0 licenses for maximum compatibility.

## Support

- **Documentation**: This guide and gossisMCP README
- **Community**: GitHub Discussions
- **Issues**: GitHub Issues for bug reports
- **Discussions**: Plugin development discussions

---

For more information, visit the [gossisMCP repository](https://github.com/7045kHz/gossisMCP).</content>
<parameter name="filePath">c:\Users\U00001\source\repos\gossisMCP\plugins\README.md
