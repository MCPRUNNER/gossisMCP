# SSIS DTSX Analyzer MCP Server

This is a comprehensive Model Context Protocol (MCP) server written in Go that provides 80+ advanced tools for analyzing SSIS (SQL Server Integration Services) DTSX files. It offers detailed insights into package structure, data flow components (10 source types, 10 transform types, 6 destination types), control flow tasks, logging configurations, performance metrics, and best practices validation.

## Features

- **Package Analysis**: Parse DTSX files and extract comprehensive package information
- **Batch Processing**: Analyze multiple DTSX files in parallel with aggregated results and performance metrics
- **Unified Analysis Interfaces**: Streamlined `analyze_source` and `analyze_destination` tools provide type-based analysis for all supported component types, reducing API complexity while maintaining full functionality
- **Event Handler Analysis**: Analyze event handlers (OnError, OnWarning, OnPreExecute, etc.) with their tasks, variables, and precedence constraints
- **Precedence Constraint Analysis**: Extract and analyze precedence constraints with expression resolution
- **Connection Management**: Extract and analyze connection manager details with expression resolution
- **Variable Extraction**: List all package and task variables with expression resolution
- **Parameter Extraction**: Extract project and package parameters with their properties, data types, and default values
- **Package Dependency Mapping**: Analyze relationships between packages, shared connections, and variables across multiple DTSX files
- **Configuration Analysis**: Analyze package configurations (XML, SQL Server, environment variable configs) with types, filters, and property mappings
- **Performance Metrics Analysis**: Analyze data flow performance settings (buffer sizes, engine threads, etc.) to identify bottlenecks and optimization opportunities
- **Security Analysis**: Detect potential security issues (hardcoded credentials, sensitive data exposure)
- **Package Comparison**: Compare two DTSX files and highlight differences
- **Code Quality Metrics**: Calculate maintainability metrics (complexity, duplication, etc.)
- **Read Text File Integration**: Read configuration or data from text files referenced by SSIS packages
- **Script Code Analysis**: Extract C#/VB.NET code from Script Tasks and Script Components
- **Script Task Analysis**: Comprehensive analysis of Script Tasks including variables, entry points, and configuration
- **Logging Configuration**: Detailed analysis of logging providers, events, and destinations
- **Best Practices Validation**: Check SSIS packages for best practices and potential issues
- **Hard-coded Value Detection**: Identify embedded literals in connection strings, messages, and expressions
- **Interactive Queries**: Ask specific questions about DTSX files and get relevant information
- **File Structure Validation**: Validate DTSX file structure and integrity
- **Multiple Output Formats**: Support for text, JSON, CSV, HTML, and Markdown output formats
- **HTTP Streaming Support**: Optional HTTP API with streaming responses for real-time output
- **Plugin System**: Extensible architecture supporting custom analysis rules and community plugins

## Plugin System

The SSIS DTSX Analyzer includes a comprehensive plugin system that allows for extensibility and customization:

### Features

- **Custom Analysis Rules**: Create and install custom analysis rules for specific SSIS patterns
- **Community Plugin Repository**: Access a marketplace of community-contributed plugins
- **Plugin Management**: Install, uninstall, enable/disable, and update plugins
- **Security Features**: Plugin signature verification and sandboxed execution
- **Plugin Development**: Easy-to-use templates and APIs for creating new plugins

### Plugin Management Tools

The server provides several tools for managing plugins:

- `list_plugins`: List all registered plugins (built-in and installed)
- `install_plugin`: Install a plugin from the community marketplace
- `uninstall_plugin`: Uninstall a plugin
- `enable_plugin`: Enable or disable a plugin
- `search_plugins`: Search for plugins in the marketplace
- `update_plugin`: Update a plugin to the latest version
- `create_custom_rule`: Create a custom analysis rule
- `execute_custom_rule`: Execute a custom analysis rule on a DTSX file

### Configuration

Plugin system settings can be configured in the configuration file:

```json
{
  "plugins": {
    "plugin_dir": "./plugins",
    "enabled_plugins": ["ssis-core-analysis"],
    "community_registry": "https://registry.gossismcp.com",
    "auto_update": true,
    "security": {
      "allow_network_access": false,
      "allowed_domains": [],
      "signature_required": true,
      "trusted_publishers": ["gossisMCP"]
    }
  }
}
```

