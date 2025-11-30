package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// resolveVariableExpressions resolves SSIS variable expressions by substituting variable references
func resolveVariableExpressions(value string, variables []Variable, maxDepth int) string {
	if maxDepth <= 0 {
		return value // Prevent infinite recursion
	}

	// Find all @[...] expressions in the value
	re := regexp.MustCompile(`@\[([^]]+)\]`)
	result := re.ReplaceAllStringFunc(value, func(match string) string {
		// Extract the variable reference (remove @[ and ])
		varRef := match[2 : len(match)-1]

		// Handle User:: and System:: prefixes
		var resolved string
		if strings.HasPrefix(varRef, "User::") {
			varName := strings.TrimPrefix(varRef, "User::")
			resolved = findVariableValue(varName, variables)
		} else if strings.HasPrefix(varRef, "System::") {
			// For system variables, we can't resolve actual runtime values,
			// but we can indicate what type of system variable it is
			resolved = "<System variable: " + varRef + ">"
		} else {
			// Try to find as a user variable without prefix
			resolved = findVariableValue(varRef, variables)
		}

		if resolved == "" {
			return match // Return original if not found
		}

		// Recursively resolve any expressions in the resolved value
		return resolveVariableExpressions(resolved, variables, maxDepth-1)
	})

	return result
}

// findVariableValue finds a variable value by name
func findVariableValue(name string, variables []Variable) string {
	for _, v := range variables {
		if v.Name == name {
			return v.Value
		}
	}
	return ""
}

// resolveFilePath resolves a file path against the package directory if it's relative
func resolveFilePath(filePath, packageDirectory string) string {
	if packageDirectory == "" || filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(packageDirectory, filePath)
}

func main() {
	// Command line flags
	httpMode := flag.Bool("http", false, "Run in HTTP streaming mode")
	httpPort := flag.String("port", "8086", "HTTP server port")
	pkgDir := flag.String("pkg-dir", "", "Root directory for SSIS packages (can also be set via GOSSIS_PKG_DIRECTORY env var, defaults to current working directory)")
	flag.Parse()

	// Determine package directory from flag or environment variable
	packageDirectory := *pkgDir
	if packageDirectory == "" {
		packageDirectory = os.Getenv("GOSSIS_PKG_DIRECTORY")
	}
	if packageDirectory == "" {
		// Default to current working directory if neither flag nor env var is set
		if cwd, err := os.Getwd(); err == nil {
			packageDirectory = cwd
		}
	}
	if packageDirectory != "" {
		// Convert to absolute path
		absPath, err := filepath.Abs(packageDirectory)
		if err == nil {
			packageDirectory = absPath
		}
		log.Printf("Using SSIS package directory: %s", packageDirectory)
	}

	s := server.NewMCPServer(
		"SSIS DTSX Analyzer",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	// Register all tools...
	// Tool to parse DTSX file and return summary
	parseTool := mcp.NewTool("parse_dtsx",
		mcp.WithDescription("Parse an SSIS DTSX file and return a summary of its structure"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file to parse (relative to package directory if set)"),
		),
	)
	s.AddTool(parseTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleParseDtsx(ctx, request, packageDirectory)
	})

	// Tool to extract tasks
	extractTasksTool := mcp.NewTool("extract_tasks",
		mcp.WithDescription("Extract and list all tasks from a DTSX file, including resolved expressions in task properties"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractTasksTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractTasks(ctx, request, packageDirectory)
	})

	// Tool to extract connections
	extractConnectionsTool := mcp.NewTool("extract_connections",
		mcp.WithDescription("Extract and list all connection managers from a DTSX file, including resolved expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractConnectionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractConnections(ctx, request, packageDirectory)
	})

	// Tool to extract precedence constraints
	extractPrecedenceTool := mcp.NewTool("extract_precedence_constraints",
		mcp.WithDescription("Extract and list all precedence constraints from a DTSX file, including resolved expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractPrecedenceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractPrecedenceConstraints(ctx, request, packageDirectory)
	})

	// Tool to extract variables
	extractVariablesTool := mcp.NewTool("extract_variables",
		mcp.WithDescription("Extract and list all variables from a DTSX file, including resolved expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractVariablesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractVariables(ctx, request, packageDirectory)
	})

	// Tool to extract parameters
	extractParametersTool := mcp.NewTool("extract_parameters",
		mcp.WithDescription("Extract and list all parameters from a DTSX file, including data types, default values, and properties"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractParametersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractParameters(ctx, request, packageDirectory)
	})

	// Tool to extract script code from Script Tasks
	extractScriptTool := mcp.NewTool("extract_script_code",
		mcp.WithDescription("Extract script code from Script Tasks in a DTSX file"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractScriptTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractScriptCode(ctx, request, packageDirectory)
	})

	// Tool to validate best practices
	validateBestPracticesTool := mcp.NewTool("validate_best_practices",
		mcp.WithDescription("Check SSIS package for best practices and potential issues"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(validateBestPracticesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleValidateBestPractices(ctx, request, packageDirectory)
	})

	// Tool to ask questions about DTSX file
	askTool := mcp.NewTool("ask_about_dtsx",
		mcp.WithDescription("Ask questions about an SSIS DTSX file and get relevant information"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("question",
			mcp.Required(),
			mcp.Description("Question about the DTSX file"),
		),
	)
	s.AddTool(askTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAskAboutDtsx(ctx, request, packageDirectory)
	})

	// Tool to analyze Message Queue Tasks
	analyzeMessageQueueTool := mcp.NewTool("analyze_message_queue_tasks",
		mcp.WithDescription("Analyze Message Queue Tasks in a DTSX file, including send/receive operations and message content"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeMessageQueueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeMessageQueueTasks(ctx, request, packageDirectory)
	})

	// Tool to detect hard-coded values
	detectHardcodedValuesTool := mcp.NewTool("detect_hardcoded_values",
		mcp.WithDescription("Detect hard-coded values in a DTSX file, such as embedded literals in connection strings, messages, or expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(detectHardcodedValuesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDetectHardcodedValues(ctx, request, packageDirectory)
	})

	// Tool to analyze logging configuration
	analyzeLoggingTool := mcp.NewTool("analyze_logging_configuration",
		mcp.WithDescription("Analyze detailed logging configuration in a DTSX file, including log providers, events, and destinations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeLoggingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeLoggingConfiguration(ctx, request, packageDirectory)
	})

	// Tool to list all DTSX packages in the package directory
	listPackagesTool := mcp.NewTool("list_packages",
		mcp.WithDescription("Recursively list all DTSX packages found in the package directory"),
	)
	s.AddTool(listPackagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListPackages(ctx, request, packageDirectory)
	})

	// Tool to analyze data flow components
	analyzeDataFlowTool := mcp.NewTool("analyze_data_flow",
		mcp.WithDescription("Analyze Data Flow components in a DTSX file, including sources, transformations, destinations, and data paths"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDataFlowTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeDataFlow(ctx, request, packageDirectory)
	})

	// Tool to analyze event handlers
	analyzeEventHandlersTool := mcp.NewTool("analyze_event_handlers",
		mcp.WithDescription("Analyze event handlers in a DTSX file, including OnError, OnWarning, and other event types"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeEventHandlersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeEventHandlers(ctx, request, packageDirectory)
	})

	// Tool to analyze package dependencies
	analyzePackageDependenciesTool := mcp.NewTool("analyze_package_dependencies",
		mcp.WithDescription("Analyze relationships between packages, shared connections, and variables across multiple DTSX files"),
	)
	s.AddTool(analyzePackageDependenciesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzePackageDependencies(ctx, request, packageDirectory)
	})

	// Tool to analyze package configurations
	analyzeConfigurationsTool := mcp.NewTool("analyze_configurations",
		mcp.WithDescription("Analyze package configurations (XML, SQL Server, environment variable configs)"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeConfigurationsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeConfigurations(ctx, request, packageDirectory)
	})

	// Tool to analyze performance metrics
	analyzePerformanceTool := mcp.NewTool("analyze_performance_metrics",
		mcp.WithDescription("Analyze data flow performance settings (buffer sizes, engine threads, etc.) to identify bottlenecks and optimization opportunities"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzePerformanceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzePerformanceMetrics(ctx, request, packageDirectory)
	})

	if *httpMode {
		// Run in HTTP streaming mode
		runHTTPServer(s, *httpPort)
	} else {
		// Run in stdio mode (default)
		if err := server.ServeStdio(s); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}
}

