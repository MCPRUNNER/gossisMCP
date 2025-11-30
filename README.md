# SSIS DTSX Analyzer MCP Server

This is a comprehensive Model Context Protocol (MCP) server written in Go that provides 22 advanced tools for analyzing SSIS (SQL Server Integration Services) DTSX files. It offers detailed insights into package structure, logging configurations, task analysis, and best practices validation.

## Features

- **Package Analysis**: Parse DTSX files and extract comprehensive package information
- **Data Flow Analysis**: Analyze components within Data Flow Tasks (sources, transformations, destinations, data paths, and component properties)
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
- **Script Code Analysis**: Extract C#/VB.NET code from Script Tasks
- **Logging Configuration**: Detailed analysis of logging providers, events, and destinations
- **Best Practices Validation**: Check SSIS packages for best practices and potential issues
- **Hard-coded Value Detection**: Identify embedded literals in connection strings, messages, and expressions
- **Interactive Queries**: Ask specific questions about DTSX files and get relevant information
- **File Structure Validation**: Validate DTSX file structure and integrity
- **HTTP Streaming Support**: Optional HTTP API with streaming responses for real-time output

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

11. **detect_hardcoded_values**

    - Description: Detect hard-coded values in a DTSX file, such as embedded literals in connection strings, messages, or expressions
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

12. **analyze_logging_configuration**

    - Description: Analyze detailed logging configuration in a DTSX file, including log providers, events, and destinations
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

13. **list_packages**

    - Description: Recursively list all DTSX packages found in the package directory
    - Parameters: None (uses the configured package directory)

14. **analyze_data_flow**

    - Description: Analyze Data Flow components in a DTSX file, including sources, transformations, destinations, and data paths
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

15. **analyze_event_handlers**

    - Description: Analyze event handlers in a DTSX file, including OnError, OnWarning, OnPreExecute, and other event types with their associated tasks, variables, and precedence constraints
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

16. **analyze_package_dependencies**

    - Description: Analyze relationships between packages, shared connections, and variables across multiple DTSX files
    - Parameters: None (analyzes all DTSX files in the package directory)

17. **analyze_configurations**

    - Description: Analyze package configurations (XML, SQL Server, environment variable configs)
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

18. **analyze_performance_metrics**

    - Description: Analyze data flow performance settings (buffer sizes, engine threads, etc.) to identify bottlenecks and optimization opportunities
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

19. **detect_security_issues**

    - Description: Detect potential security issues (hardcoded credentials, sensitive data exposure)
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

20. **compare_packages**

    - Description: Compare two DTSX files and highlight differences
    - Parameters:
      - `file_path1` (string, required): Path to the first DTSX file (relative to package directory if set, or absolute path)
      - `file_path2` (string, required): Path to the second DTSX file (relative to package directory if set, or absolute path)

21. **analyze_code_quality**

    - Description: Calculate maintainability metrics (complexity, duplication, etc.) to assess package quality and technical debt
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

22. **read_text_file**

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

### Package Discovery

```
"List all DTSX packages in my workspace"
→ Returns a numbered list of all .dtsx files found recursively in the package directory
```

### Message Queue Analysis

```
"Analyze the Message Queue Tasks in this DTSX file"
→ Provides send/receive operations and message content details
```

### Parameter Extraction

```
"Extract all parameters from this DTSX file"
→ Returns parameter names, data types, default values, and properties
```

### Data Flow Analysis

```
"Analyze the data flow components in this DTSX file"
→ Provides detailed analysis of sources, transformations, destinations, and data paths
```

### Event Handler Analysis

```
"Analyze event handlers in this DTSX file"
→ Returns OnError, OnWarning handlers with their tasks, variables, and constraints
```

### Package Dependency Analysis

```
"Analyze package dependencies across all DTSX files"
→ Identifies shared connections and variables between packages
```

### Configuration Analysis

```
"Analyze configurations in this DTSX file"
→ Returns XML, SQL Server, and environment variable configurations with migration recommendations
```

### Security Analysis

```
"Detect security issues in this DTSX file"
→ Identifies hardcoded credentials, authentication vulnerabilities, and sensitive data exposure
```

### Package Comparison

```
"Compare these two DTSX files"
→ Highlights differences in tasks, connections, variables, parameters, and configurations
```

### Code Quality Analysis

```
"Analyze code quality metrics for this DTSX file"
→ Calculates maintainability scores, complexity metrics, and provides improvement recommendations
```

### Text File Analysis

```
"Read and analyze this configuration file"
→ Parses text files (.bat, .config, .sql) and extracts relevant configuration data
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

## License

MIT License