### Plugin Development

Plugins are Go modules that implement the plugin interface and are compiled as shared libraries (.so files on Linux/macOS, .dll on Windows). Here's how to create a custom plugin:

1. **Create a new Go module:**

```bash
mkdir my-ssis-plugin
cd my-ssis-plugin
go mod init my-ssis-plugin
```

2. **Implement the plugin interface:**

```go
package main

import (
    "context"
    "github.com/mark3labs/mcp-go/mcp"
)

// MyPlugin implements the plugin interface
type MyPlugin struct{}

// Metadata returns plugin metadata
func (p *MyPlugin) Metadata() map[string]interface{} {
    return map[string]interface{}{
        "id": "my-custom-plugin",
        "name": "My Custom SSIS Plugin",
        "version": "1.0.0",
        "description": "Custom analysis for specific SSIS patterns",
        "author": "Your Name",
        "category": "Analysis",
        "tags": []string{"custom", "analysis"},
    }
}

// Tools returns the tools provided by this plugin
func (p *MyPlugin) Tools() []map[string]interface{} {
    return []map[string]interface{}{
        {
            "name": "my_custom_analysis",
            "description": "Perform custom analysis on DTSX files",
            "parameters": []map[string]interface{}{
                {
                    "name": "file_path",
                    "type": "string",
                    "description": "Path to the DTSX file",
                    "required": true,
                },
            },
        },
    }
}

// ExecuteTool executes a tool
func (p *MyPlugin) ExecuteTool(ctx context.Context, name string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    switch name {
    case "my_custom_analysis":
        filePath := request.GetString("file_path", "")
        // Perform your custom analysis here
        result := map[string]interface{}{
            "analysis": "Custom analysis result",
            "file": filePath,
        }
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                {
                    Type: "text",
                    Text: fmt.Sprintf("Analysis result: %+v", result),
                },
            },
        }, nil
    }
    return nil, fmt.Errorf("unknown tool: %s", name)
}

// Export the plugin
var Plugin MyPlugin

func main() {
    // Plugin entry point
}
```

3. **Build the plugin:**

```bash
go build -buildmode=plugin -o my-plugin.so .
```

4. **Install the plugin:**
   Use the `install_plugin` tool or manually place the .so/.dll file in the plugins directory.

### Testing Strategy

- Run `go test ./...` before publishing or updating plugins; this executes core plugin-system tests plus the example plugin coverage that validates tool metadata, execution, and default severity handling.
- Keep plugin code modular so you can back the MCP tool entry points with functions that are testable without loading the Go `plugin` package.
- For marketplace or end-to-end scenarios, add integration tests that exercise install/enable flows via the plugin management handlers once mock registries are available (tracked in roadmap).

## Prerequisites

- Go 1.19 or later
- Access to SSIS DTSX files

## Installation

1. Clone or download this repository
2. Navigate to the project directory
3. Install dependencies:

```bash
go mod tidy
```

4. Build the server:

```bash
go build -o ssis-analyzer.exe .
```

5. Run the server:

**Standard MCP mode (default):**

```bash
./ssis-analyzer.exe
```

**HTTP streaming mode:**

```bash
./ssis-analyzer.exe -http
```

You can also specify a package directory:

```bash
./ssis-analyzer.exe -http -pkg-dir /path/to/ssis/packages
```

Or set the `GOSSIS_PKG_DIRECTORY` environment variable:

```bash
export GOSSIS_PKG_DIRECTORY=/path/to/ssis/packages
./ssis-analyzer.exe -http
```

## Usage

This MCP server is designed to be used with MCP-compatible clients (like Claude Desktop or other AI assistants that support MCP).

### Command Line Options

- `-http`: Run in HTTP streaming mode (default port 8086)
- `-port`: HTTP server port (default: 8086)
- `-pkg-dir`: Root directory for SSIS packages (can also be set via `GOSSIS_PKG_DIRECTORY` environment variable, defaults to current working directory)
- `-config`: Path to configuration file (JSON or YAML format)

### Configuration Files

The server supports configuration files in JSON or YAML format for more advanced configuration management. Configuration files allow you to set server settings, package directories, and logging options.

**Example JSON configuration (`config.json`):**