func handleParseDtsx(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	summary := "Package Summary:\n"
	summary += fmt.Sprintf("- Properties: %d\n", len(pkg.Properties))
	summary += fmt.Sprintf("- Connections: %d\n", len(pkg.ConnectionMgr.Connections))
	summary += fmt.Sprintf("- Tasks: %d\n", len(pkg.Executables.Tasks))
	summary += fmt.Sprintf("- Variables: %d\n", len(pkg.Variables.Vars))

	for _, prop := range pkg.Properties {
		if prop.Name == "Name" || prop.Name == "Description" {
			summary += fmt.Sprintf("  %s: %s\n", prop.Name, strings.TrimSpace(prop.Value))
		}
	}

	for _, v := range pkg.Variables.Vars {
		summary += fmt.Sprintf("  Variable %s: %s\n", v.Name, v.Value)
	}

	return mcp.NewToolResultText(summary), nil
}

func handleExtractTasks(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	tasks := "Tasks:\n"
	for i, task := range pkg.Executables.Tasks {
		tasks += fmt.Sprintf("%d. %s\n", i+1, task.Name)
		for _, prop := range task.Properties {
			if prop.Name == "Description" {
				tasks += fmt.Sprintf("   Description: %s\n", strings.TrimSpace(prop.Value))
			} else if prop.Name == "SqlStatementSource" || prop.Name == "Executable" || prop.Name == "Arguments" {
				// Show important task properties that might contain expressions
				propValue := strings.TrimSpace(prop.Value)
				tasks += fmt.Sprintf("   %s: %s\n", prop.Name, propValue)

				// Check if property value contains expressions
				if strings.Contains(propValue, "@[") {
					resolvedValue := resolveVariableExpressions(propValue, pkg.Variables.Vars, 10)
					if resolvedValue != propValue {
						tasks += fmt.Sprintf("   Resolved %s: %s\n", prop.Name, resolvedValue)
					}
				}
			}
		}
	}

	return mcp.NewToolResultText(tasks), nil
}

func handleExtractConnections(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	connections := "Connections:\n"
	for i, conn := range pkg.ConnectionMgr.Connections {
		connections += fmt.Sprintf("%d. %s\n", i+1, conn.Name)
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		connections += fmt.Sprintf("   Connection String: %s\n", connStr)

		// Check if connection string contains expressions and resolve them
		if strings.Contains(connStr, "@[") {
			resolvedConnStr := resolveVariableExpressions(connStr, pkg.Variables.Vars, 10)
			if resolvedConnStr != connStr {
				connections += fmt.Sprintf("   Resolved Connection String: %s\n", resolvedConnStr)
			}
		}
	}

	return mcp.NewToolResultText(connections), nil
}

func handleExtractPrecedenceConstraints(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	constraints := "Precedence Constraints:\n"
	for i, constraint := range pkg.PrecedenceConstraints.Constraints {
		constraints += fmt.Sprintf("%d. %s\n", i+1, constraint.Name)
		constraints += fmt.Sprintf("   From: %s\n", constraint.From)
		constraints += fmt.Sprintf("   To: %s\n", constraint.To)
		constraints += fmt.Sprintf("   Evaluation Operation: %s\n", constraint.EvalOp)

		if constraint.Expression != "" {
			constraints += fmt.Sprintf("   Expression: %s\n", constraint.Expression)

			// Resolve expressions in the constraint
			if strings.Contains(constraint.Expression, "@[") {
				resolvedExpr := resolveVariableExpressions(constraint.Expression, pkg.Variables.Vars, 10)
				if resolvedExpr != constraint.Expression {
					constraints += fmt.Sprintf("   Resolved Expression: %s\n", resolvedExpr)
				}
			}
		}
		constraints += "\n"
	}

	return mcp.NewToolResultText(constraints), nil
}

func handleValidateDtsx(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid DTSX structure: %v", err)), nil
	}

	// Basic validation
	if len(pkg.Properties) == 0 {
		return mcp.NewToolResultText("Validation: Warning - No properties found"), nil
	}

	return mcp.NewToolResultText("Validation: DTSX file structure is valid"), nil
}

