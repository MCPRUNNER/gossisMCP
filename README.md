# SSIS DTSX Analyzer MCP Server

This is a comprehensive Model Context Protocol (MCP) server written in Go that provides advanced tools for analyzing SSIS (SQL Server Integration Services) DTSX files. It offers detailed insights into package structure, logging configurations, task analysis, and best practices validation.

## Features

- **Package Analysis**: Parse DTSX files and extract comprehensive package information
- **Task Analysis**: List and analyze all tasks within packages, including specialized tasks like Message Queue Tasks
- **Connection Management**: Extract and analyze connection manager details
- **Variable Extraction**: List all package and task variables
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
go build -o ssis-analyzer.exe main.go
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

   - Description: Extract and list all tasks from a DTSX file
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

3. **extract_connections**

   - Description: Extract and list all connection managers from a DTSX file
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

4. **extract_variables**

   - Description: Extract and list all variables from a DTSX file
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

5. **extract_script_code**

   - Description: Extract script code from Script Tasks in a DTSX file
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

6. **validate_best_practices**

   - Description: Check SSIS package for best practices and potential issues
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

7. **ask_about_dtsx**

   - Description: Ask questions about an SSIS DTSX file and get relevant information
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)
     - `question` (string, required): Question about the DTSX file

8. **analyze_message_queue_tasks**

   - Description: Analyze Message Queue Tasks in a DTSX file, including send/receive operations and message content
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

9. **detect_hardcoded_values**

   - Description: Detect hard-coded values in a DTSX file, such as embedded literals in connection strings, messages, or expressions
   - Parameters:
     - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

10. **analyze_logging_configuration**

    - Description: Analyze detailed logging configuration in a DTSX file, including log providers, events, and destinations
    - Parameters:
      - `file_path` (string, required): Path to the DTSX file (relative to package directory if set, or absolute path)

11. **list_packages**

    - Description: Recursively list all DTSX packages found in the package directory
    - Parameters: None (uses the configured package directory)

## Advanced Analysis Capabilities

The SSIS DTSX Analyzer provides specialized analysis for:

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

## Development

To modify or extend the server:

1. Edit `main.go` to add new tools or modify existing ones
2. Update the SSIS XML parsing structs as needed for more detailed analysis
3. Run `go build` to compile changes

## Notes

- This server provides comprehensive analysis of SSIS package elements including advanced features like logging configuration, script code extraction, and specialized task analysis
- SSIS DTSX files have complex XML structures; the parsing handles namespace prefixes and various XML schemas
- Ensure the MCP client has read access to the DTSX files you want to analyze
- Supports analysis of SQL Server, Message Queue, Script, and other specialized SSIS tasks
- The HTTP transport uses the official MCP Streamable HTTP protocol for full compatibility with MCP clients
- Both stdio and HTTP transports are supported for maximum flexibility
- Use the `-pkg-dir` flag or `GOSSIS_PKG_DIRECTORY` environment variable to specify a root directory for SSIS packages, allowing relative path references in tool calls (defaults to current working directory if not specified)

## License

MIT License