```json
{
  "server": {
    "http_mode": false,
    "port": "8086"
  },
  "packages": {
    "directory": "path/to/ssis/packages",
    "exclude_file": ".gossisignore"
  },
  "logging": {
    "level": "info",
    "format": "text"
  }
}
```

Create a `.gossisignore` file in the same directory as the configured `packages.directory` to skip folders (for example `bin/` or `obj/`) during directory scans used by tools like `list_packages` and `batch_analyze`. Use one pattern per line; lines starting with `#` are treated as comments.

**Example YAML configuration (`config.yaml`):**

```yaml
server:
  http_mode: false
  port: "8086"
packages:
  directory: "path/to/ssis/packages"
  exclude_file: ".gossisignore"
logging:
  level: "info"
  format: "text"
```

The same `.gossisignore` file is honored when using the YAML configuration.

**Configuration Options:**

- `server.http_mode`: Whether to run in HTTP streaming mode (boolean)
- `server.port`: HTTP server port (string)
- `packages.directory`: Root directory for SSIS packages (string)
- `packages.exclude_file`: Optional path to a `.gossisignore`-style file for excluding subpaths during scans (string, relative to `packages.directory` if not absolute)
- `logging.level`: Log level - "debug", "info", "warn", "error" (string)
- `logging.format`: Log format - "text" or "json" (string)

**Environment Variables:**

You can override configuration values using environment variables:

- `GOSSIS_HTTP_PORT`: Override server port
- `GOSSIS_PKG_DIRECTORY`: Override package directory
- `GOSSIS_LOG_LEVEL`: Override log level ("debug", "info", "warn", "error")
- `GOSSIS_LOG_FORMAT`: Override log format ("text", "json")

**Usage with configuration file:**

```bash
./ssis-analyzer.exe -config config.json
./ssis-analyzer.exe -config config.yaml
```

Command line flags take precedence over configuration file settings and environment variables.

### Package Directory Feature

The server supports specifying a root directory for SSIS packages using either the `-pkg-dir` command line flag or the `GOSSIS_PKG_DIRECTORY` environment variable. When set, relative file paths in tool requests will be resolved against this directory.

If neither the flag nor environment variable is provided, the server defaults to using the current working directory as the package directory.

**Examples:**

```bash
# Using command line flag
./ssis-analyzer.exe -http -pkg-dir C:\SSIS\Packages

# Using environment variable
export GOSSIS_PKG_DIRECTORY=C:\SSIS\Packages
./ssis-analyzer.exe -http

# Using both (command line takes precedence)
export GOSSIS_PKG_DIRECTORY=C:\SSIS\Packages
./ssis-analyzer.exe -http -pkg-dir C:\Other\Packages

# Using default (current working directory)
./ssis-analyzer.exe -http
```

When the package directory is set, you can reference DTSX files using relative paths:

```json
{
  "tool_name": "parse_dtsx",
  "args": {
    "file_path": "MyPackage.dtsx" // Resolves to C:\SSIS\Packages\MyPackage.dtsx
  }
}
```

If no package directory is set, absolute paths must be used.

### Configuration

To use this server with Claude Desktop, add the following to your `.vscode/mcp.json`:

```json
{
  "servers": {
    "ssis-analyzer-http": {
      "type": "http",
      "url": "http://localhost:8086/mcp"
    },
    "ssis-analyzer": {
      "type": "stdio",
      "command": "ssis-analyzer.exe"
    }
  }
}
```

This configuration provides both HTTP and stdio transport options. The HTTP transport uses the official MCP Streamable HTTP protocol for full MCP compatibility.

### Available Tools

1. **parse_dtsx**

   - Description: Parse an SSIS DTSX file and return a summary of its structure
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file to parse (relative to package directory if set, or absolute path)

2. **extract_tasks**

   - Description: Extract and list all tasks from a DTSX file, including resolved expressions in task properties
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

3. **extract_connections**

   - Description: Extract and list all connection managers from a DTSX file, including resolved expressions in connection strings
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

4. **extract_precedence_constraints**

   - Description: Extract and list all precedence constraints from a DTSX file, including resolved expressions
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

5. **extract_variables**

   - Description: Extract and list all variables from a DTSX file, including resolved expressions
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

6. **extract_parameters**

   - Description: Extract and list all parameters from a DTSX file, including data types, default values, and properties
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

7. **extract_script_code**

   - Description: Extract script code from Script Tasks in a DTSX file
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