func handleExtractVariables(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	variables := "Variables:\n"
	for i, v := range pkg.Variables.Vars {
		variables += fmt.Sprintf("%d. %s\n", i+1, v.Name)

		// Show expression if it exists
		if v.Expression != "" {
			variables += fmt.Sprintf("   Expression: %s\n", v.Expression)
		}

		variables += fmt.Sprintf("   Raw: %s\n", v.Value)

		// Try to resolve expressions in the value
		resolvedValue := resolveVariableExpressions(v.Value, pkg.Variables.Vars, 10)
		if resolvedValue != v.Value {
			variables += fmt.Sprintf("   Resolved: %s\n", resolvedValue)
		}

		variables += "\n"
	}

	return mcp.NewToolResultText(variables), nil
}

func handleExtractParameters(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Parameters:\n")

	if len(pkg.Parameters.Params) == 0 {
		result.WriteString("No parameters found in this package.\n")
		return mcp.NewToolResultText(result.String()), nil
	}

	for i, p := range pkg.Parameters.Params {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, p.Name))
		result.WriteString(fmt.Sprintf("   Data Type: %s\n", p.DataType))
		result.WriteString(fmt.Sprintf("   Value: %s\n", p.Value))
		if p.Description != "" {
			result.WriteString(fmt.Sprintf("   Description: %s\n", p.Description))
		}
		result.WriteString(fmt.Sprintf("   Required: %t\n", p.Required))
		result.WriteString(fmt.Sprintf("   Sensitive: %t\n", p.Sensitive))
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleExtractScriptCode(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	scriptCode := "Script Tasks Code:\n"
	found := false

	for i, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "script") {
			found = true
			scriptCode += fmt.Sprintf("Script Task %d: %s\n", i+1, task.Name)

			// Extract script code from ScriptProject
			code := task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode
			if code != "" {
				// Clean up the XML formatting
				code = strings.TrimSpace(code)
				code = strings.ReplaceAll(code, "&lt;", "<")
				code = strings.ReplaceAll(code, "&gt;", ">")
				code = strings.ReplaceAll(code, "&amp;", "&")
				scriptCode += fmt.Sprintf("Code:\n%s\n", code)
			} else {
				scriptCode += "No script code found in this task.\n"
			}
		}
	}

	if !found {
		scriptCode += "No Script Tasks found in this package.\n"
	}

	return mcp.NewToolResultText(scriptCode), nil
}

func handleValidateBestPractices(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	report := "Best Practices Validation Report:\n"

	// Check for variables
	if len(pkg.Variables.Vars) == 0 {
		report += "- WARNING: No user-defined variables found\n"
	} else {
		report += fmt.Sprintf("- OK: %d variables defined\n", len(pkg.Variables.Vars))
	}

	// Check for connections
	if len(pkg.ConnectionMgr.Connections) == 0 {
		report += "- WARNING: No connection managers defined\n"
	} else {
		report += fmt.Sprintf("- OK: %d connection managers defined\n", len(pkg.ConnectionMgr.Connections))
	}

	// Check for tasks
	if len(pkg.Executables.Tasks) == 0 {
		report += "- ERROR: No executable tasks found\n"
	} else {
		report += fmt.Sprintf("- OK: %d tasks defined\n", len(pkg.Executables.Tasks))
	}

	// Check for logging (basic)
	if strings.Contains(string(data), "LoggingOptions") {
		report += "- OK: Logging appears to be configured\n"
	} else {
		report += "- WARNING: No logging configuration found\n"
	}

	report += "- Note: This is a basic validation. Consider using SSIS best practices guidelines for comprehensive analysis.\n"

	return mcp.NewToolResultText(report), nil
}

func handleAskAboutDtsx(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	question, err := request.RequireString("question")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	questionLower := strings.ToLower(question)

	// Simple keyword-based answering
	if strings.Contains(questionLower, "task") || strings.Contains(questionLower, "executables") {
		tasks := "Tasks:\n"
		for i, task := range pkg.Executables.Tasks {
			tasks += fmt.Sprintf("%d. %s\n", i+1, task.Name)
			for _, prop := range task.Properties {
				if prop.Name == "Description" {
					tasks += fmt.Sprintf("   Description: %s\n", strings.TrimSpace(prop.Value))
				}
			}
		}
		return mcp.NewToolResultText(tasks), nil
	} else if strings.Contains(questionLower, "connection") {
		connections := "Connections:\n"
		for i, conn := range pkg.ConnectionMgr.Connections {
			connections += fmt.Sprintf("%d. %s\n", i+1, conn.Name)
			connStr := conn.ObjectData.ConnectionMgr.ConnectionString
			if connStr == "" {
				connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
			}
			connections += fmt.Sprintf("   Connection String: %s\n", connStr)
		}
		return mcp.NewToolResultText(connections), nil
	} else if strings.Contains(questionLower, "variable") {
		variables := "Variables:\n"
		for i, v := range pkg.Variables.Vars {
			variables += fmt.Sprintf("%d. %s = %s\n", i+1, v.Name, v.Value)
		}
		return mcp.NewToolResultText(variables), nil
	} else if strings.Contains(questionLower, "validate") || strings.Contains(questionLower, "valid") {
		if len(pkg.Properties) == 0 {
			return mcp.NewToolResultText("Validation: Warning - No properties found"), nil
		}
		return mcp.NewToolResultText("Validation: DTSX file structure is valid"), nil
	} else {
		// Default to summary
		summary := "Package Summary:\n"
		summary += fmt.Sprintf("- Properties: %d\n", len(pkg.Properties))
		summary += fmt.Sprintf("- Connections: %d\n", len(pkg.ConnectionMgr.Connections))
		summary += fmt.Sprintf("- Tasks: %d\n", len(pkg.Executables.Tasks))
		summary += fmt.Sprintf("- Variables: %d\n", len(pkg.Variables.Vars))

		for _, prop := range pkg.Properties {
			if prop.Name == "Name" || prop.Name == "Description" {
				summary += fmt.Sprintf("  %s: %s\n", prop.Name, strings.TrimSpace(prop.Value))
			}
		}

		for _, v := range pkg.Variables.Vars {
			summary += fmt.Sprintf("  Variable %s: %s\n", v.Name, v.Value)
		}
		return mcp.NewToolResultText(summary), nil
	}
}

