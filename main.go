package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/MCPRUNNER/gossisMCP/pkg/config"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/analysis"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/extraction"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/optimization"
	packagehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/packages"
	templatehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/templates"
	serverutil "github.com/MCPRUNNER/gossisMCP/pkg/util/server"
	workflowutil "github.com/MCPRUNNER/gossisMCP/pkg/util/workflow"
	"github.com/MCPRUNNER/gossisMCP/pkg/workflow"
)

// loadConfigFile loads configuration from JSON or YAML file
// loadEnvironmentConfig loads configuration overrides from environment variables
// validateConfig validates the configuration
// configureLogging configures the logging based on the configuration
func configureLogging(config config.LoggingConfig) {
	// Set log level (simplified - in a real implementation you'd use a proper logging library)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if strings.ToLower(config.Level) == "debug" {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	}
}

func main() {
	// Command line flags
	httpMode := flag.Bool("http", false, "Run in HTTP streaming mode")
	httpPort := flag.String("port", "8086", "HTTP server port")
	pkgDir := flag.String("pkg-dir", "", "Root directory for SSIS packages (can also be set via GOSSIS_PKG_DIRECTORY env var, defaults to current working directory)")
	configPath := flag.String("config", "", "Path to configuration file (JSON or YAML)")
	flag.Parse()

	// Load configuration
	config, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config with command line flags if provided
	if *httpMode {
		config.Server.HTTPMode = true
	}
	if *httpPort != "8086" {
		config.Server.Port = *httpPort
	}
	if *pkgDir != "" {
		config.Packages.Directory = *pkgDir
	}

	// Configure logging
	configureLogging(config.Logging)

	// Determine package directory from config, environment variable, or default
	packageDirectory := config.Packages.Directory
	excludeFile := config.Packages.ExcludeFile
	if packageDirectory == "" {
		packageDirectory = os.Getenv("GOSSIS_PKG_DIRECTORY")
	}
	if packageDirectory == "" {
		// Default to current working directory if neither config nor env var is set
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

	// Initialize plugin system
	pluginSystem := NewPluginSystem(config.Plugins)

	// Register plugin management tools
	pluginSystem.createPluginManagementTools(s)

	// Register all tools...
	// Tool to parse DTSX file and return summary
	parseTool := mcp.NewTool("parse_dtsx",
		mcp.WithDescription("Parse an SSIS DTSX file and return a summary of its structure"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file to parse (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(parseTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleParseDtsx(ctx, request, packageDirectory)
	})

	// Tool to extract tasks
	extractTasksTool := mcp.NewTool("extract_tasks",
		mcp.WithDescription("Extract and list all tasks from a DTSX file, including resolved expressions in task properties"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(extractTasksTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleExtractTasks(ctx, request, packageDirectory)
	})

	// Tool to extract connections
	extractConnectionsTool := mcp.NewTool("extract_connections",
		mcp.WithDescription("Extract and list all connection managers from a DTSX file, including resolved expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(extractConnectionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleExtractConnections(ctx, request, packageDirectory)
	})

	// Tool to extract precedence constraints
	extractPrecedenceTool := mcp.NewTool("extract_precedence_constraints",
		mcp.WithDescription("Extract and list all precedence constraints from a DTSX file, including resolved expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(extractPrecedenceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleExtractPrecedenceConstraints(ctx, request, packageDirectory)
	})

	// Tool to extract variables
	extractVariablesTool := mcp.NewTool("extract_variables",
		mcp.WithDescription("Extract and list all variables from a DTSX file, including resolved expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(extractVariablesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleExtractVariables(ctx, request, packageDirectory)
	})

	// Tool to extract parameters
	extractParametersTool := mcp.NewTool("extract_parameters",
		mcp.WithDescription("Extract and list all parameters from a DTSX file, including data types, default values, and properties"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(extractParametersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleExtractParameters(ctx, request, packageDirectory)
	})

	// Tool to extract script code from Script Tasks
	extractScriptTool := mcp.NewTool("extract_script_code",
		mcp.WithDescription("Extract script code from Script Tasks in a DTSX file"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(extractScriptTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleExtractScriptCode(ctx, request, packageDirectory)
	})

	// Tool to validate best practices
	validateBestPracticesTool := mcp.NewTool("validate_best_practices",
		mcp.WithDescription("Check SSIS package for best practices and potential issues"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(validateBestPracticesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleValidateBestPractices(ctx, request, packageDirectory)
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
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(askTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleAskAboutDtsx(ctx, request, packageDirectory)
	})

	// Tool to analyze Message Queue Tasks
	analyzeMessageQueueTool := mcp.NewTool("analyze_message_queue_tasks",
		mcp.WithDescription("Analyze Message Queue Tasks in a DTSX file, including send/receive operations and message content"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeMessageQueueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleAnalyzeMessageQueueTasks(ctx, request, packageDirectory)
	})

	// Tool to analyze Script Tasks
	analyzeScriptTaskTool := mcp.NewTool("analyze_script_task",
		mcp.WithDescription("Analyze Script Tasks in a DTSX file, including script code, variables, and task configuration"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeScriptTaskTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleAnalyzeScriptTask(ctx, request, packageDirectory)
	})

	// Tool to detect hard-coded values
	detectHardcodedValuesTool := mcp.NewTool("detect_hardcoded_values",
		mcp.WithDescription("Detect hard-coded values in a DTSX file, such as embedded literals in connection strings, messages, or expressions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(detectHardcodedValuesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleDetectHardcodedValues(ctx, request, packageDirectory)
	})

	// Tool to analyze logging configuration
	analyzeLoggingTool := mcp.NewTool("analyze_logging_configuration",
		mcp.WithDescription("Analyze detailed logging configuration in a DTSX file, including log providers, events, and destinations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeLoggingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleAnalyzeLoggingConfiguration(ctx, request, packageDirectory)
	})

	// Tool to list all DTSX packages in the package directory
	listPackagesTool := mcp.NewTool("list_packages",
		mcp.WithDescription("Recursively list all DTSX packages found in the package directory"),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(listPackagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleListPackages(ctx, request, packageDirectory, excludeFile)
	})

	renderTemplateTool := mcp.NewTool("render_template",
		mcp.WithDescription("Render an html/template using JSON data and write the output to a file"),
		mcp.WithString("template_file_path",
			mcp.Required(),
			mcp.Description("Path to the template file (relative to package directory if set)"),
		),
		mcp.WithString("output_file_path",
			mcp.Required(),
			mcp.Description("Destination path for the rendered output (relative to package directory if set)"),
		),
		mcp.WithString("json_data",
			mcp.Description("Inline JSON payload to apply to the template"),
		),
		mcp.WithString("json_file_path",
			mcp.Description("Path to a JSON file containing the template data (relative to package directory if set)"),
		),
	)
	s.AddTool(renderTemplateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return templatehandlers.HandleRenderTemplate(ctx, request, packageDirectory)
	})

	// Tool to analyze data flow components
	analyzeDataFlowTool := mcp.NewTool("analyze_data_flow",
		mcp.WithDescription("Analyze Data Flow components in a DTSX file, including sources, transformations, destinations, and data paths"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDataFlowTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDataFlow(ctx, request, packageDirectory)
	})

	// Tool to analyze data flow components with detailed configurations
	analyzeDataFlowDetailedTool := mcp.NewTool("analyze_data_flow_detailed",
		mcp.WithDescription("Provide detailed analysis of Data Flow components including configurations, properties, inputs/outputs, and data mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDataFlowDetailedTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDataFlowDetailed(ctx, request, packageDirectory)
	})

	// Unified tool to analyze source components
	analyzeSourceTool := mcp.NewTool("analyze_source",
		mcp.WithDescription("Analyze source components in a DTSX file by type (unified interface for all source types)"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
		mcp.WithString("source_type",
			mcp.Required(),
			mcp.Description("Type of source to analyze: ole_db, ado_net, odbc, flat_file, excel, access, xml, raw_file, cdc, sap_bw"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSource(ctx, request, packageDirectory)
	})

	// Unified tool to analyze destination components
	analyzeDestinationTool := mcp.NewTool("analyze_destination",
		mcp.WithDescription("Analyze destination components in a DTSX file by type (unified interface for all destination types)"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
		mcp.WithString("destination_type",
			mcp.Required(),
			mcp.Description("Type of destination to analyze: ole_db, flat_file, sql_server, excel, raw_file"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze OLE DB Destination components
	analyzeOLEDBDestinationTool := mcp.NewTool("analyze_ole_db_destination",
		mcp.WithDescription("Analyze OLE DB Destination components in a DTSX file, extracting connection, access mode, and column mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeOLEDBDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeOLEDBDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze Flat File Destination components
	analyzeFlatFileDestinationTool := mcp.NewTool("analyze_flat_file_destination",
		mcp.WithDescription("Analyze Flat File Destination components in a DTSX file, extracting file configuration and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeFlatFileDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeFlatFileDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze SQL Server Destination components
	analyzeSQLServerDestinationTool := mcp.NewTool("analyze_sql_server_destination",
		mcp.WithDescription("Analyze SQL Server Destination components in a DTSX file, extracting table configuration and bulk load settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeSQLServerDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSQLServerDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze OLE DB Source components
	analyzeOLEDBSourceTool := mcp.NewTool("analyze_ole_db_source",
		mcp.WithDescription("Analyze OLE DB Source components in a DTSX file, extracting connection details, access mode, SQL commands, and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeOLEDBSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeOLEDBSource(ctx, request, packageDirectory)
	})

	// Tool to analyze ADO.NET Source components
	analyzeADONETSourceTool := mcp.NewTool("analyze_ado_net_source",
		mcp.WithDescription("Analyze ADO.NET Source components in a DTSX file, extracting connection, command, and column metadata"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeADONETSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeADONETSource(ctx, request, packageDirectory)
	})

	// Tool to analyze ODBC Source components
	analyzeODBCSourceTool := mcp.NewTool("analyze_odbc_source",
		mcp.WithDescription("Analyze ODBC Source components in a DTSX file, extracting DSN usage, SQL command, and column metadata"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeODBCSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeODBCSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Flat File Source components
	analyzeFlatFileSourceTool := mcp.NewTool("analyze_flat_file_source",
		mcp.WithDescription("Analyze Flat File Source components in a DTSX file, extracting file configuration and column definitions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeFlatFileSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeFlatFileSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Excel Source components
	analyzeExcelSourceTool := mcp.NewTool("analyze_excel_source",
		mcp.WithDescription("Analyze Excel Source components in a DTSX file, extracting worksheet configuration and column metadata"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeExcelSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeExcelSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Access Source components
	analyzeAccessSourceTool := mcp.NewTool("analyze_access_source",
		mcp.WithDescription("Analyze Access Source components in a DTSX file, extracting database configuration and query details"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeAccessSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeAccessSource(ctx, request, packageDirectory)
	})

	// Tool to analyze XML Source components
	analyzeXMLSourceTool := mcp.NewTool("analyze_xml_source",
		mcp.WithDescription("Analyze XML Source components in a DTSX file, extracting schema, inline fragments, and column mapping"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeXMLSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeXMLSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Raw File Source components
	analyzeRawFileSourceTool := mcp.NewTool("analyze_raw_file_source",
		mcp.WithDescription("Analyze Raw File Source components in a DTSX file, extracting file metadata and column mapping"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeRawFileSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeRawFileSource(ctx, request, packageDirectory)
	})

	// Tool to analyze CDC Source components
	analyzeCDCSourceTool := mcp.NewTool("analyze_cdc_source",
		mcp.WithDescription("Analyze CDC Source components in a DTSX file, extracting capture instance and synchronization settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeCDCSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeCDCSource(ctx, request, packageDirectory)
	})

	// Tool to analyze SAP BW Source components
	analyzeSAPBWSourceTool := mcp.NewTool("analyze_sap_bw_source",
		mcp.WithDescription("Analyze SAP BW Source components in a DTSX file, extracting connection, query, and extraction details"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeSAPBWSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSAPBWSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Export Column destinations
	analyzeExportColumnTool := mcp.NewTool("analyze_export_column",
		mcp.WithDescription("Analyze Export Column destinations in a DTSX file, extracting file data columns, file path columns, and export settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeExportColumnTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeExportColumn(ctx, request, packageDirectory)
	})

	// Tool to analyze Data Conversion transformations
	analyzeDataConversionTool := mcp.NewTool("analyze_data_conversion",
		mcp.WithDescription("Analyze Data Conversion transformations in a DTSX file, extracting input/output mappings and data type conversions"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDataConversionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDataConversion(ctx, request, packageDirectory)
	})

	// Tool to analyze Union All transformations
	analyzeUnionAllTool := mcp.NewTool("analyze_union_all",
		mcp.WithDescription("Analyze Union All transformations in a DTSX file, extracting input mappings and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeUnionAllTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeUnionAll(ctx, request, packageDirectory)
	})

	// Tool to analyze Multicast transformations
	analyzeMulticastTool := mcp.NewTool("analyze_multicast",
		mcp.WithDescription("Analyze Multicast transformations in a DTSX file, extracting input/output configuration details"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeMulticastTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeMulticast(ctx, request, packageDirectory)
	})

	// Tool to analyze Derived Column transformations
	analyzeDerivedColumnTool := mcp.NewTool("analyze_derived_column",
		mcp.WithDescription("Analyze Derived Column transformations in a DTSX file, extracting expression logic and output mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDerivedColumnTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDerivedColumn(ctx, request, packageDirectory)
	})

	// Tool to analyze Lookup transformations
	analyzeLookupTool := mcp.NewTool("analyze_lookup",
		mcp.WithDescription("Analyze Lookup transformations in a DTSX file, extracting query configuration, caching, and column mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeLookupTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeLookup(ctx, request, packageDirectory)
	})

	// Tool to analyze Conditional Split transformations
	analyzeConditionalSplitTool := mcp.NewTool("analyze_conditional_split",
		mcp.WithDescription("Analyze Conditional Split transformations in a DTSX file, extracting conditions and output paths"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeConditionalSplitTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeConditionalSplit(ctx, request, packageDirectory)
	})

	// Tool to analyze Sort transformations
	analyzeSortTool := mcp.NewTool("analyze_sort",
		mcp.WithDescription("Analyze Sort transformations in a DTSX file, extracting sort keys and advanced options"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeSortTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSort(ctx, request, packageDirectory)
	})

	// Tool to analyze Aggregate transformations
	analyzeAggregateTool := mcp.NewTool("analyze_aggregate",
		mcp.WithDescription("Analyze Aggregate transformations in a DTSX file, extracting grouping and aggregation settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeAggregateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeAggregate(ctx, request, packageDirectory)
	})

	// Tool to analyze Merge Join transformations
	analyzeMergeJoinTool := mcp.NewTool("analyze_merge_join",
		mcp.WithDescription("Analyze Merge Join transformations in a DTSX file, extracting join configuration and column lineage"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeMergeJoinTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeMergeJoin(ctx, request, packageDirectory)
	})

	// Tool to analyze Row Count transformations
	analyzeRowCountTool := mcp.NewTool("analyze_row_count",
		mcp.WithDescription("Analyze Row Count transformations in a DTSX file, extracting variable assignments and result storage"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeRowCountTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeRowCount(ctx, request, packageDirectory)
	})

	// Tool to analyze Character Map transformations
	analyzeCharacterMapTool := mcp.NewTool("analyze_character_map",
		mcp.WithDescription("Analyze Character Map transformations in a DTSX file, extracting character mapping operations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeCharacterMapTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeCharacterMap(ctx, request, packageDirectory)
	})

	// Tool to analyze Copy Column transformations
	analyzeCopyColumnTool := mcp.NewTool("analyze_copy_column",
		mcp.WithDescription("Analyze Copy Column transformations in a DTSX file, extracting column duplication settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeCopyColumnTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeCopyColumn(ctx, request, packageDirectory)
	})

	// Tool to analyze containers
	analyzeContainersTool := mcp.NewTool("analyze_containers",
		mcp.WithDescription("Analyze containers in a DTSX file, including Sequence, For Loop, and Foreach Loop containers with their properties and nested executables"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeContainersTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeContainers(ctx, request, packageDirectory)
	})

	// Tool to analyze custom and third-party components
	analyzeCustomComponentsTool := mcp.NewTool("analyze_custom_components",
		mcp.WithDescription("Analyze custom and third-party components in a DTSX file, identifying non-standard components and their configurations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeCustomComponentsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeCustomComponents(ctx, request, packageDirectory)
	})

	comparePackagesTool := mcp.NewTool("compare_packages",
		mcp.WithDescription("Compare two DTSX files and highlight differences"),
		mcp.WithString("file_path1",
			mcp.Required(),
			mcp.Description("Path to the first DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("file_path2",
			mcp.Required(),
			mcp.Description("Path to the second DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(comparePackagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleComparePackages(ctx, request, packageDirectory)
	})

	analyzeCodeQualityTool := mcp.NewTool("analyze_code_quality",
		mcp.WithDescription("Calculate maintainability metrics (complexity, duplication, etc.) to assess package quality and technical debt"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeCodeQualityTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeCodeQuality(ctx, request, packageDirectory)
	})

	readTextFileTool := mcp.NewTool("read_text_file",
		mcp.WithDescription("Read configuration or data from text files referenced by SSIS packages"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the text file to read (relative to package directory if set, or absolute path)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithBoolean("line_numbers",
			mcp.DefaultBool(true),
			mcp.Description("Include enable line numbers in the content (true or false, default: true)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(readTextFileTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleReadTextFile(ctx, request, packageDirectory)
	})

	// Tool for XPath queries on XML data
	xpathTool := mcp.NewTool("xpath_query",
		mcp.WithDescription("Execute XPath queries on XML data from files, raw XML strings, or JSONified XML content"),
		mcp.WithString("xpath",
			mcp.Required(),
			mcp.Description("XPath expression to execute"),
		),
		mcp.WithString("file_path",
			mcp.Description("Path to XML file to query (relative to package directory if set)"),
		),
		mcp.WithString("xml",
			mcp.Description("Raw XML string to query"),
		),
		mcp.WithString("json_xml",
			mcp.Description("JSONified XML content (e.g., from read_text_file output)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(xpathTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return extraction.HandleXPathQuery(ctx, request, packageDirectory)
	})

	// Advanced Security Features - Phase 2

	// Tool for comprehensive credential scanning with pattern matching
	scanCredentialsTool := mcp.NewTool("scan_credentials",
		mcp.WithDescription("Perform comprehensive credential scanning with advanced pattern matching to detect hardcoded credentials, API keys, tokens, and sensitive data patterns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(scanCredentialsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleScanCredentials(ctx, request, packageDirectory)
	})

	// Tool for encryption detection and recommendations
	detectEncryptionTool := mcp.NewTool("detect_encryption",
		mcp.WithDescription("Detect encryption settings and provide recommendations for securing sensitive data in SSIS packages"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(detectEncryptionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleDetectEncryption(ctx, request, packageDirectory)
	})

	// Tool for compliance checking (GDPR, HIPAA patterns)
	checkComplianceTool := mcp.NewTool("check_compliance",
		mcp.WithDescription("Check SSIS packages for compliance with GDPR, HIPAA, and other regulatory requirements by detecting sensitive data patterns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("compliance_standard",
			mcp.Description("Compliance standard to check (gdpr, hipaa, pci, or 'all' for comprehensive check)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(checkComplianceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleCheckCompliance(ctx, request, packageDirectory)
	})

	// Performance Optimization Tools - Phase 2

	// Tool for buffer size optimization recommendations
	optimizeBufferSizeTool := mcp.NewTool("optimize_buffer_size",
		mcp.WithDescription("Analyze and provide specific buffer size optimization recommendations for data flows based on data volume and processing patterns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(optimizeBufferSizeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return optimization.HandleOptimizeBufferSize(ctx, request, packageDirectory)
	})

	// Tool for parallel processing analysis
	analyzeParallelProcessingTool := mcp.NewTool("analyze_parallel_processing",
		mcp.WithDescription("Analyze parallel processing capabilities and provide recommendations for optimizing concurrent execution in SSIS packages"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeParallelProcessingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return optimization.HandleAnalyzeParallelProcessing(ctx, request, packageDirectory)
	})

	// Tool for memory usage profiling of data flows
	profileMemoryUsageTool := mcp.NewTool("profile_memory_usage",
		mcp.WithDescription("Profile memory usage patterns in data flows and provide recommendations for optimizing memory consumption"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(profileMemoryUsageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return optimization.HandleProfileMemoryUsage(ctx, request, packageDirectory)
	})
	// Tool to merge multiple JSON files into a single JSON object
	mergeJSONFilesTool := mcp.NewTool("merge_json",

		mcp.WithDescription("Merge multiple JSON files into a single JSON object with a 'root' parent, where each file's data is nested under its base filename"),
		mcp.WithArray("file_paths",
			mcp.Required(),
			mcp.Description("Array of JSON file paths to merge (relative to package directory if set)"),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the merged JSON (relative to package directory if set)"),
		),
	)
	s.AddTool(mergeJSONFilesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.MergeJSONFilesHandler(ctx, request, packageDirectory)
	})
	// Batch Processing Tools

	// Tool for batch analysis of multiple DTSX files
	batchAnalyzeTool := mcp.NewTool("batch_analyze",
		mcp.WithDescription("Analyze multiple DTSX files in parallel and provide aggregated results"),
		mcp.WithArray("file_paths",
			mcp.Required(),
			mcp.Description("Array of DTSX file paths to analyze (relative to package directory if set)"),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithNumber("max_concurrent",
			mcp.Description("Maximum number of concurrent analyses (default: 4)"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the tool result (relative to package directory if set)"),
		),
	)
	s.AddTool(batchAnalyzeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return packagehandlers.HandleBatchAnalyze(ctx, request, packageDirectory)
	})

	registerWorkflowRunnerTool(s, packageDirectory, excludeFile)

	if config.Server.HTTPMode {
		// Run in HTTP streaming mode
		serverutil.RunHTTPServer(s, config.Server.Port)
	} else {
		// Run in stdio mode (default)
		if err := server.ServeStdio(s); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}
}

func registerWorkflowRunnerTool(s *server.MCPServer, packageDirectory, excludeFile string) {
	workflowRunnerTool := mcp.NewTool("workflow_runner",
		mcp.WithDescription("Execute a workflow definition file and run each referenced MCP tool step sequentially"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the workflow definition (JSON or YAML)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: markdown (default) or json"),
		),
		mcp.WithString("output_file_path",
			mcp.Description("Destination path to write the workflow summary (relative to package directory if set)"),
		),
	)

	s.AddTool(workflowRunnerTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleWorkflowRunner(ctx, request, packageDirectory, excludeFile)
	})
}

func handleWorkflowRunner(ctx context.Context, request mcp.CallToolRequest, packageDirectory, excludeFile string) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})

	workflowPath := workflowutil.ExtractStringArg(args, "file_path")
	if workflowPath == "" {
		workflowPath = workflowutil.ExtractStringArg(args, "workflow_file")
	}
	if workflowPath == "" {
		return mcp.NewToolResultError("workflow_runner requires a file_path parameter"), nil
	}

	if !filepath.IsAbs(workflowPath) {
		abs, err := filepath.Abs(workflowPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to resolve workflow path: %v", err)), nil
		}
		workflowPath = abs
	}

	info, err := os.Stat(workflowPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to access workflow file: %v", err)), nil
	}
	if info.IsDir() {
		return mcp.NewToolResultError("workflow_runner expects a file, not a directory"), nil
	}

	workflowDir := filepath.Dir(workflowPath)
	var writtenOutputs []string

	runner := func(stepCtx context.Context, tool string, params map[string]interface{}) (string, error) {
		normalized := workflowutil.CloneArguments(params)

		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "directory")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "file_path")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "outputFilePath")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "templateFilePath")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "output_file_path")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "template_file_path")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "json_file_path")
		workflowutil.NormalizeWorkflowPathArg(normalized, workflowPath, "jsonFilePath")
		workflowutil.NormalizeWorkflowPathArrayArg(normalized, workflowPath, "file_paths")

		if tool == "list_packages" {
			if format := workflowutil.StringFromAny(normalized["format"]); format == "" {
				normalized["format"] = "json"
			}
			if dir := workflowutil.StringFromAny(normalized["directory"]); dir == "" && packageDirectory != "" {
				normalized["directory"] = packageDirectory
			}
		}

		if tool == "batch_analyze" {
			if rawJSON, ok := normalized["jsonData"]; ok {
				jsonText, ok := rawJSON.(string)
				if !ok {
					return "", fmt.Errorf("batch_analyze: jsonData must be a string value")
				}
				files, err := workflowutil.ExtractFilePathsFromJSON(jsonText)
				if err != nil {
					return "", err
				}
				normalized["file_paths"] = workflowutil.ToInterfaceSlice(files)
				delete(normalized, "jsonData")
			}
		}

		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: normalized,
			},
		}

		var result *mcp.CallToolResult
		switch tool {
		case "list_packages":
			res, err := packagehandlers.HandleListPackages(stepCtx, req, packageDirectory, excludeFile)
			if err != nil {
				return "", err
			}
			result = res
		case "read_text_file":
			res, err := extraction.HandleReadTextFile(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "batch_analyze":
			res, err := packagehandlers.HandleBatchAnalyze(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_logging_configuration":
			res, err := packagehandlers.HandleAnalyzeLoggingConfiguration(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_data_flow":
			res, err := analysis.HandleAnalyzeDataFlow(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "render_template":
			res, err := templatehandlers.HandleRenderTemplate(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "merge_json":
			res, err := packagehandlers.MergeJSONFilesHandler(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_data_flow_detailed":
			res, err := analysis.HandleAnalyzeDataFlowDetailed(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "validate_best_practices":
			res, err := packagehandlers.HandleValidateBestPractices(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "parse_dtsx":
			res, err := extraction.HandleParseDtsx(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "extract_tasks":
			res, err := extraction.HandleExtractTasks(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "extract_connections":
			res, err := extraction.HandleExtractConnections(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "extract_precedence_constraints":
			res, err := extraction.HandleExtractPrecedenceConstraints(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "extract_variables":
			res, err := extraction.HandleExtractVariables(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "extract_parameters":
			res, err := extraction.HandleExtractParameters(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "extract_script_code":
			res, err := extraction.HandleExtractScriptCode(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "ask_about_dtsx":
			res, err := packagehandlers.HandleAskAboutDtsx(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_message_queue_tasks":
			res, err := packagehandlers.HandleAnalyzeMessageQueueTasks(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_script_task":
			res, err := packagehandlers.HandleAnalyzeScriptTask(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "detect_hardcoded_values":
			res, err := packagehandlers.HandleDetectHardcodedValues(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_source":
			res, err := analysis.HandleAnalyzeSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_destination":
			res, err := analysis.HandleAnalyzeDestination(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_ole_db_destination":
			res, err := analysis.HandleAnalyzeOLEDBDestination(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_flat_file_destination":
			res, err := analysis.HandleAnalyzeFlatFileDestination(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_sql_server_destination":
			res, err := analysis.HandleAnalyzeSQLServerDestination(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_ole_db_source":
			res, err := analysis.HandleAnalyzeOLEDBSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_ado_net_source":
			res, err := analysis.HandleAnalyzeADONETSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_odbc_source":
			res, err := analysis.HandleAnalyzeODBCSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_flat_file_source":
			res, err := analysis.HandleAnalyzeFlatFileSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_excel_source":
			res, err := analysis.HandleAnalyzeExcelSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_access_source":
			res, err := analysis.HandleAnalyzeAccessSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_xml_source":
			res, err := analysis.HandleAnalyzeXMLSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_raw_file_source":
			res, err := analysis.HandleAnalyzeRawFileSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_cdc_source":
			res, err := analysis.HandleAnalyzeCDCSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_sap_bw_source":
			res, err := analysis.HandleAnalyzeSAPBWSource(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_export_column":
			res, err := analysis.HandleAnalyzeExportColumn(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_data_conversion":
			res, err := analysis.HandleAnalyzeDataConversion(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_union_all":
			res, err := analysis.HandleAnalyzeUnionAll(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_multicast":
			res, err := analysis.HandleAnalyzeMulticast(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_derived_column":
			res, err := analysis.HandleAnalyzeDerivedColumn(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_lookup":
			res, err := analysis.HandleAnalyzeLookup(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_conditional_split":
			res, err := analysis.HandleAnalyzeConditionalSplit(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_sort":
			res, err := analysis.HandleAnalyzeSort(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_aggregate":
			res, err := analysis.HandleAnalyzeAggregate(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_merge_join":
			res, err := analysis.HandleAnalyzeMergeJoin(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_row_count":
			res, err := analysis.HandleAnalyzeRowCount(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_character_map":
			res, err := analysis.HandleAnalyzeCharacterMap(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_copy_column":
			res, err := analysis.HandleAnalyzeCopyColumn(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_containers":
			res, err := analysis.HandleAnalyzeContainers(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_custom_components":
			res, err := analysis.HandleAnalyzeCustomComponents(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "compare_packages":
			res, err := packagehandlers.HandleComparePackages(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_code_quality":
			res, err := analysis.HandleAnalyzeCodeQuality(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "xpath_query":
			res, err := extraction.HandleXPathQuery(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "scan_credentials":
			res, err := analysis.HandleScanCredentials(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "detect_encryption":
			res, err := analysis.HandleDetectEncryption(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "check_compliance":
			res, err := analysis.HandleCheckCompliance(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "optimize_buffer_size":
			res, err := optimization.HandleOptimizeBufferSize(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "analyze_parallel_processing":
			res, err := optimization.HandleAnalyzeParallelProcessing(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		case "profile_memory_usage":
			res, err := optimization.HandleProfileMemoryUsage(stepCtx, req, packageDirectory)
			if err != nil {
				return "", err
			}
			result = res
		default:
			return "", fmt.Errorf("workflow runner: tool %q is not supported", tool)
		}

		text, err := workflow.ToolResultToString(result)
		if err != nil {
			return "", err
		}

		renderedPath := ""
		if tool == "render_template" {
			renderedPath = workflowutil.StringFromAny(normalized["outputFilePath"])
			if renderedPath == "" {
				renderedPath = workflowutil.StringFromAny(normalized["output_file_path"])
			}
		}

		outputPath := workflowutil.StringFromAny(normalized["outputFilePath"])
		if outputPath == "" {
			outputPath = workflowutil.StringFromAny(normalized["output_file_path"])
		}
		if tool == "render_template" {
			outputPath = ""
		}
		if outputPath != "" {
			// If the tool returned structured content, prefer writing that JSON
			if result != nil && result.StructuredContent != nil {
				data, marshalErr := json.MarshalIndent(result.StructuredContent, "", "  ")
				if marshalErr != nil {
					return "", fmt.Errorf("failed to marshal structured tool result: %w", marshalErr)
				}
				if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
					return "", fmt.Errorf("failed to create workflow output directory %s: %w", filepath.Dir(outputPath), err)
				}
				if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
					return "", fmt.Errorf("failed to write workflow output %s: %w", outputPath, err)
				}
			} else {
				// If no structured content is available, only write text when the
				// destination file doesn't already exist or is empty — this avoids
				// overwriting files that handlers may have already created.
				if fi, statErr := os.Stat(outputPath); statErr == nil && fi.Size() > 0 {
					// file exists and is non-empty; skip overwriting
				} else {
					if err := workflowutil.WriteWorkflowOutput(outputPath, text); err != nil {
						return "", err
					}
				}
			}
			display := outputPath
			if rel, relErr := filepath.Rel(workflowDir, outputPath); relErr == nil && !strings.HasPrefix(rel, "..") {
				display = rel
			}
			writtenOutputs = append(writtenOutputs, display)
		}
		if tool == "render_template" && renderedPath != "" {
			display := renderedPath
			if rel, relErr := filepath.Rel(workflowDir, renderedPath); relErr == nil && !strings.HasPrefix(rel, "..") {
				display = rel
			}
			writtenOutputs = append(writtenOutputs, display)
		}

		return text, nil
	}

	wf, results, err := workflow.RunFile(ctx, workflowPath, runner)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Workflow execution failed: %v", err)), nil
	}

	// Write any combined step outputs declared by the workflow steps.
	if combined, cerr := workflow.WriteCombinedStepOutputs(workflowPath, wf, results); cerr == nil {
		writtenOutputs = append(writtenOutputs, combined...)
	} else {
		// helper already prints errors to stderr; still append any files that were written
		writtenOutputs = append(writtenOutputs, combined...)
	}

	summary := workflowutil.CreateWorkflowExecutionSummary(workflowPath, wf, results, writtenOutputs)

	format := strings.ToLower(workflowutil.ExtractStringArg(args, "format"))
	switch format {
	case "json":
		data, marshalErr := json.MarshalIndent(summary, "", "  ")
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal workflow summary: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	default:
		markdown := workflowutil.FormatWorkflowSummaryMarkdown(summary)
		return mcp.NewToolResultText(markdown), nil
	}
}