8. **validate_best_practices**

   - Description: Check SSIS package for best practices and potential issues
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

9. **ask_about_dtsx**

   - Description: Ask questions about an SSIS DTSX file and get relevant information
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)
     - `question` (string, required): Question about the DTSX file

10. **analyze_message_queue_tasks**

    - Description: Analyze Message Queue Tasks in a DTSX file, including send/receive operations and message content
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

11. **analyze_script_task**

    - Description: Analyze Script Tasks in a DTSX file, including script code, variables, and task configuration
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

12. **detect_hardcoded_values**

    - Description: Detect hard-coded values in a DTSX file, such as embedded literals in connection strings, messages, or expressions
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

13. **analyze_logging_configuration**

    - Description: Analyze detailed logging configuration in a DTSX file, including log providers, events, and destinations
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

14. **list_packages**

    - Description: Recursively list all DTSX packages found in the package directory

- Parameters: None (uses the configured package directory)
- Notes: Create a `.gossisignore` file in the package directory to skip paths (for example `bin/` or `obj/`); blank lines and `#` comments are ignored.

15. **batch_analyze**

    - Description: Analyze multiple DTSX files in parallel and provide aggregated results
    - Parameters:
      - `file_paths` (array, required): Array of DTSX file paths to analyze (relative to package directory if set)
      - `format` (string, optional): Output format: text, json, csv, html, markdown (default: text)
      - `max_concurrent` (number, optional): Maximum number of concurrent analyses (default: 4)

16. **analyze_data_flow**

    - Description: Analyze Data Flow components in a DTSX file, including sources, transformations, destinations, and data paths
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

17. **analyze_data_flow_detailed**

    - Description: Provide detailed analysis of Data Flow components including configurations, properties, inputs/outputs, and data mappings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

18. **analyze_source**

    - Description: Analyze source components in a DTSX file by type (unified interface for all source types)
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)
      - `source_type` (string, required): Type of source to analyze: ole_db, ado_net, odbc, flat_file, excel, access, xml, raw_file, cdc, sap_bw

19. **analyze_destination**

    - Description: Analyze destination components in a DTSX file by type (unified interface for all destination types)
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)
      - `destination_type` (string, required): Type of destination to analyze: ole_db, flat_file, sql_server, excel, raw_file

20. **analyze_ole_db_source**

    - Description: Analyze OLE DB Source components in a DTSX file, extracting connection details, access mode, SQL commands, and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

21. **analyze_export_column**

    - Description: Analyze Export Column destinations in a DTSX file, extracting file data columns, file path columns, and export settings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

22. **analyze_data_conversion**

    - Description: Analyze Data Conversion transformations in a DTSX file, extracting input/output mappings and data type conversions
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

23. **analyze_ado_net_source**

    - Description: Analyze ADO.NET Source components in a DTSX file, extracting connection details and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

24. **analyze_odbc_source**

    - Description: Analyze ODBC Source components in a DTSX file, extracting connection details and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

25. **analyze_flat_file_source**

    - Description: Analyze Flat File Source components in a DTSX file, extracting file connection details and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

26. **analyze_excel_source**

    - Description: Analyze Excel Source components in a DTSX file, extracting Excel file details, sheet names, and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

27. **analyze_access_source**

    - Description: Analyze Access Source components in a DTSX file, extracting database connection details and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

28. **analyze_xml_source**

    - Description: Analyze XML Source components in a DTSX file, extracting XML structure details and output columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

29. **analyze_raw_file_source**

    - Description: Analyze Raw File Source components in a DTSX file, extracting file metadata and column structure
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

30. **analyze_cdc_source**

    - Description: Analyze CDC Source components in a DTSX file, extracting CDC configuration and change tracking details
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

31. **analyze_sap_bw_source**

    - Description: Analyze SAP BW Source components in a DTSX file, extracting SAP BW integration details and InfoObject mappings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

32. **analyze_ole_db_destination**

    - Description: Analyze OLE DB Destination components in a DTSX file, extracting target table mappings and bulk load settings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

33. **analyze_flat_file_destination**

    - Description: Analyze Flat File Destination components in a DTSX file, extracting file format settings and column mappings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

34. **analyze_sql_server_destination**

    - Description: Analyze SQL Server Destination components in a DTSX file, extracting bulk insert configuration and performance settings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