func handleAnalyzeMessageQueueTasks(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	analysis := "Message Queue Tasks Analysis:\n"
	found := false

	for i, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "message queue") {
			found = true
			analysis += fmt.Sprintf("Task %d: %s\n", i+1, task.Name)

			// Check message type and content
			messageType := task.ObjectData.Task.MessageQueueTask.MessageQueueTaskData.MessageType
			message := task.ObjectData.Task.MessageQueueTask.MessageQueueTaskData.Message

			analysis += fmt.Sprintf("  Message Type: %s\n", messageType)
			analysis += fmt.Sprintf("  Message Content: %s\n", message)

			// Additional properties
			for _, prop := range task.Properties {
				if prop.Name == "Description" {
					analysis += fmt.Sprintf("  Description: %s\n", strings.TrimSpace(prop.Value))
				}
			}
		}
	}

	if !found {
		analysis += "No Message Queue Tasks found in this package.\n"
	}

	return mcp.NewToolResultText(analysis), nil
}

func handleDetectHardcodedValues(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	report := "Hard-coded Values Detection Report:\n"
	found := false

	// Check connection strings for hard-coded values
	for _, conn := range pkg.ConnectionMgr.Connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		if strings.Contains(connStr, "localhost") || strings.Contains(connStr, "127.0.0.1") || strings.Contains(connStr, "hardcoded") {
			report += fmt.Sprintf("- WARNING: Connection '%s' contains hard-coded value: %s\n", conn.Name, connStr)
			found = true
		}
	}

	// Check variables for hard-coded values
	for _, v := range pkg.Variables.Vars {
		if strings.Contains(strings.ToLower(v.Value), "c:\\") || strings.Contains(v.Value, "localhost") {
			report += fmt.Sprintf("- WARNING: Variable '%s' contains hard-coded path/value: %s\n", v.Name, v.Value)
			found = true
		}
	}

	// Check tasks for hard-coded messages or expressions
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "message queue") {
			message := task.ObjectData.Task.MessageQueueTask.MessageQueueTaskData.Message
			if message != "" && !strings.Contains(message, "@[") { // @[ indicates expression
				report += fmt.Sprintf("- WARNING: Message Queue Task '%s' contains hard-coded message: %s\n", task.Name, message)
				found = true
			}
		}
		// Check properties for hard-coded values
		for _, prop := range task.Properties {
			if strings.Contains(prop.Value, "localhost") || strings.Contains(prop.Value, "127.0.0.1") {
				report += fmt.Sprintf("- WARNING: Task '%s' property '%s' contains hard-coded value: %s\n", task.Name, prop.Name, prop.Value)
				found = true
			}
		}
	}

	if !found {
		report += "No obvious hard-coded values detected. Note: This is a basic check and may not catch all cases.\n"
	}

	return mcp.NewToolResultText(report), nil
}

func handleAnalyzeLoggingConfiguration(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	report := "Logging Configuration Analysis:\n"

	// Check if logging is configured at package level
	if !strings.Contains(string(data), "LoggingOptions") {
		report += "❌ No logging configuration found in this package.\n"
		return mcp.NewToolResultText(report), nil
	}

	report += "✅ Logging is configured in this package.\n\n"

	// Extract log providers
	logProvidersSection := extractSection(string(data), "<LogProviders>", "</LogProviders>")
	if logProvidersSection != "" {
		report += "📋 Log Providers:\n"
		// Simple string extraction for log providers
		if strings.Contains(logProvidersSection, `CreationName="Microsoft.LogProviderSQLServer"`) {
			report += "  1. Type: SQL Server Log Provider\n"
			// Extract connection string
			connMatch := regexp.MustCompile(`ConfigString="([^"]*)"`)
			if matches := connMatch.FindStringSubmatch(logProvidersSection); len(matches) > 1 {
				report += fmt.Sprintf("     Connection: %s\n", matches[1])
			}
			report += "     Description: Writes log entries for events to a SQL Server database\n"
		} else if strings.Contains(logProvidersSection, `CreationName="Microsoft.LogProviderTextFile"`) {
			report += "  1. Type: Text File Log Provider\n"
			connMatch := regexp.MustCompile(`ConfigString="([^"]*)"`)
			if matches := connMatch.FindStringSubmatch(logProvidersSection); len(matches) > 1 {
				report += fmt.Sprintf("     File Path: %s\n", matches[1])
			}
		} else if strings.Contains(logProvidersSection, `CreationName="Microsoft.LogProviderEventLog"`) {
			report += "  1. Type: Windows Event Log Provider\n"
			report += "     Target: Windows Event Log\n"
		}
		report += "\n"
	}

	// Extract package-level logging options
	loggingOptionsSection := extractSection(string(data), "<LoggingOptions", "</LoggingOptions>")
	if loggingOptionsSection != "" {
		report += "⚙️ Package-Level Logging Settings:\n"

		// Extract logging mode
		if strings.Contains(loggingOptionsSection, `LoggingMode="1"`) {
			report += "  • Logging Mode: Enabled\n"
		} else {
			report += "  • Logging Mode: Disabled\n"
		}

		// Extract event filters
		eventFilterMatch := regexp.MustCompile(`EventFilter">([^<]+)</`)
		if matches := eventFilterMatch.FindStringSubmatch(loggingOptionsSection); len(matches) > 1 {
			report += fmt.Sprintf("  • Events Logged: %s\n", matches[1])
		}

		// Extract selected log providers
		selectedProvidersMatch := regexp.MustCompile(`SelectedLogProvider[^}]*InstanceID="([^"]*)"`)
		if matches := selectedProvidersMatch.FindAllStringSubmatch(loggingOptionsSection, -1); len(matches) > 0 {
			report += "  • Selected Providers:\n"
			for _, match := range matches {
				if len(match) > 1 {
					report += fmt.Sprintf("    - %s\n", match[1])
				}
			}
		}

		report += "\n"
	}

	// Check for task-level logging overrides
	taskLoggingCount := strings.Count(string(data), `<LoggingOptions`)
	if taskLoggingCount > 1 {
		report += "🔧 Task-Level Logging Overrides:\n"
		report += fmt.Sprintf("  • %d tasks have custom logging settings\n", taskLoggingCount-1)
		report += "  • Tasks inherit package-level logging unless explicitly overridden\n\n"
	}

	// Provide recommendations
	report += "💡 Recommendations:\n"
	if strings.Contains(string(data), `LoggingMode="1"`) {
		report += "  • ✅ Logging is properly enabled\n"
		if strings.Contains(string(data), "OnError") {
			report += "  • ✅ Error logging is configured\n"
		}
		if strings.Contains(string(data), "Microsoft.LogProviderSQLServer") {
			report += "  • ✅ SQL Server logging provides good audit trail\n"
		}
	} else {
		report += "  • ⚠️ Consider enabling logging for better monitoring and troubleshooting\n"
	}

	return mcp.NewToolResultText(report), nil
}