35. **analyze_derived_column**

    - Description: Analyze Derived Column components in a DTSX file, extracting expressions and data transformations
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

36. **analyze_lookup**

    - Description: Analyze Lookup components in a DTSX file, extracting reference table joins and cache configuration
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

37. **analyze_conditional_split**

    - Description: Analyze Conditional Split components in a DTSX file, extracting split conditions and output configurations
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

38. **analyze_sort**

    - Description: Analyze Sort transform components in a DTSX file, extracting sort keys and memory usage
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

39. **analyze_aggregate**

    - Description: Analyze Aggregate transform components in a DTSX file, extracting aggregation operations and group by columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

40. **analyze_merge_join**

    - Description: Analyze Merge Join transform components in a DTSX file, extracting join type and key columns
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

41. **analyze_union_all**

    - Description: Analyze Union All transform components in a DTSX file, extracting input/output column mappings
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

42. **analyze_multicast**

    - Description: Analyze Multicast transform components in a DTSX file, extracting output configurations
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

43. **analyze_script_component**

    - Description: Analyze Script Component transform components in a DTSX file, extracting script code and configuration
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

44. **analyze_excel_destination**

    - Description: Analyze Excel Destination components in a DTSX file, extracting sheet configuration and data type mapping
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

45. **analyze_raw_file_destination**

    - Description: Analyze Raw File Destination components in a DTSX file, extracting file metadata and write options
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

46. **analyze_event_handlers**

    - Description: Analyze event handlers in a DTSX file, including OnError, OnWarning, OnPreExecute, and other event types with their associated tasks, variables, and precedence constraints
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

47. **analyze_package_dependencies**

    - Description: Analyze relationships between packages, shared connections, and variables across multiple DTSX files
    - Parameters: None (analyzes all DTSX files in the package directory)

48. **analyze_configurations**

    - Description: Analyze package configurations (XML, SQL Server, environment variable configs)
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

49. **analyze_performance_metrics**

    - Description: Analyze data flow performance settings (buffer sizes, engine threads, etc.) to identify bottlenecks and optimization opportunities
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

50. **detect_security_issues**

    - Description: Detect potential security issues (hardcoded credentials, sensitive data exposure)
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

51. **compare_packages**

    - Description: Compare two DTSX files and highlight differences
    - Parameters:
      - `file_path1` (string, required): Path to the first DTSX file (relative to package directory if set, or absolute path)
      - `file_path2` (string, required): Path to the second DTSX file (relative to package directory if set, or absolute path)

52. **analyze_code_quality**

    - Description: Calculate maintainability metrics (complexity, duplication, etc.) to assess package quality and technical debt
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

53. **read_text_file**

    - Description: Read configuration or data from text files referenced by SSIS packages
    - Parameters:
      - `file_path` (string, required): Path to the text file to read (relative to package directory if set, or absolute path)

## Advanced Analysis Capabilities

The SSIS DTSX Analyzer provides specialized analysis for:

- **Data Flow Components**: Detailed analysis of sources, transformations, destinations, and data paths within Data Flow Tasks
- **Event Handlers**: Comprehensive analysis of OnError, OnWarning, OnPreExecute handlers with tasks, variables, and precedence constraints
- **Parameters**: Extraction of SSIS 2012+ project and package parameters with data types, default values, and properties
- **Package Dependencies**: Cross-package analysis of shared connections and variables to understand ETL workflow relationships
- **Configurations**: Analysis of legacy SSIS configurations (XML, SQL Server, environment variables) with migration recommendations
- **Performance Metrics**: Analysis of data flow performance settings including buffer sizes, engine threads, and optimization recommendations
- **Security Analysis**: Detection of hardcoded credentials, sensitive data exposure, and authentication vulnerabilities
- **Package Comparison**: Structural diff of tasks, connections, variables, parameters, and configurations between two DTSX files
- **Code Quality Metrics**: Analysis of script complexity, expression complexity, structural metrics, and overall maintainability scoring
- **Read Text File Integration**: Parsing and analysis of text files (.bat, .config, .sql) referenced by SSIS packages
- **Message Queue Tasks**: Send/receive operations and message content analysis
- **Logging Configuration**: Detailed log provider, event, and destination analysis
- **Script Task Code**: Full C#/VB.NET code extraction from embedded scripts
- **Hard-coded Values**: Detection of embedded literals that should be parameterized
- **Best Practices**: Comprehensive validation against SSIS development standards

## Usage Examples

### Analyzing Logging Configuration

```
"Analyze the logging configuration in this DTSX file"
→ Returns detailed log providers, events, and destinations
```

### Detecting Hard-coded Values

```
"Detect any hard-coded values in this DTSX file"
→ Identifies connection strings, paths, and literals that should be parameterized
```

### Batch Analysis

```
"Analyze multiple DTSX files in parallel"
→ Returns aggregated results with success/failure counts, performance metrics, and detailed package summaries
```

### Multiple Output Formats

```
"Generate an HTML report for this DTSX analysis"
→ Returns formatted HTML output with styling and tables
```

```
"Export analysis results as CSV"
→ Returns comma-separated values for spreadsheet analysis
```

### Performance Optimization

```
"Optimize buffer sizes for this data flow"
→ Provides specific recommendations for buffer configuration
```

```
"Analyze parallel processing capabilities"
→ Identifies optimization opportunities for concurrent execution
```

### Security Analysis

```
"Scan for hardcoded credentials in this package"
→ Advanced pattern matching for security vulnerabilities
```

```
"Check GDPR compliance"
→ Validates regulatory compliance for data protection
```

## Development

To modify or extend the server:

1. Edit `main.go` to add new tools or modify existing ones
2. Update the SSIS XML parsing structs in `ssis_types.go` as needed for more detailed analysis
3. Run `go build -o ssis-analyzer.exe .` to compile changes

## Notes

- This server provides comprehensive analysis of SSIS package elements including advanced features like data flow analysis, event handler analysis, parameter extraction, package dependency mapping, configuration analysis, logging configuration, script code extraction, performance metrics analysis, security analysis, package comparison, code quality metrics, text file integration, and specialized task analysis
- SSIS DTSX files have complex XML structures; the parsing handles namespace prefixes and various XML schemas
- Ensure the MCP client has read access to the DTSX files you want to analyze
- Supports analysis of SQL Server, Message Queue, Script, and other specialized SSIS tasks
- The HTTP transport uses the official MCP Streamable HTTP protocol for full compatibility with MCP clients
- Both stdio and HTTP transports are supported for maximum flexibility
- Use the `-pkg-dir` flag or `GOSSIS_PKG_DIRECTORY` environment variable to specify a root directory for SSIS packages, allowing relative path references in tool calls (defaults to current working directory if not specified)

## Implementation Status

All recommended missing features from the original feature request have been successfully implemented and significantly expanded, transforming this server from a basic DTSX parser into a comprehensive SSIS analysis platform. The server now includes:

- ✅ **Batch Processing**: Parallel analysis of multiple DTSX files with aggregated results and performance metrics
- ✅ **Multiple Output Formats**: Support for text, JSON, CSV, HTML, and Markdown output formats across all tools
- ✅ **Performance Optimization Tools**: Buffer size optimization, parallel processing analysis, and memory usage profiling
- ✅ **10 High/Medium Priority Features**: Data flow analysis, event handlers, parameters, dependencies, configurations, performance metrics
- ✅ **5 Lower Priority Features**: Security analysis, package comparison, code quality metrics, text file integration
- ✅ **Unified Analysis Interfaces**: Streamlined source and destination analysis with type-based dispatch
- ✅ **Expanded Component Coverage**: Analysis tools for 10 source types, 10 transform types, and 6 destination types
- ✅ **Advanced Task Analysis**: Comprehensive Script Task analysis with variables, entry points, and configuration
- 🔄 **1 Remaining Feature**: SSIS Catalog Integration (database connectivity to SSISDB for deployed package analysis)

The server has evolved from supporting basic package parsing to providing enterprise-grade SSIS development and maintenance capabilities with 80+ specialized analysis tools.

## Documentation

- [Main README](README.md) - This file with comprehensive server documentation
- [MSMQ Integration Examples](Documents/Query_EXAMPLES/MSMQ_README.md) - Detailed analysis of MSMQ message queue packages with architecture diagrams
- [SSIS Feature Recommendations](Documents/EXAMPLE_QUERY.md) - Recommended features and implementation roadmap for SSIS packages

## License

MIT License