// Helper function to extract XML sections
func extractSection(content, startTag, endTag string) string {
	start := strings.Index(content, startTag)
	if start == -1 {
		return ""
	}
	end := strings.Index(content[start:], endTag)
	if end == -1 {
		return ""
	}
	return content[start : start+end+len(endTag)]
}

func handleListPackages(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	var packages []string

	// Walk the package directory recursively to find .dtsx files
	err := filepath.Walk(packageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".dtsx" {
			// Get relative path from package directory
			relPath, err := filepath.Rel(packageDirectory, path)
			if err != nil {
				relPath = path // fallback to absolute path if relative fails
			}
			packages = append(packages, relPath)
		}
		return nil
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to scan directory: %v", err)), nil
	}

	if len(packages) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No DTSX files found in package directory: %s", packageDirectory)), nil
	}

	result := fmt.Sprintf("Found %d DTSX package(s) in directory: %s\n\n", len(packages), packageDirectory)
	for i, pkg := range packages {
		result += fmt.Sprintf("%d. %s\n", i+1, pkg)
	}

	return mcp.NewToolResultText(result), nil
}

func handleAnalyzeDataFlow(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	fullPath := resolveFilePath(filePath, packageDirectory)

	// Read the DTSX file as string for analysis
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	xmlContent := string(data)

	var result strings.Builder
	result.WriteString("Data Flow Analysis:\n\n")

	// Check if this package contains data flow tasks
	if !strings.Contains(xmlContent, "Microsoft.Pipeline") {
		result.WriteString("No Data Flow Tasks found in this package.\n")
		return mcp.NewToolResultText(result.String()), nil
	}

	result.WriteString("Data Flow Task found in package.\n\n")

	// Extract component information using regex
	// Find all component definitions
	componentRegex := regexp.MustCompile(`(?s)<component[^>]*>`)
	matches := componentRegex.FindAllString(xmlContent, -1)

	if len(matches) > 0 {
		result.WriteString("Components:\n")
		for _, componentTag := range matches {
			// Extract individual attributes
			nameRegex := regexp.MustCompile(`name="([^"]*)"`)
			classIDRegex := regexp.MustCompile(`componentClassID="([^"]*)"`)
			descRegex := regexp.MustCompile(`description="([^"]*)"`)

			nameMatch := nameRegex.FindStringSubmatch(componentTag)
			classIDMatch := classIDRegex.FindStringSubmatch(componentTag)
			descMatch := descRegex.FindStringSubmatch(componentTag)

			name := ""
			if len(nameMatch) > 1 {
				name = nameMatch[1]
			}

			classID := ""
			if len(classIDMatch) > 1 {
				classID = classIDMatch[1]
			}

			description := ""
			if len(descMatch) > 1 {
				description = descMatch[1]
			}

			componentType := getComponentType(classID)

			result.WriteString(fmt.Sprintf("  - %s (%s)\n", name, componentType))
			if description != "" {
				result.WriteString(fmt.Sprintf("    Description: %s\n", description))
			}

			// Try to find key properties for this component
			// Look for properties within this component's section
			componentStart := strings.Index(xmlContent, fmt.Sprintf(`name="%s"`, name))
			if componentStart > 0 {
				componentEnd := strings.Index(xmlContent[componentStart:], "</component>")
				if componentEnd > 0 {
					componentSection := xmlContent[componentStart : componentStart+componentEnd]

					// Look for key properties
					keyProps := []string{"SqlCommand", "TableOrViewName", "FileName", "ConnectionString"}
					for _, prop := range keyProps {
						propRegex := regexp.MustCompile(fmt.Sprintf(`<property[^>]*name="%s"[^>]*>([^<]*)</property>`, prop))
						propMatch := propRegex.FindStringSubmatch(componentSection)
						if len(propMatch) > 1 && strings.TrimSpace(propMatch[1]) != "" {
							result.WriteString(fmt.Sprintf("    %s: %s\n", prop, strings.TrimSpace(propMatch[1])))
						}
					}
				}
			}
			result.WriteString("\n")
		}
	} else {
		result.WriteString("No components found in data flow.\n")
	} // Extract path information
	pathRegex := regexp.MustCompile(`(?s)<path[^>]*>`)
	pathMatches := pathRegex.FindAllString(xmlContent, -1)

	if len(pathMatches) > 0 {
		result.WriteString("Data Paths:\n")
		for _, pathTag := range pathMatches {
			// Extract individual attributes
			refIDRegex := regexp.MustCompile(`refId="[^"]*\.Paths\[([^]]+)\]"`)
			startIDRegex := regexp.MustCompile(`startId="([^"]*)"`)
			endIDRegex := regexp.MustCompile(`endId="([^"]*)"`)

			refIDMatch := refIDRegex.FindStringSubmatch(pathTag)
			startIDMatch := startIDRegex.FindStringSubmatch(pathTag)
			endIDMatch := endIDRegex.FindStringSubmatch(pathTag)

			pathName := ""
			if len(refIDMatch) > 1 {
				pathName = refIDMatch[1]
			}

			startID := ""
			if len(startIDMatch) > 1 {
				// Extract just the component name from the full ID
				startFull := startIDMatch[1]
				if idx := strings.LastIndex(startFull, "\\"); idx > 0 {
					startID = startFull[idx+1:]
				} else {
					startID = startFull
				}
			}

			endID := ""
			if len(endIDMatch) > 1 {
				// Extract just the component name from the full ID
				endFull := endIDMatch[1]
				if idx := strings.LastIndex(endFull, "\\"); idx > 0 {
					endID = endFull[idx+1:]
				} else {
					endID = endFull
				}
			}

			result.WriteString(fmt.Sprintf("  - %s: %s → %s\n", pathName, startID, endID))
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeEventHandlers(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	fullPath := resolveFilePath(filePath, packageDirectory)

	// Read and parse the DTSX file
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Event Handler Analysis:\n\n")

	if len(pkg.EventHandlers.EventHandlers) == 0 {
		result.WriteString("No event handlers found in this package.\n")
		return mcp.NewToolResultText(result.String()), nil
	}

	result.WriteString(fmt.Sprintf("Found %d event handler(s):\n\n", len(pkg.EventHandlers.EventHandlers)))

	for i, eh := range pkg.EventHandlers.EventHandlers {
		result.WriteString(fmt.Sprintf("%d. Event Handler: %s\n", i+1, eh.ObjectName))
		result.WriteString(fmt.Sprintf("   Type: %s\n", eh.EventHandlerType))
		result.WriteString(fmt.Sprintf("   Container: %s\n", eh.ContainerID))

		// Analyze tasks in the event handler
		if len(eh.Executables.Tasks) > 0 {
			result.WriteString(fmt.Sprintf("   Tasks (%d):\n", len(eh.Executables.Tasks)))
			for _, task := range eh.Executables.Tasks {
				taskType := getTaskType(task)
				result.WriteString(fmt.Sprintf("     - %s (%s)\n", task.Name, taskType))

				// Show key properties
				for _, prop := range task.Properties {
					if isKeyProperty(prop.Name) {
						result.WriteString(fmt.Sprintf("       %s: %s\n", prop.Name, prop.Value))
					}
				}
			}
		} else {
			result.WriteString("   Tasks: None\n")
		}

		// Analyze variables in the event handler
		if len(eh.Variables.Vars) > 0 {
			result.WriteString(fmt.Sprintf("   Variables (%d):\n", len(eh.Variables.Vars)))
			for _, variable := range eh.Variables.Vars {
				resolvedValue := resolveVariableExpressions(variable.Value, pkg.Variables.Vars, 10)
				result.WriteString(fmt.Sprintf("     - %s: %s\n", variable.Name, resolvedValue))
				if variable.Expression != "" {
					result.WriteString(fmt.Sprintf("       Expression: %s\n", variable.Expression))
				}
			}
		}

		// Analyze precedence constraints in the event handler
		if len(eh.PrecedenceConstraints.Constraints) > 0 {
			result.WriteString(fmt.Sprintf("   Precedence Constraints (%d):\n", len(eh.PrecedenceConstraints.Constraints)))
			for _, constraint := range eh.PrecedenceConstraints.Constraints {
				result.WriteString(fmt.Sprintf("     - %s → %s", constraint.From, constraint.To))
				if constraint.Expression != "" {
					resolvedExpr := resolveVariableExpressions(constraint.Expression, append(pkg.Variables.Vars, eh.Variables.Vars...), 10)
					result.WriteString(fmt.Sprintf(" (Expression: %s)", resolvedExpr))
				}
				result.WriteString("\n")
			}
		}

		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzePackageDependencies(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	// Find all DTSX files in the package directory
	var dtsxFiles []string
	err := filepath.Walk(packageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".dtsx") {
			dtsxFiles = append(dtsxFiles, path)
		}
		return nil
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to scan directory: %v", err)), nil
	}

	if len(dtsxFiles) == 0 {
		return mcp.NewToolResultText("No DTSX files found in the package directory."), nil
	}

	// Data structures to track dependencies
	type ConnectionInfo struct {
		Name             string
		ConnectionString string
		Packages         []string
	}

	type VariableInfo struct {
		Name     string
		Value    string
		Packages []string
	}

	connections := make(map[string]*ConnectionInfo)
	variables := make(map[string]*VariableInfo)

	// Process each DTSX file
	for _, filePath := range dtsxFiles {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Remove namespace prefixes for easier parsing
		data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

		var pkg SSISPackage
		if err := xml.Unmarshal(data, &pkg); err != nil {
			continue // Skip files that can't be parsed
		}

		packageName := filepath.Base(filePath)

		// Extract connections
		for _, conn := range pkg.ConnectionMgr.Connections {
			connKey := conn.Name // Use connection name as key
			if connections[connKey] == nil {
				connections[connKey] = &ConnectionInfo{
					Name:     conn.Name,
					Packages: []string{},
				}
			}
			// Add package to connection's package list if not already present
			found := false
			for _, pkg := range connections[connKey].Packages {
				if pkg == packageName {
					found = true
					break
				}
			}
			if !found {
				connections[connKey].Packages = append(connections[connKey].Packages, packageName)
			}
		}

		// Extract variables
		for _, variable := range pkg.Variables.Vars {
			varKey := variable.Name // Use variable name as key
			if variables[varKey] == nil {
				variables[varKey] = &VariableInfo{
					Name:     variable.Name,
					Packages: []string{},
				}
			}
			// Add package to variable's package list if not already present
			found := false
			for _, pkg := range variables[varKey].Packages {
				if pkg == packageName {
					found = true
					break
				}
			}
			if !found {
				variables[varKey].Packages = append(variables[varKey].Packages, packageName)
			}
		}
	}

	// Build the result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Package Dependency Analysis (%d packages scanned)\n\n", len(dtsxFiles)))

	// Shared Connections
	result.WriteString("🔗 Shared Connections:\n")
	sharedConnections := 0
	for _, conn := range connections {
		if len(conn.Packages) > 1 {
			sharedConnections++
			result.WriteString(fmt.Sprintf("• %s (used by %d packages):\n", conn.Name, len(conn.Packages)))
			for _, pkg := range conn.Packages {
				result.WriteString(fmt.Sprintf("  - %s\n", pkg))
			}
			result.WriteString("\n")
		}
	}
	if sharedConnections == 0 {
		result.WriteString("No shared connections found.\n\n")
	}

	// Shared Variables
	result.WriteString("📊 Shared Variables:\n")
	sharedVariables := 0
	for _, variable := range variables {
		if len(variable.Packages) > 1 {
			sharedVariables++
			result.WriteString(fmt.Sprintf("• %s (used by %d packages):\n", variable.Name, len(variable.Packages)))
			for _, pkg := range variable.Packages {
				result.WriteString(fmt.Sprintf("  - %s\n", pkg))
			}
			result.WriteString("\n")
		}
	}
	if sharedVariables == 0 {
		result.WriteString("No shared variables found.\n\n")
	}

	// Summary
	result.WriteString("📈 Summary:\n")
	result.WriteString(fmt.Sprintf("• Total packages analyzed: %d\n", len(dtsxFiles)))
	result.WriteString(fmt.Sprintf("• Shared connections: %d\n", sharedConnections))
	result.WriteString(fmt.Sprintf("• Shared variables: %d\n", sharedVariables))

	if sharedConnections > 0 || sharedVariables > 0 {
		result.WriteString("\n💡 These shared resources indicate potential dependencies between packages.")
		result.WriteString("\n   Consider documenting these relationships for maintenance and deployment purposes.")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeConfigurations(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Configuration Analysis:\n\n")

	if len(pkg.Configurations.Configs) == 0 {
		result.WriteString("No configurations found in this package.\n")
		result.WriteString("\n💡 Note: Configurations were used in SSIS 2005-2008 for parameterization.")
		result.WriteString(" Modern SSIS (2012+) uses Parameters instead.")
		return mcp.NewToolResultText(result.String()), nil
	}

	result.WriteString(fmt.Sprintf("Found %d configuration(s):\n\n", len(pkg.Configurations.Configs)))

	// Configuration type mapping
	configTypes := map[int]string{
		0: "Parent Package Variable",
		1: "XML Configuration File",
		2: "Environment Variable",
		3: "Registry Entry",
		4: "Parent Package Variable (indirect)",
		5: "XML Configuration File (indirect)",
		6: "Environment Variable (indirect)",
		7: "Registry Entry (indirect)",
		8: "SQL Server",
		9: "SQL Server (indirect)",
	}

	for i, config := range pkg.Configurations.Configs {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, config.Name))

		// Configuration type
		typeName, exists := configTypes[config.Type]
		if exists {
			result.WriteString(fmt.Sprintf("   Type: %s (%d)\n", typeName, config.Type))
		} else {
			result.WriteString(fmt.Sprintf("   Type: Unknown (%d)\n", config.Type))
		}

		// Description
		if config.Description != "" {
			result.WriteString(fmt.Sprintf("   Description: %s\n", config.Description))
		}

		// Configuration string (connection info for SQL Server, file path for XML, etc.)
		if config.ConfigurationString != "" {
			result.WriteString(fmt.Sprintf("   Configuration String: %s\n", config.ConfigurationString))
		}

		// Configured type and value
		if config.ConfiguredType != "" {
			result.WriteString(fmt.Sprintf("   Configured Type: %s\n", config.ConfiguredType))
		}
		if config.ConfiguredValue != "" {
			result.WriteString(fmt.Sprintf("   Configured Value: %s\n", config.ConfiguredValue))
		}

		result.WriteString("\n")
	}

	// Summary and recommendations
	result.WriteString("📋 Configuration Summary:\n")
	xmlConfigs := 0
	sqlConfigs := 0
	envConfigs := 0

	for _, config := range pkg.Configurations.Configs {
		switch config.Type {
		case 1, 5: // XML Configuration File
			xmlConfigs++
		case 8, 9: // SQL Server
			sqlConfigs++
		case 2, 6: // Environment Variable
			envConfigs++
		}
	}

	if xmlConfigs > 0 {
		result.WriteString(fmt.Sprintf("• XML Configuration Files: %d\n", xmlConfigs))
	}
	if sqlConfigs > 0 {
		result.WriteString(fmt.Sprintf("• SQL Server Configurations: %d\n", sqlConfigs))
	}
	if envConfigs > 0 {
		result.WriteString(fmt.Sprintf("• Environment Variable Configurations: %d\n", envConfigs))
	}

	result.WriteString("\n💡 Recommendations:\n")
	result.WriteString("• Consider migrating to SSIS 2012+ Parameters for better security and maintainability\n")
	result.WriteString("• XML configurations should be stored in secure locations\n")
	result.WriteString("• SQL Server configurations require appropriate database permissions\n")
	result.WriteString("• Environment variables are machine-specific and may not work across environments\n")

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzePerformanceMetrics(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Performance Metrics Analysis:\n\n")

	// Analyze package-level performance properties
	result.WriteString("📦 Package-Level Performance Settings:\n")
	packagePerfProps := extractPerformanceProperties(pkg.Properties, "package")
	if len(packagePerfProps) > 0 {
		for _, prop := range packagePerfProps {
			result.WriteString(fmt.Sprintf("• %s: %s\n", prop.Name, prop.Value))
			if prop.Recommendation != "" {
				result.WriteString(fmt.Sprintf("  💡 %s\n", prop.Recommendation))
			}
		}
	} else {
		result.WriteString("No performance-related package properties found.\n")
	}
	result.WriteString("\n")

	// Analyze data flow performance settings
	result.WriteString("🔄 Data Flow Performance Analysis:\n")
	dataFlowCount := 0

	for _, task := range pkg.Executables.Tasks {
		if isDataFlowTask(task) {
			dataFlowCount++
			result.WriteString(fmt.Sprintf("Data Flow Task: %s\n", task.Name))

			// Extract data flow task properties
			taskPerfProps := extractPerformanceProperties(task.Properties, "dataflow")
			if len(taskPerfProps) > 0 {
				result.WriteString("  Task Properties:\n")
				for _, prop := range taskPerfProps {
					result.WriteString(fmt.Sprintf("  • %s: %s\n", prop.Name, prop.Value))
					if prop.Recommendation != "" {
						result.WriteString(fmt.Sprintf("    💡 %s\n", prop.Recommendation))
					}
				}
			}

			// Analyze data flow components
			if task.ObjectData.DataFlow.Components.Components != nil {
				result.WriteString("  Components:\n")
				for _, comp := range task.ObjectData.DataFlow.Components.Components {
					compPerfProps := extractComponentPerformanceProperties(comp)
					if len(compPerfProps) > 0 {
						result.WriteString(fmt.Sprintf("    • %s (%s):\n", comp.Name, getComponentType(comp.ComponentClassID)))
						for _, prop := range compPerfProps {
							result.WriteString(fmt.Sprintf("      - %s: %s\n", prop.Name, prop.Value))
							if prop.Recommendation != "" {
								result.WriteString(fmt.Sprintf("        💡 %s\n", prop.Recommendation))
							}
						}
					}
				}
			}
			result.WriteString("\n")
		}
	}

	if dataFlowCount == 0 {
		result.WriteString("No Data Flow Tasks found in this package.\n\n")
	}

	// Performance recommendations
	result.WriteString("🚀 Performance Optimization Recommendations:\n")
	result.WriteString("• Increase DefaultBufferSize if processing large datasets (recommended: 10MB+)\n")
	result.WriteString("• Adjust DefaultBufferMaxRows based on row size (recommended: 10,000-100,000)\n")
	result.WriteString("• Increase EngineThreads for parallel processing (recommended: 2-4 per CPU core)\n")
	result.WriteString("• Use BLOBTempStoragePath and BufferTempStoragePath for large datasets\n")
	result.WriteString("• Consider MaxConcurrentExecutables for parallel task execution\n")
	result.WriteString("• Monitor AutoAdjustBufferSize for optimal memory usage\n")

	return mcp.NewToolResultText(result.String()), nil
}

func extractPerformanceProperties(properties []Property, category string) []PerformanceProperty {
	var perfProps []PerformanceProperty

	performancePropNames := map[string]bool{
		// Package level
		"MaxConcurrentExecutables": true,
		"EnableConfigurableRetry":  true,
		"RetryCount":               true,
		"RetryInterval":            true,

		// Data flow level
		"DefaultBufferSize":     true,
		"DefaultBufferMaxRows":  true,
		"EngineThreads":         true,
		"BufferTempStoragePath": true,
		"BLOBTempStoragePath":   true,
		"AutoAdjustBufferSize":  true,
		"BufferMaxRows":         true,
		"MaxBufferSize":         true,
		"MinBufferSize":         true,
	}

	for _, prop := range properties {
		if performancePropNames[prop.Name] {
			perfProp := PerformanceProperty{
				Name:  prop.Name,
				Value: prop.Value,
			}

			// Add recommendations based on property values
			switch prop.Name {
			case "DefaultBufferSize":
				if val, err := strconv.Atoi(prop.Value); err == nil && val < 10485760 { // 10MB
					perfProp.Recommendation = "Consider increasing to 10MB+ for better performance with large datasets"
				}
			case "DefaultBufferMaxRows":
				if val, err := strconv.Atoi(prop.Value); err == nil && val < 10000 {
					perfProp.Recommendation = "Consider increasing to 10,000+ rows for better throughput"
				}
			case "EngineThreads":
				if val, err := strconv.Atoi(prop.Value); err == nil && val < 2 {
					perfProp.Recommendation = "Consider increasing to 2+ threads for parallel processing"
				}
			}

			perfProps = append(perfProps, perfProp)
		}
	}

	return perfProps
}

func extractComponentPerformanceProperties(component DataFlowComponent) []PerformanceProperty {
	var perfProps []PerformanceProperty

	for _, prop := range component.ObjectData.PipelineComponent.Properties.Properties {
		// Component-specific performance properties
		switch prop.Name {
		case "CommandTimeout", "BatchSize", "RowsPerBatch", "MaximumInsertCommitSize":
			perfProps = append(perfProps, PerformanceProperty{
				Name:  prop.Name,
				Value: prop.Value,
			})
		}
	}

	return perfProps
}

func isDataFlowTask(task Task) bool {
	for _, prop := range task.Properties {
		if prop.Name == "CreationName" && strings.Contains(prop.Value, "DataFlow") {
			return true
		}
	}
	return false
}

func getTaskType(task Task) string {
	// Determine task type based on properties or creation name
	for _, prop := range task.Properties {
		if prop.Name == "CreationName" {
			switch prop.Value {
			case "Microsoft.ExecuteSQLTask":
				return "Execute SQL Task"
			case "Microsoft.SendMailTask":
				return "Send Mail Task"
			case "Microsoft.ExecuteProcessTask":
				return "Execute Process Task"
			case "Microsoft.ScriptTask":
				return "Script Task"
			case "Microsoft.BulkInsertTask":
				return "Bulk Insert Task"
			case "Microsoft.DataProfilingTask":
				return "Data Profiling Task"
			case "Microsoft.MessageQueueTask":
				return "Message Queue Task"
			default:
				return prop.Value
			}
		}
	}
	return "Unknown Task Type"
}

func getComponentType(classID string) string {
	// Map common SSIS component class IDs to readable names
	switch {
	case strings.Contains(classID, "OLEDBSource"):
		return "OLE DB Source"
	case strings.Contains(classID, "OLEDBDestination"):
		return "OLE DB Destination"
	case strings.Contains(classID, "FlatFileSource"):
		return "Flat File Source"
	case strings.Contains(classID, "FlatFileDestination"):
		return "Flat File Destination"
	case strings.Contains(classID, "Lookup"):
		return "Lookup"
	case strings.Contains(classID, "Sort"):
		return "Sort"
	case strings.Contains(classID, "Aggregate"):
		return "Aggregate"
	case strings.Contains(classID, "Merge"):
		return "Merge"
	case strings.Contains(classID, "MergeJoin"):
		return "Merge Join"
	case strings.Contains(classID, "ConditionalSplit"):
		return "Conditional Split"
	case strings.Contains(classID, "Multicast"):
		return "Multicast"
	case strings.Contains(classID, "UnionAll"):
		return "Union All"
	case strings.Contains(classID, "ScriptComponent"):
		return "Script Component"
	case strings.Contains(classID, "Lineage"):
		return "Audit"
	case strings.Contains(classID, "ManagedComponentHost"):
		return "Script Component"
	case strings.Contains(classID, "Cache"):
		return "Cache Transform"
	default:
		return classID
	}
}

func isKeyProperty(propName string) bool {
	keyProps := []string{
		"SqlCommand", "TableOrViewName", "FileName", "ConnectionString",
		"Expression", "SortKeyPosition", "AggregationType", "Operation",
	}
	for _, key := range keyProps {
		if propName == key {
			return true
		}
	}
	return false
}

// runHTTPServer starts an HTTP server with streaming capabilities
func runHTTPServer(s *server.MCPServer, port string) {
	// Use the official MCP StreamableHTTPServer for proper MCP HTTP transport
	streamableServer := server.NewStreamableHTTPServer(s)

	log.Printf("Starting MCP HTTP server on port %s", port)
	log.Printf("MCP endpoints available at: http://localhost:%s/mcp", port)
	log.Printf("Health check available at: http://localhost:%s/health", port)

	// Start the server
	if err := streamableServer.Start(":" + port); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
