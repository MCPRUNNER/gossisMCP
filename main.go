package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"

	"github.com/MCPRUNNER/gossisMCP/pkg/config"
	"github.com/MCPRUNNER/gossisMCP/pkg/formatter"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/analysis"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/extraction"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig        `json:"server" yaml:"server"`
	Packages PackageConfig       `json:"packages" yaml:"packages"`
	Logging  LoggingConfig       `json:"logging" yaml:"logging"`
	Plugins  config.PluginConfig `json:"plugins" yaml:"plugins"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	HTTPMode bool   `json:"http_mode" yaml:"http_mode"`
	Port     string `json:"port" yaml:"port"`
}

// PackageConfig holds package directory configuration
type PackageConfig struct {
	Directory string `json:"directory" yaml:"directory"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `json:"level" yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			HTTPMode: false,
			Port:     "8086",
		},
		Packages: PackageConfig{
			Directory: "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Plugins: DefaultPluginConfig(),
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (Config, error) {
	config := DefaultConfig()

	// Load from config file if specified
	if configPath != "" {
		if err := loadConfigFile(configPath, &config); err != nil {
			return config, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Load environment-specific overrides
	if err := loadEnvironmentConfig(&config); err != nil {
		return config, fmt.Errorf("failed to load environment config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return config, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadConfigFile loads configuration from JSON or YAML file
func loadConfigFile(configPath string, config *Config) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Try JSON first
	if err := json.Unmarshal(data, config); err != nil {
		// Try YAML
		if yamlErr := yaml.Unmarshal(data, config); yamlErr != nil {
			return fmt.Errorf("failed to parse config as JSON or YAML: %v, %v", err, yamlErr)
		}
	}

	return nil
}

// loadEnvironmentConfig loads configuration overrides from environment variables
func loadEnvironmentConfig(config *Config) error {
	// Server configuration
	if port := os.Getenv("GOSSIS_HTTP_PORT"); port != "" {
		config.Server.Port = port
	}

	// Package directory
	if pkgDir := os.Getenv("GOSSIS_PKG_DIRECTORY"); pkgDir != "" {
		config.Packages.Directory = pkgDir
	}

	// Logging configuration
	if logLevel := os.Getenv("GOSSIS_LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("GOSSIS_LOG_FORMAT"); logFormat != "" {
		config.Logging.Format = logFormat
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config Config) error {
	// Validate port
	if config.Server.Port != "" {
		if port, err := strconv.Atoi(config.Server.Port); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid server port: %s", config.Server.Port)
		}
	}

	// Validate package directory if specified
	if config.Packages.Directory != "" {
		if _, err := os.Stat(config.Packages.Directory); os.IsNotExist(err) {
			return fmt.Errorf("package directory does not exist: %s", config.Packages.Directory)
		}
	}

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(config.Logging.Level)] {
		return fmt.Errorf("invalid log level: %s", config.Logging.Level)
	}

	// Validate log format
	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[strings.ToLower(config.Logging.Format)] {
		return fmt.Errorf("invalid log format: %s", config.Logging.Format)
	}

	return nil
}

// configureLogging configures the logging based on the configuration
func configureLogging(config LoggingConfig) {
	// Set log level (simplified - in a real implementation you'd use a proper logging library)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if strings.ToLower(config.Level) == "debug" {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	}
}

// mergeConfigs merges two configurations (for future use with multiple config files)
func mergeConfigs(base, override Config) Config {
	result := base

	// Merge server config
	if override.Server.Port != "" {
		result.Server.Port = override.Server.Port
	}
	if override.Server.HTTPMode {
		result.Server.HTTPMode = override.Server.HTTPMode
	}

	// Merge package config
	if override.Packages.Directory != "" {
		result.Packages.Directory = override.Packages.Directory
	}

	// Merge logging config
	if override.Logging.Level != "" {
		result.Logging.Level = override.Logging.Level
	}
	if override.Logging.Format != "" {
		result.Logging.Format = override.Logging.Format
	}

	return result
}

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

// Unified source component analysis handler
func handleAnalyzeSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sourceType, err := request.RequireString("source_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Map source types to ComponentClassIDs
	sourceTypeMap := map[string]string{
		"ole_db":    "Microsoft.OLEDBSource",
		"ado_net":   "Microsoft.SqlServer.Dts.Pipeline.DataReaderSourceAdapter",
		"odbc":      "Microsoft.SqlServer.Dts.Pipeline.OdbcSourceAdapter",
		"flat_file": "Microsoft.SqlServer.Dts.Pipeline.FlatFileSourceAdapter",
		"excel":     "Microsoft.SqlServer.Dts.Pipeline.ExcelSourceAdapter",
		"access":    "Microsoft.SqlServer.Dts.Pipeline.AccessSourceAdapter",
		"xml":       "Microsoft.SqlServer.Dts.Pipeline.XmlSourceAdapter",
		"raw_file":  "Microsoft.SqlServer.Dts.Pipeline.RawFileSourceAdapter",
		"cdc":       "Microsoft.SqlServer.Dts.Pipeline.CdcSourceAdapter",
		"sap_bw":    "Microsoft.SqlServer.Dts.Pipeline.SapBwSourceAdapter",
	}

	componentClassID, exists := sourceTypeMap[sourceType]
	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Unknown source type: %s. Supported types: ole_db, ado_net, odbc, flat_file, excel, access, xml, raw_file, cdc, sap_bw", sourceType)), nil
	}

	// Map source types to display names
	sourceNameMap := map[string]string{
		"ole_db":    "OLE DB Source",
		"ado_net":   "ADO.NET Source",
		"odbc":      "ODBC Source",
		"flat_file": "Flat File Source",
		"excel":     "Excel Source",
		"access":    "Access Source",
		"xml":       "XML Source",
		"raw_file":  "Raw File Source",
		"cdc":       "CDC Source",
		"sap_bw":    "SAP BW Source",
	}

	displayName := sourceNameMap[sourceType]

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s Analysis:\n\n", displayName))

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == componentClassID {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Output Columns
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString(fmt.Sprintf("No %s components found in this package.\n", displayName))
	}

	return mcp.NewToolResultText(result.String()), nil
}

// Unified destination component analysis handler
func handleAnalyzeDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	destinationType, err := request.RequireString("destination_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Map destination types to ComponentClassIDs
	destinationTypeMap := map[string]string{
		"ole_db":     "Microsoft.SqlServer.Dts.Pipeline.OLEDBDestinationAdapter",
		"flat_file":  "Microsoft.SqlServer.Dts.Pipeline.FlatFileDestinationAdapter",
		"sql_server": "Microsoft.SqlServer.Dts.Pipeline.SqlServerDestinationAdapter",
		"excel":      "Microsoft.SqlServer.Dts.Pipeline.ExcelDestinationAdapter",
		"raw_file":   "Microsoft.SqlServer.Dts.Pipeline.RawFileDestinationAdapter",
	}

	componentClassID, exists := destinationTypeMap[destinationType]
	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Unknown destination type: %s. Supported types: ole_db, flat_file, sql_server, excel, raw_file", destinationType)), nil
	}

	// Map destination types to display names
	destinationNameMap := map[string]string{
		"ole_db":     "OLE DB Destination",
		"flat_file":  "Flat File Destination",
		"sql_server": "SQL Server Destination",
		"excel":      "Excel Destination",
		"raw_file":   "Raw File Destination",
	}

	displayName := destinationNameMap[destinationType]

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s Analysis:\n\n", displayName))

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == componentClassID {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Input Columns
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString(fmt.Sprintf("No %s components found in this package.\n", displayName))
	}

	return mcp.NewToolResultText(result.String()), nil
}

// Output Format System is now in pkg/formatter

func handleToolWithFormat(request mcp.CallToolRequest, toolName string, dataFunc func() (interface{}, error)) *mcp.CallToolResult {
	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	// Get file path for error reporting (may be empty for some tools)
	filePath := request.GetString("file_path", "")

	data, err := dataFunc()
	result := formatter.CreateAnalysisResult(toolName, filePath, data, err)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format))
}

// Batch analysis data structures
type BatchAnalysisResult struct {
	PackagePath    string                 `json:"package_path"`
	Success        bool                   `json:"success"`
	Error          string                 `json:"error,omitempty"`
	AnalysisResult map[string]interface{} `json:"analysis_result,omitempty"`
	Duration       time.Duration          `json:"duration"`
}

type BatchSummary struct {
	TotalPackages    int                   `json:"total_packages"`
	Successful       int                   `json:"successful"`
	Failed           int                   `json:"failed"`
	TotalDuration    time.Duration         `json:"total_duration"`
	AverageDuration  time.Duration         `json:"average_duration"`
	Errors           []string              `json:"errors,omitempty"`
	PackageSummaries []BatchAnalysisResult `json:"package_summaries"`
}

func main() {
	// Command line flags
	httpMode := flag.Bool("http", false, "Run in HTTP streaming mode")
	httpPort := flag.String("port", "8086", "HTTP server port")
	pkgDir := flag.String("pkg-dir", "", "Root directory for SSIS packages (can also be set via GOSSIS_PKG_DIRECTORY env var, defaults to current working directory)")
	configPath := flag.String("config", "", "Path to configuration file (JSON or YAML)")
	flag.Parse()

	// Load configuration
	config, err := LoadConfig(*configPath)
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
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
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
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
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
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeMessageQueueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeMessageQueueTasks(ctx, request, packageDirectory)
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
	)
	s.AddTool(analyzeScriptTaskTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeScriptTask(ctx, request, packageDirectory)
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
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeLoggingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeLoggingConfiguration(ctx, request, packageDirectory)
	})

	// Tool to list all DTSX packages in the package directory
	listPackagesTool := mcp.NewTool("list_packages",
		mcp.WithDescription("Recursively list all DTSX packages found in the package directory"),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
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
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
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
	)
	s.AddTool(analyzeDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeDestination(ctx, request, packageDirectory)
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
	)
	s.AddTool(analyzeOLEDBSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeOLEDBSource(ctx, request, packageDirectory)
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
	)
	s.AddTool(analyzeDataConversionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDataConversion(ctx, request, packageDirectory)
	})

	// Tool to analyze ADO.NET Source components
	analyzeADONETSourceTool := mcp.NewTool("analyze_ado_net_source",
		mcp.WithDescription("Analyze ADO.NET Source components in a DTSX file, extracting connection details and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeADONETSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeADONETSource(ctx, request, packageDirectory)
	})

	// Tool to analyze ODBC Source components
	analyzeODBCSourceTool := mcp.NewTool("analyze_odbc_source",
		mcp.WithDescription("Analyze ODBC Source components in a DTSX file, extracting connection details and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeODBCSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeODBCSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Flat File Source components
	analyzeFlatFileSourceTool := mcp.NewTool("analyze_flat_file_source",
		mcp.WithDescription("Analyze Flat File Source components in a DTSX file, extracting file connection details and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeFlatFileSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeFlatFileSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Excel Source components
	analyzeExcelSourceTool := mcp.NewTool("analyze_excel_source",
		mcp.WithDescription("Analyze Excel Source components in a DTSX file, extracting Excel file details, sheet names, and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeExcelSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeExcelSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Access Source components
	analyzeAccessSourceTool := mcp.NewTool("analyze_access_source",
		mcp.WithDescription("Analyze Access Source components in a DTSX file, extracting database connection details and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeAccessSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeAccessSource(ctx, request, packageDirectory)
	})

	// Tool to analyze XML Source components
	analyzeXMLSourceTool := mcp.NewTool("analyze_xml_source",
		mcp.WithDescription("Analyze XML Source components in a DTSX file, extracting XML structure details and output columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeXMLSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeXMLSource(ctx, request, packageDirectory)
	})

	// Tool to analyze Raw File Source components
	analyzeRawFileSourceTool := mcp.NewTool("analyze_raw_file_source",
		mcp.WithDescription("Analyze Raw File Source components in a DTSX file, extracting file metadata and column structure"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeRawFileSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeRawFileSource(ctx, request, packageDirectory)
	})

	// Tool to analyze CDC Source components
	analyzeCDCSourceTool := mcp.NewTool("analyze_cdc_source",
		mcp.WithDescription("Analyze CDC Source components in a DTSX file, extracting CDC configuration and change tracking details"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeCDCSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeCDCSource(ctx, request, packageDirectory)
	})

	// Tool to analyze SAP BW Source components
	analyzeSAPBWSourceTool := mcp.NewTool("analyze_sap_bw_source",
		mcp.WithDescription("Analyze SAP BW Source components in a DTSX file, extracting SAP BW integration details and InfoObject mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeSAPBWSourceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSAPBWSource(ctx, request, packageDirectory)
	})

	// Tool to analyze OLE DB Destination components
	analyzeOLEDBDestinationTool := mcp.NewTool("analyze_ole_db_destination",
		mcp.WithDescription("Analyze OLE DB Destination components in a DTSX file, extracting target table mappings and bulk load settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
	)
	s.AddTool(analyzeOLEDBDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeOLEDBDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze Flat File Destination components
	analyzeFlatFileDestinationTool := mcp.NewTool("analyze_flat_file_destination",
		mcp.WithDescription("Analyze Flat File Destination components in a DTSX file, extracting file format settings and column mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeFlatFileDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeFlatFileDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze SQL Server Destination components
	analyzeSQLServerDestinationTool := mcp.NewTool("analyze_sql_server_destination",
		mcp.WithDescription("Analyze SQL Server Destination components in a DTSX file, extracting bulk insert configuration and performance settings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeSQLServerDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSQLServerDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze Derived Column components
	analyzeDerivedColumnTool := mcp.NewTool("analyze_derived_column",
		mcp.WithDescription("Analyze Derived Column components in a DTSX file, extracting expressions and data transformations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeDerivedColumnTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeDerivedColumn(ctx, request, packageDirectory)
	})

	// Tool to analyze Lookup components
	analyzeLookupTool := mcp.NewTool("analyze_lookup",
		mcp.WithDescription("Analyze Lookup components in a DTSX file, extracting reference table joins and cache configuration"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeLookupTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeLookup(ctx, request, packageDirectory)
	})

	// Tool to analyze Conditional Split components
	analyzeConditionalSplitTool := mcp.NewTool("analyze_conditional_split",
		mcp.WithDescription("Analyze Conditional Split components in a DTSX file, extracting split conditions and output configurations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeConditionalSplitTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeConditionalSplit(ctx, request, packageDirectory)
	})

	// Tool to analyze Sort components
	analyzeSortTool := mcp.NewTool("analyze_sort",
		mcp.WithDescription("Analyze Sort transform components in a DTSX file, extracting sort keys and memory usage"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeSortTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeSort(ctx, request, packageDirectory)
	})

	// Tool to analyze Aggregate components
	analyzeAggregateTool := mcp.NewTool("analyze_aggregate",
		mcp.WithDescription("Analyze Aggregate transform components in a DTSX file, extracting aggregation operations and group by columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeAggregateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeAggregate(ctx, request, packageDirectory)
	})

	// Tool to analyze Merge Join components
	analyzeMergeJoinTool := mcp.NewTool("analyze_merge_join",
		mcp.WithDescription("Analyze Merge Join transform components in a DTSX file, extracting join type and key columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeMergeJoinTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeMergeJoin(ctx, request, packageDirectory)
	})

	// Tool to analyze Union All components
	analyzeUnionAllTool := mcp.NewTool("analyze_union_all",
		mcp.WithDescription("Analyze Union All transform components in a DTSX file, extracting input/output column mappings"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeUnionAllTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeUnionAll(ctx, request, packageDirectory)
	})

	// Tool to analyze Multicast components
	analyzeMulticastTool := mcp.NewTool("analyze_multicast",
		mcp.WithDescription("Analyze Multicast transform components in a DTSX file, extracting output configurations"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeMulticastTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeMulticast(ctx, request, packageDirectory)
	})

	// Tool to analyze Script Component components
	analyzeScriptComponentTool := mcp.NewTool("analyze_script_component",
		mcp.WithDescription("Analyze Script Component transform components in a DTSX file, extracting script code and configuration"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeScriptComponentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeScriptComponent(ctx, request, packageDirectory)
	})

	// Tool to analyze Excel Destination components
	analyzeExcelDestinationTool := mcp.NewTool("analyze_excel_destination",
		mcp.WithDescription("Analyze Excel Destination components in a DTSX file, extracting sheet configuration and data type mapping"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeExcelDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeExcelDestination(ctx, request, packageDirectory)
	})

	// Tool to analyze Raw File Destination components
	analyzeRawFileDestinationTool := mcp.NewTool("analyze_raw_file_destination",
		mcp.WithDescription("Analyze Raw File Destination components in a DTSX file, extracting file metadata and write options"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set, or absolute path)"),
		),
	)
	s.AddTool(analyzeRawFileDestinationTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeRawFileDestination(ctx, request, packageDirectory)
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
		return analysis.HandleAnalyzeEventHandlers(ctx, request, packageDirectory)
	})

	// Tool to analyze package dependencies
	analyzePackageDependenciesTool := mcp.NewTool("analyze_package_dependencies",
		mcp.WithDescription("Analyze relationships between packages, shared connections, and variables across multiple DTSX files"),
	)
	s.AddTool(analyzePackageDependenciesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzePackageDependencies(ctx, request, packageDirectory)
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
		return analysis.HandleAnalyzeConfigurations(ctx, request, packageDirectory)
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
		return analysis.HandleAnalyzePerformanceMetrics(ctx, request, packageDirectory)
	})

	detectSecurityIssuesTool := mcp.NewTool("detect_security_issues",
		mcp.WithDescription("Detect potential security issues (hardcoded credentials, sensitive data exposure)"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(detectSecurityIssuesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleDetectSecurityIssues(ctx, request, packageDirectory)
	})

	// Tool to analyze Pivot transformations
	analyzePivotTool := mcp.NewTool("analyze_pivot",
		mcp.WithDescription("Analyze Pivot transformations in a DTSX file, extracting pivot keys, set keys, and value columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzePivotTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzePivot(ctx, request, packageDirectory)
	})

	// Tool to analyze Unpivot transformations
	analyzeUnpivotTool := mcp.NewTool("analyze_unpivot",
		mcp.WithDescription("Analyze Unpivot transformations in a DTSX file, extracting pivot key values and destination columns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeUnpivotTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeUnpivot(ctx, request, packageDirectory)
	})

	// Tool to analyze Term Extraction transformations
	analyzeTermExtractionTool := mcp.NewTool("analyze_term_extraction",
		mcp.WithDescription("Analyze Term Extraction transformations in a DTSX file, extracting term extraction settings and exclusion lists"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeTermExtractionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeTermExtraction(ctx, request, packageDirectory)
	})

	// Tool to analyze Fuzzy Lookup transformations
	analyzeFuzzyLookupTool := mcp.NewTool("analyze_fuzzy_lookup",
		mcp.WithDescription("Analyze Fuzzy Lookup transformations in a DTSX file, extracting reference table settings and similarity thresholds"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeFuzzyLookupTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeFuzzyLookup(ctx, request, packageDirectory)
	})

	// Tool to analyze Fuzzy Grouping transformations
	analyzeFuzzyGroupingTool := mcp.NewTool("analyze_fuzzy_grouping",
		mcp.WithDescription("Analyze Fuzzy Grouping transformations in a DTSX file, extracting grouping keys and similarity thresholds"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(analyzeFuzzyGroupingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return analysis.HandleAnalyzeFuzzyGrouping(ctx, request, packageDirectory)
	})

	// Tool to analyze Row Count transformations
	analyzeRowCountTool := mcp.NewTool("analyze_row_count",
		mcp.WithDescription("Analyze Row Count transformations in a DTSX file, extracting variable assignments and result storage"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
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
	)
	s.AddTool(comparePackagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleComparePackages(ctx, request, packageDirectory)
	})

	analyzeCodeQualityTool := mcp.NewTool("mcp_ssis-analyzer_analyze_code_quality",
		mcp.WithDescription("Calculate maintainability metrics (complexity, duplication, etc.) to assess package quality and technical debt"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
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
	)
	s.AddTool(readTextFileTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleReadTextFile(ctx, request, packageDirectory)
	})

	// Advanced Security Features - Phase 2

	// Tool for comprehensive credential scanning with pattern matching
	scanCredentialsTool := mcp.NewTool("scan_credentials",
		mcp.WithDescription("Perform comprehensive credential scanning with advanced pattern matching to detect hardcoded credentials, API keys, tokens, and sensitive data patterns"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(scanCredentialsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleScanCredentials(ctx, request, packageDirectory)
	})

	// Tool for encryption detection and recommendations
	detectEncryptionTool := mcp.NewTool("detect_encryption",
		mcp.WithDescription("Detect encryption settings and provide recommendations for securing sensitive data in SSIS packages"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(detectEncryptionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDetectEncryption(ctx, request, packageDirectory)
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
	)
	s.AddTool(checkComplianceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleCheckCompliance(ctx, request, packageDirectory)
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
	)
	s.AddTool(optimizeBufferSizeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleOptimizeBufferSize(ctx, request, packageDirectory)
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
	)
	s.AddTool(analyzeParallelProcessingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleAnalyzeParallelProcessing(ctx, request, packageDirectory)
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
	)
	s.AddTool(profileMemoryUsageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleProfileMemoryUsage(ctx, request, packageDirectory)
	})

	// Batch Processing Tools

	// Batch analysis handler
	handleBatchAnalyze := func(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments"), nil
		}

		filePaths, ok := args["file_paths"].([]interface{})
		if !ok {
			return mcp.NewToolResultError("file_paths parameter is required and must be an array"), nil
		}

		format := "text"
		if f, ok := args["format"].(string); ok && f != "" {
			format = f
		}

		maxConcurrency := 4 // Default concurrency limit (matches tool definition)
		if mc, ok := args["max_concurrent"].(float64); ok && mc > 0 {
			maxConcurrency = int(mc)
		}

		var paths []string
		for _, p := range filePaths {
			if pathStr, ok := p.(string); ok {
				paths = append(paths, pathStr)
			}
		}

		if len(paths) == 0 {
			return mcp.NewToolResultError("no valid file paths provided"), nil
		}

		// Create semaphore for concurrency control
		sem := make(chan struct{}, maxConcurrency)
		results := make(chan BatchAnalysisResult, len(paths))
		startTime := time.Now()

		// Launch goroutines for parallel processing
		for _, path := range paths {
			go func(filePath string) {
				sem <- struct{}{}        // Acquire semaphore
				defer func() { <-sem }() // Release semaphore

				result := BatchAnalysisResult{
					PackagePath: filePath,
				}
				resultStart := time.Now()

				// Perform analysis (simplified - you might want to call specific analysis functions)
				analysisResult, err := performPackageAnalysis(filePath, packageDirectory)
				result.Duration = time.Since(resultStart)

				if err != nil {
					result.Success = false
					result.Error = err.Error()
				} else {
					result.Success = true
					result.AnalysisResult = analysisResult
				}

				results <- result
			}(path)
		}

		// Collect results
		var batchResults []BatchAnalysisResult
		for i := 0; i < len(paths); i++ {
			select {
			case result := <-results:
				batchResults = append(batchResults, result)
			case <-ctx.Done():
				return mcp.NewToolResultError("batch analysis cancelled"), nil
			}
		}

		// Calculate summary
		totalDuration := time.Since(startTime)
		successful := 0
		failed := 0
		var errors []string

		for _, result := range batchResults {
			if result.Success {
				successful++
			} else {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", result.PackagePath, result.Error))
			}
		}

		summary := BatchSummary{
			TotalPackages:    len(paths),
			Successful:       successful,
			Failed:           failed,
			TotalDuration:    totalDuration,
			AverageDuration:  totalDuration / time.Duration(len(paths)),
			Errors:           errors,
			PackageSummaries: batchResults,
		}

		// Format output
		var output string
		switch format {
		case "json":
			jsonData, _ := json.MarshalIndent(summary, "", "  ")
			output = string(jsonData)
		case "csv":
			output = formatBatchSummaryAsCSV(summary)
		case "html":
			output = formatBatchSummaryAsHTML(summary)
		case "markdown":
			output = formatBatchSummaryAsMarkdown(summary)
		default:
			output = formatBatchSummaryAsText(summary)
		}

		return mcp.NewToolResultText(output), nil
	}

	// Tool for batch analysis of multiple DTSX files
	batchAnalyzeTool := mcp.NewTool("batch_analyze",
		mcp.WithDescription("Analyze multiple DTSX files in parallel and provide aggregated results"),
		mcp.WithArray("file_paths",
			mcp.Required(),
			mcp.Description("Array of DTSX file paths to analyze (relative to package directory if set)"),
			mcp.WithStringItems(),
		),
		mcp.WithString("format",
			mcp.Description("Output format: text, json, csv, html, markdown (default: text)"),
		),
		mcp.WithNumber("max_concurrent",
			mcp.Description("Maximum number of concurrent analyses (default: 4)"),
		),
	)
	s.AddTool(batchAnalyzeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleBatchAnalyze(ctx, request, packageDirectory)
	})

	if config.Server.HTTPMode {
		// Run in HTTP streaming mode
		runHTTPServer(s, config.Server.Port)
	} else {
		// Run in stdio mode (default)
		if err := server.ServeStdio(s); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}
}

func handleParseDtsx(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("parse_dtsx", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("parse_dtsx", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Create structured data for the result
	summaryData := map[string]interface{}{
		"properties_count":  len(pkg.Properties),
		"connections_count": len(pkg.ConnectionMgr.Connections),
		"tasks_count":       len(pkg.Executables.Tasks),
		"variables_count":   len(pkg.Variables.Vars),
	}

	// Add package properties
	properties := make([]map[string]string, 0)
	for _, prop := range pkg.Properties {
		if prop.Name == "Name" || prop.Name == "Description" {
			properties = append(properties, map[string]string{
				"name":  prop.Name,
				"value": strings.TrimSpace(prop.Value),
			})
		}
	}
	summaryData["properties"] = properties

	// Add variables
	variables := make([]map[string]string, 0)
	for _, v := range pkg.Variables.Vars {
		variables = append(variables, map[string]string{
			"name":  v.Name,
			"value": v.Value,
		})
	}
	summaryData["variables"] = variables

	result := formatter.CreateAnalysisResult("parse_dtsx", filePath, summaryData, nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
}

func handleExtractTasks(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleExtractConnections(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleExtractPrecedenceConstraints(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleValidateDtsx(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, err := os.ReadFile(filePath)
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

func handleExtractVariables(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleExtractParameters(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleExtractScriptCode(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleValidateBestPractices(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleAskAboutDtsx(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
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

	data, err := os.ReadFile(resolvedPath)
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

func handleAnalyzeMessageQueueTasks(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleAnalyzeScriptTask(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	analysis := "Script Tasks Analysis:\n"
	found := false

	for i, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "script") {
			found = true
			analysis += fmt.Sprintf("Task %d: %s\n", i+1, task.Name)

			// Task description
			for _, prop := range task.Properties {
				if prop.Name == "Description" {
					analysis += fmt.Sprintf("  Description: %s\n", strings.TrimSpace(prop.Value))
				}
			}

			// Script task specific properties
			scriptTaskData := task.ObjectData.ScriptTask.ScriptTaskData

			// Extract script code
			if scriptTaskData.ScriptProject.ScriptCode != "" {
				analysis += "  Script Code:\n"
				code := strings.TrimSpace(scriptTaskData.ScriptProject.ScriptCode)
				code = strings.ReplaceAll(code, "&lt;", "<")
				code = strings.ReplaceAll(code, "&gt;", ">")
				code = strings.ReplaceAll(code, "&amp;", "&")
				analysis += fmt.Sprintf("    %s\n", code)
			} else {
				analysis += "  Script Code: No script code found\n"
			}

			// Look for additional script task properties in the raw XML
			// Since our struct is limited, we'll parse some additional properties from the raw data
			rawData := string(data)
			taskStart := strings.Index(rawData, fmt.Sprintf("<Executable Name=\"%s\"", task.Name))
			if taskStart != -1 {
				taskEnd := strings.Index(rawData[taskStart:], "</Executable>")
				if taskEnd != -1 {
					taskXML := rawData[taskStart : taskStart+taskEnd+len("</Executable>")]

					// Extract ReadOnlyVariables
					if strings.Contains(taskXML, "ReadOnlyVariables") {
						roVars := extractPropertyValue(taskXML, "ReadOnlyVariables")
						if roVars != "" {
							analysis += fmt.Sprintf("  ReadOnly Variables: %s\n", roVars)
						}
					}

					// Extract ReadWriteVariables
					if strings.Contains(taskXML, "ReadWriteVariables") {
						rwVars := extractPropertyValue(taskXML, "ReadWriteVariables")
						if rwVars != "" {
							analysis += fmt.Sprintf("  ReadWrite Variables: %s\n", rwVars)
						}
					}

					// Extract EntryPoint
					if strings.Contains(taskXML, "EntryPoint") {
						entryPoint := extractPropertyValue(taskXML, "EntryPoint")
						if entryPoint != "" {
							analysis += fmt.Sprintf("  Entry Point: %s\n", entryPoint)
						}
					}

					// Extract ScriptLanguage
					if strings.Contains(taskXML, "ScriptLanguage") {
						scriptLang := extractPropertyValue(taskXML, "ScriptLanguage")
						if scriptLang != "" {
							analysis += fmt.Sprintf("  Script Language: %s\n", scriptLang)
						}
					}
				}
			}

			analysis += "\n"
		}
	}

	if !found {
		analysis += "No Script Tasks found in this package.\n"
	}

	return mcp.NewToolResultText(analysis), nil
}

func handleDetectHardcodedValues(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

func handleAnalyzeLoggingConfiguration(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
		report += "âŒ No logging configuration found in this package.\n"
		return mcp.NewToolResultText(report), nil
	}

	report += "âœ… Logging is configured in this package.\n\n"

	// Extract log providers
	logProvidersSection := extractSection(string(data), "<LogProviders>", "</LogProviders>")
	if logProvidersSection != "" {
		report += "ðŸ“‹ Log Providers:\n"
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
		report += "âš™ï¸ Package-Level Logging Settings:\n"

		// Extract logging mode
		if strings.Contains(loggingOptionsSection, `LoggingMode="1"`) {
			report += "  â€¢ Logging Mode: Enabled\n"
		} else {
			report += "  â€¢ Logging Mode: Disabled\n"
		}

		// Extract event filters
		eventFilterMatch := regexp.MustCompile(`EventFilter">([^<]+)</`)
		if matches := eventFilterMatch.FindStringSubmatch(loggingOptionsSection); len(matches) > 1 {
			report += fmt.Sprintf("  â€¢ Events Logged: %s\n", matches[1])
		}

		// Extract selected log providers
		selectedProvidersMatch := regexp.MustCompile(`SelectedLogProvider[^}]*InstanceID="([^"]*)"`)
		if matches := selectedProvidersMatch.FindAllStringSubmatch(loggingOptionsSection, -1); len(matches) > 0 {
			report += "  â€¢ Selected Providers:\n"
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
		report += "ðŸ”§ Task-Level Logging Overrides:\n"
		report += fmt.Sprintf("  â€¢ %d tasks have custom logging settings\n", taskLoggingCount-1)
		report += "  â€¢ Tasks inherit package-level logging unless explicitly overridden\n\n"
	}

	// Provide recommendations
	report += "ðŸ’¡ Recommendations:\n"
	if strings.Contains(string(data), `LoggingMode="1"`) {
		report += "  â€¢ âœ… Logging is properly enabled\n"
		if strings.Contains(string(data), "OnError") {
			report += "  â€¢ âœ… Error logging is configured\n"
		}
		if strings.Contains(string(data), "Microsoft.LogProviderSQLServer") {
			report += "  â€¢ âœ… SQL Server logging provides good audit trail\n"
		}
	} else {
		report += "  â€¢ âš ï¸ Consider enabling logging for better monitoring and troubleshooting\n"
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

func handleListPackages(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
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

func handleAnalyzeDataFlow(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	fullPath := resolveFilePath(filePath, packageDirectory)

	// Read the DTSX file as string for analysis
	data, err := os.ReadFile(fullPath)
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

			result.WriteString(fmt.Sprintf("  - %s: %s â†’ %s\n", pathName, startID, endID))
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func extractComponentName(fullID string) string {
	if idx := strings.LastIndex(fullID, "\\"); idx > 0 {
		return fullID[idx+1:]
	}
	return fullID
}

func handleAnalyzeDataFlowDetailed(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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

	var result strings.Builder
	result.WriteString("Detailed Data Flow Analysis:\n\n")

	// Find the data flow task
	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			found = true
			result.WriteString(fmt.Sprintf("Data Flow Task: %s\n", task.Name))
			result.WriteString(fmt.Sprintf("Description: %s\n\n", task.Description))

			// Get components
			components := task.ObjectData.DataFlow.Components.Components
			if len(components) == 0 {
				result.WriteString("No components found.\n")
				break
			}

			result.WriteString("Components:\n")
			for _, comp := range components {
				result.WriteString(fmt.Sprintf("\nComponent: %s\n", comp.Name))
				result.WriteString(fmt.Sprintf("  Type: %s\n", getComponentType(comp.ComponentClassID)))
				if comp.Description != "" {
					result.WriteString(fmt.Sprintf("  Description: %s\n", comp.Description))
				}

				// Properties
				if len(comp.ObjectData.PipelineComponent.Properties.Properties) > 0 {
					result.WriteString("  Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("    %s: %s\n", prop.Name, prop.Value))
					}
				}

				// Inputs
				if len(comp.Inputs.Inputs) > 0 {
					result.WriteString("  Inputs:\n")
					for _, input := range comp.Inputs.Inputs {
						result.WriteString(fmt.Sprintf("    Input: %s\n", input.Name))
						if len(input.InputColumns.Columns) > 0 {
							result.WriteString("      Columns:\n")
							for _, col := range input.InputColumns.Columns {
								result.WriteString(fmt.Sprintf("        %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
				}

				// Outputs
				if len(comp.Outputs.Outputs) > 0 {
					result.WriteString("  Outputs:\n")
					for _, output := range comp.Outputs.Outputs {
						result.WriteString(fmt.Sprintf("    Output: %s", output.Name))
						if output.IsErrorOut {
							result.WriteString(" (Error Output)")
						}
						result.WriteString("\n")
						if len(output.OutputColumns.Columns) > 0 {
							result.WriteString("      Columns:\n")
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("        %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
				}
			}

			// Data Paths
			paths := task.ObjectData.DataFlow.Paths.Paths
			if len(paths) > 0 {
				result.WriteString("\nData Paths:\n")
				for _, path := range paths {
					result.WriteString(fmt.Sprintf("  %s: %s â†’ %s\n", path.Name, extractComponentName(path.StartID), extractComponentName(path.EndID)))
				}
			}

			break
		}
	}

	if !found {
		result.WriteString("No Data Flow Tasks found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeOLEDBSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("OLE DB Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.OLEDBSource" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Connections
					result.WriteString("Connections:\n")
					// Note: Connections are not in the struct yet, but we can add later

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No OLE DB Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeExportColumn(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Export Column Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.Extractor" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Export Column components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeDataConversion(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Data Conversion Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.DataConvert" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Data Conversion components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeADONETSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("ADO.NET Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.DataReaderSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No ADO.NET Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeODBCSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("ODBC Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.OdbcSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No ODBC Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeFlatFileSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Flat File Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FlatFileSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Flat File Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeExcelSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Excel Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ExcelSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Excel Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeAccessSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Access Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.AccessSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Access Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeXMLSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("XML Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.XmlSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No XML Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeRawFileSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Raw File Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.RawFileSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Raw File Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeCDCSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("CDC Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.CdcSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No CDC Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeSAPBWSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("SAP BW Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.SapBwSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No SAP BW Source components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeOLEDBDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("OLE DB Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.OLEDBDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No OLE DB Destination components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeFlatFileDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Flat File Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FlatFileDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Flat File Destination components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeSQLServerDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("SQL Server Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.SqlServerDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No SQL Server Destination components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeDerivedColumn(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Derived Column Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.DerivedColumn" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  Output: %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Derived Column components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeLookup(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Lookup Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Lookup" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  Output: %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Lookup components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeConditionalSplit(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Conditional Split Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ConditionalSplit" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Conditional Split components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeSort(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Sort Transform Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Sort" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Sort components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeAggregate(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Aggregate Transform Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Aggregate" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Aggregate components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeMergeJoin(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Merge Join Transform Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.MergeJoin" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Merge Join components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeUnionAll(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Union All Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.UnionAll" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					inputCount := 0
					for _, input := range comp.Inputs.Inputs {
						inputCount++
						result.WriteString(fmt.Sprintf("  Input %d:\n", inputCount))
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString("  Output:\n")
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Union All components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeMulticast(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Multicast Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Multicast" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Configuration:\n")
					for _, input := range comp.Inputs.Inputs {
						result.WriteString("  Input:\n")
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					outputCount := 0
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							outputCount++
							result.WriteString(fmt.Sprintf("  Output %d:\n", outputCount))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString(fmt.Sprintf("Total Outputs: %d\n", outputCount))
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Multicast components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeScriptComponent(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Script Component Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ScriptComponent" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) || prop.Name == "ScriptLanguage" || prop.Name == "ReadOnlyVariables" || prop.Name == "ReadWriteVariables" {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}

					// Script code (if available in properties)
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if prop.Name == "SourceCode" || prop.Name == "ScriptCode" {
							result.WriteString("Script Code:\n")
							result.WriteString(fmt.Sprintf("  %s\n", prop.Value))
						}
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						result.WriteString("  Input:\n")
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString("  Output:\n")
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Script Component components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeExcelDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Excel Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ExcelDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Excel Destination components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeRawFileDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Raw File Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.RawFileDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Raw File Destination components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeEventHandlers(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	fullPath := resolveFilePath(filePath, packageDirectory)

	// Read and parse the DTSX file
	data, err := os.ReadFile(fullPath)
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
				result.WriteString(fmt.Sprintf("     - %s â†’ %s", constraint.From, constraint.To))
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

func handleAnalyzePackageDependencies(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
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
		data, err := os.ReadFile(filePath)
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
	result.WriteString("ðŸ”— Shared Connections:\n")
	sharedConnections := 0
	for _, conn := range connections {
		if len(conn.Packages) > 1 {
			sharedConnections++
			result.WriteString(fmt.Sprintf("â€¢ %s (used by %d packages):\n", conn.Name, len(conn.Packages)))
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
	result.WriteString("ðŸ“Š Shared Variables:\n")
	sharedVariables := 0
	for _, variable := range variables {
		if len(variable.Packages) > 1 {
			sharedVariables++
			result.WriteString(fmt.Sprintf("â€¢ %s (used by %d packages):\n", variable.Name, len(variable.Packages)))
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
	result.WriteString("ðŸ“ˆ Summary:\n")
	result.WriteString(fmt.Sprintf("â€¢ Total packages analyzed: %d\n", len(dtsxFiles)))
	result.WriteString(fmt.Sprintf("â€¢ Shared connections: %d\n", sharedConnections))
	result.WriteString(fmt.Sprintf("â€¢ Shared variables: %d\n", sharedVariables))

	if sharedConnections > 0 || sharedVariables > 0 {
		result.WriteString("\nðŸ’¡ These shared resources indicate potential dependencies between packages.")
		result.WriteString("\n   Consider documenting these relationships for maintenance and deployment purposes.")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeConfigurations(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
		result.WriteString("\nðŸ’¡ Note: Configurations were used in SSIS 2005-2008 for parameterization.")
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
	result.WriteString("ðŸ“‹ Configuration Summary:\n")
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
		result.WriteString(fmt.Sprintf("â€¢ XML Configuration Files: %d\n", xmlConfigs))
	}
	if sqlConfigs > 0 {
		result.WriteString(fmt.Sprintf("â€¢ SQL Server Configurations: %d\n", sqlConfigs))
	}
	if envConfigs > 0 {
		result.WriteString(fmt.Sprintf("â€¢ Environment Variable Configurations: %d\n", envConfigs))
	}

	result.WriteString("\nðŸ’¡ Recommendations:\n")
	result.WriteString("â€¢ Consider migrating to SSIS 2012+ Parameters for better security and maintainability\n")
	result.WriteString("â€¢ XML configurations should be stored in secure locations\n")
	result.WriteString("â€¢ SQL Server configurations require appropriate database permissions\n")
	result.WriteString("â€¢ Environment variables are machine-specific and may not work across environments\n")

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzePerformanceMetrics(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ“¦ Package-Level Performance Settings:\n")
	packagePerfProps := extractPerformanceProperties(pkg.Properties, "package")
	if len(packagePerfProps) > 0 {
		for _, prop := range packagePerfProps {
			result.WriteString(fmt.Sprintf("â€¢ %s: %s\n", prop.Name, prop.Value))
			if prop.Recommendation != "" {
				result.WriteString(fmt.Sprintf("  ðŸ’¡ %s\n", prop.Recommendation))
			}
		}
	} else {
		result.WriteString("No performance-related package properties found.\n")
	}
	result.WriteString("\n")

	// Analyze data flow performance settings
	result.WriteString("ðŸ”„ Data Flow Performance Analysis:\n")
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
					result.WriteString(fmt.Sprintf("  â€¢ %s: %s\n", prop.Name, prop.Value))
					if prop.Recommendation != "" {
						result.WriteString(fmt.Sprintf("    ðŸ’¡ %s\n", prop.Recommendation))
					}
				}
			}

			// Analyze data flow components
			if task.ObjectData.DataFlow.Components.Components != nil {
				result.WriteString("  Components:\n")
				for _, comp := range task.ObjectData.DataFlow.Components.Components {
					compPerfProps := extractComponentPerformanceProperties(comp)
					if len(compPerfProps) > 0 {
						result.WriteString(fmt.Sprintf("    â€¢ %s (%s):\n", comp.Name, getComponentType(comp.ComponentClassID)))
						for _, prop := range compPerfProps {
							result.WriteString(fmt.Sprintf("      - %s: %s\n", prop.Name, prop.Value))
							if prop.Recommendation != "" {
								result.WriteString(fmt.Sprintf("        ðŸ’¡ %s\n", prop.Recommendation))
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
	result.WriteString("ðŸš€ Performance Optimization Recommendations:\n")
	result.WriteString("â€¢ Increase DefaultBufferSize if processing large datasets (recommended: 10MB+)\n")
	result.WriteString("â€¢ Adjust DefaultBufferMaxRows based on row size (recommended: 10,000-100,000)\n")
	result.WriteString("â€¢ Increase EngineThreads for parallel processing (recommended: 2-4 per CPU core)\n")
	result.WriteString("â€¢ Use BLOBTempStoragePath and BufferTempStoragePath for large datasets\n")
	result.WriteString("â€¢ Consider MaxConcurrentExecutables for parallel task execution\n")
	result.WriteString("â€¢ Monitor AutoAdjustBufferSize for optimal memory usage\n")

	return mcp.NewToolResultText(result.String()), nil
}

func handleDetectSecurityIssues(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ”’ Security Issues Analysis:\n\n")

	issuesFound := false

	// Check connection managers for hardcoded credentials
	result.WriteString("ðŸ”— Connection Managers:\n")
	connIssues := analyzeConnectionSecurity(pkg.ConnectionMgr.Connections)
	if len(connIssues) > 0 {
		issuesFound = true
		for _, issue := range connIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in connection managers.\n")
	}
	result.WriteString("\n")

	// Check variables for sensitive data
	result.WriteString("ðŸ“Š Variables:\n")
	varIssues := analyzeVariableSecurity(pkg.Variables.Vars)
	if len(varIssues) > 0 {
		issuesFound = true
		for _, issue := range varIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in variables.\n")
	}
	result.WriteString("\n")

	// Check script tasks for hardcoded credentials
	result.WriteString("ðŸ“œ Script Tasks:\n")
	scriptIssues := analyzeScriptSecurity(pkg.Executables.Tasks)
	if len(scriptIssues) > 0 {
		issuesFound = true
		for _, issue := range scriptIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in script tasks.\n")
	}
	result.WriteString("\n")

	// Check expressions for sensitive data
	result.WriteString("ðŸ” Expressions:\n")
	exprIssues := analyzeExpressionSecurity(pkg.Executables.Tasks, pkg.Variables.Vars)
	if len(exprIssues) > 0 {
		issuesFound = true
		for _, issue := range exprIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in expressions.\n")
	}
	result.WriteString("\n")

	if !issuesFound {
		result.WriteString("âœ… No security issues detected in this package.\n\n")
		result.WriteString("ðŸ’¡ Security Best Practices:\n")
		result.WriteString("â€¢ Use package parameters or environment variables for credentials\n")
		result.WriteString("â€¢ Avoid hardcoded passwords in connection strings\n")
		result.WriteString("â€¢ Use SSIS package protection levels for sensitive data\n")
		result.WriteString("â€¢ Consider using Azure Key Vault or similar for credential management\n")
	} else {
		result.WriteString("ðŸš¨ Security Recommendations:\n")
		result.WriteString("â€¢ Replace hardcoded credentials with parameters or expressions\n")
		result.WriteString("â€¢ Use SSIS package configurations for sensitive connection properties\n")
		result.WriteString("â€¢ Implement proper package protection and encryption\n")
		result.WriteString("â€¢ Review and audit access to sensitive data\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleComparePackages(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath1, err := request.RequireString("file_path1")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	filePath2, err := request.RequireString("file_path2")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve file paths
	resolvedPath1 := resolveFilePath(filePath1, packageDirectory)
	resolvedPath2 := resolveFilePath(filePath2, packageDirectory)

	// Parse first package
	data1, err := os.ReadFile(resolvedPath1)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read first file: %v", err)), nil
	}
	data1 = []byte(strings.ReplaceAll(string(data1), "DTS:", ""))
	var pkg1 SSISPackage
	if err := xml.Unmarshal(data1, &pkg1); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse first file: %v", err)), nil
	}

	// Parse second package
	data2, err := os.ReadFile(resolvedPath2)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read second file: %v", err)), nil
	}
	data2 = []byte(strings.ReplaceAll(string(data2), "DTS:", ""))
	var pkg2 SSISPackage
	if err := xml.Unmarshal(data2, &pkg2); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse second file: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("ðŸ“Š Package Comparison Report\n\n")
	result.WriteString(fmt.Sprintf("File 1: %s\n", filepath.Base(resolvedPath1)))
	result.WriteString(fmt.Sprintf("File 2: %s\n\n", filepath.Base(resolvedPath2)))

	// Compare package properties
	result.WriteString("ðŸ“‹ Package Properties:\n")
	compareProperties(pkg1.Properties, pkg2.Properties, &result)

	// Compare connections
	result.WriteString("\nðŸ”— Connection Managers:\n")
	compareConnections(pkg1.ConnectionMgr.Connections, pkg2.ConnectionMgr.Connections, &result)

	// Compare variables
	result.WriteString("\nðŸ“Š Variables:\n")
	compareVariables(pkg1.Variables.Vars, pkg2.Variables.Vars, &result)

	// Compare parameters
	result.WriteString("\nâš™ï¸ Parameters:\n")
	compareParameters(pkg1.Parameters.Params, pkg2.Parameters.Params, &result)

	// Compare configurations
	result.WriteString("\nðŸ”§ Configurations:\n")
	compareConfigurations(pkg1.Configurations.Configs, pkg2.Configurations.Configs, &result)

	// Compare tasks
	result.WriteString("\nðŸŽ¯ Tasks:\n")
	compareTasks(pkg1.Executables.Tasks, pkg2.Executables.Tasks, &result)

	// Compare event handlers
	result.WriteString("\nðŸš¨ Event Handlers:\n")
	compareEventHandlers(pkg1.EventHandlers.EventHandlers, pkg2.EventHandlers.EventHandlers, &result)

	// Compare precedence constraints
	result.WriteString("\nðŸ”€ Precedence Constraints:\n")
	comparePrecedenceConstraints(pkg1.PrecedenceConstraints.Constraints, pkg2.PrecedenceConstraints.Constraints, &result)

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeCodeQuality(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ“Š Code Quality Metrics Analysis\n\n")
	result.WriteString(fmt.Sprintf("Package: %s\n\n", filepath.Base(resolvedPath)))

	// Structural Complexity Metrics
	result.WriteString("ðŸ—ï¸ Structural Complexity:\n")
	structuralScore := calculateStructuralComplexity(pkg)
	result.WriteString(fmt.Sprintf("â€¢ Package Size Score: %d/10 (Tasks: %d, Connections: %d, Variables: %d)\n",
		structuralScore, len(pkg.Executables.Tasks), len(pkg.ConnectionMgr.Connections), len(pkg.Variables.Vars)))
	result.WriteString(fmt.Sprintf("â€¢ Control Flow Complexity: %d/10 (Precedence Constraints: %d)\n",
		calculateControlFlowComplexity(pkg), len(pkg.PrecedenceConstraints.Constraints)))

	// Script Complexity Metrics
	result.WriteString("\nðŸ“œ Script Complexity:\n")
	scriptMetrics := analyzeScriptComplexity(pkg.Executables.Tasks)
	result.WriteString(fmt.Sprintf("â€¢ Script Tasks: %d\n", scriptMetrics.ScriptTaskCount))
	result.WriteString(fmt.Sprintf("â€¢ Total Script Lines: %d\n", scriptMetrics.TotalLines))
	result.WriteString(fmt.Sprintf("â€¢ Average Script Complexity: %.1f/10\n", scriptMetrics.AverageComplexity))
	if scriptMetrics.ScriptTaskCount > 0 {
		result.WriteString(fmt.Sprintf("â€¢ Script Quality Score: %d/10\n", scriptMetrics.QualityScore))
	}

	// Expression Complexity Metrics
	result.WriteString("\nðŸ” Expression Complexity:\n")
	expressionMetrics := analyzeExpressionComplexity(pkg)
	result.WriteString(fmt.Sprintf("â€¢ Total Expressions: %d\n", expressionMetrics.TotalExpressions))
	result.WriteString(fmt.Sprintf("â€¢ Average Expression Length: %.1f characters\n", expressionMetrics.AverageLength))
	result.WriteString(fmt.Sprintf("â€¢ Expression Complexity Score: %d/10\n", expressionMetrics.ComplexityScore))

	// Variable Usage Metrics
	result.WriteString("\nðŸ“Š Variable Usage:\n")
	variableMetrics := analyzeVariableUsage(pkg)
	result.WriteString(fmt.Sprintf("â€¢ Total Variables: %d\n", variableMetrics.TotalVariables))
	result.WriteString(fmt.Sprintf("â€¢ Variables with Expressions: %d\n", variableMetrics.ExpressionsCount))
	result.WriteString(fmt.Sprintf("â€¢ Variable Usage Score: %d/10\n", variableMetrics.UsageScore))

	// Overall Maintainability Score
	result.WriteString("\nðŸŽ¯ Overall Maintainability Score:\n")
	overallScore := calculateOverallScore(structuralScore, scriptMetrics.QualityScore, expressionMetrics.ComplexityScore, variableMetrics.UsageScore)
	result.WriteString(fmt.Sprintf("â€¢ Composite Score: %d/10\n", overallScore))
	result.WriteString(fmt.Sprintf("â€¢ Rating: %s\n", getMaintainabilityRating(overallScore)))

	// Recommendations
	result.WriteString("\nðŸ’¡ Recommendations:\n")
	addQualityRecommendations(&result, overallScore, structuralScore, scriptMetrics, expressionMetrics, variableMetrics)

	return mcp.NewToolResultText(result.String()), nil
}

func handleReadTextFile(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("ðŸ“„ Text File Analysis\n\n")
	result.WriteString(fmt.Sprintf("File: %s\n", filepath.Base(resolvedPath)))
	result.WriteString(fmt.Sprintf("Path: %s\n\n", resolvedPath))

	content := string(data)
	lines := strings.Split(content, "\n")
	result.WriteString("ðŸ“Š File Statistics:\n")
	result.WriteString(fmt.Sprintf("â€¢ Total Lines: %d\n", len(lines)))
	result.WriteString(fmt.Sprintf("â€¢ Total Characters: %d\n", len(content)))
	result.WriteString(fmt.Sprintf("â€¢ File Size: %d bytes\n\n", len(data)))

	// Detect file type and parse accordingly
	ext := strings.ToLower(filepath.Ext(resolvedPath))
	switch ext {
	case ".bat", ".cmd":
		result.WriteString("ðŸ”§ Batch File Analysis:\n")
		analyzeBatchFile(content, &result)
	case ".config", ".cfg":
		result.WriteString("âš™ï¸ Configuration File Analysis:\n")
		analyzeConfigFile(content, &result)
	case ".sql":
		result.WriteString("ðŸ—„ï¸ SQL File Analysis:\n")
		analyzeSQLFile(content, &result)
	default:
		result.WriteString("ðŸ“ Text File Content:\n")
		analyzeGenericTextFile(content, &result)
	}

	return mcp.NewToolResultText(result.String()), nil
}

func analyzeBatchFile(content string, result *strings.Builder) {
	lines := strings.Split(content, "\n")
	var variables []string
	var commands []string
	var calls []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "REM") || strings.HasPrefix(line, "::") {
			continue
		}

		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "SET ") {
			variables = append(variables, line)
		} else if strings.HasPrefix(upperLine, "CALL ") {
			calls = append(calls, line)
		} else if !strings.HasPrefix(upperLine, "ECHO ") && !strings.HasPrefix(upperLine, "@") {
			commands = append(commands, line)
		}
	}

	result.WriteString(fmt.Sprintf("â€¢ Variables Set: %d\n", len(variables)))
	if len(variables) > 0 {
		result.WriteString("  Variables:\n")
		for _, v := range variables {
			result.WriteString(fmt.Sprintf("    %s\n", v))
		}
	}

	result.WriteString(fmt.Sprintf("â€¢ Function Calls: %d\n", len(calls)))
	if len(calls) > 0 {
		result.WriteString("  Calls:\n")
		for _, c := range calls {
			result.WriteString(fmt.Sprintf("    %s\n", c))
		}
	}

	result.WriteString(fmt.Sprintf("â€¢ Executable Commands: %d\n", len(commands)))
	if len(commands) > 0 {
		result.WriteString("  Commands:\n")
		for i, c := range commands {
			if i >= 10 { // Limit output
				result.WriteString(fmt.Sprintf("    ... and %d more\n", len(commands)-10))
				break
			}
			result.WriteString(fmt.Sprintf("    %s\n", c))
		}
	}
}

func analyzeConfigFile(content string, result *strings.Builder) {
	lines := strings.Split(content, "\n")
	var keyValues []string
	var sections []string

	currentSection := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line
			sections = append(sections, line)
		} else if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if currentSection != "" {
					keyValues = append(keyValues, fmt.Sprintf("[%s] %s = %s", currentSection, key, value))
				} else {
					keyValues = append(keyValues, fmt.Sprintf("%s = %s", key, value))
				}
			}
		}
	}

	result.WriteString(fmt.Sprintf("â€¢ Configuration Sections: %d\n", len(sections)))
	if len(sections) > 0 {
		result.WriteString("  Sections:\n")
		for _, s := range sections {
			result.WriteString(fmt.Sprintf("    %s\n", s))
		}
	}

	result.WriteString(fmt.Sprintf("â€¢ Key-Value Pairs: %d\n", len(keyValues)))
	if len(keyValues) > 0 {
		result.WriteString("  Settings:\n")
		for i, kv := range keyValues {
			if i >= 20 { // Limit output
				result.WriteString(fmt.Sprintf("    ... and %d more\n", len(keyValues)-20))
				break
			}
			result.WriteString(fmt.Sprintf("    %s\n", kv))
		}
	}
}

func analyzeSQLFile(content string, result *strings.Builder) {
	upperContent := strings.ToUpper(content)

	// Count different types of SQL statements
	selectCount := strings.Count(upperContent, "SELECT ")
	insertCount := strings.Count(upperContent, "INSERT ")
	updateCount := strings.Count(upperContent, "UPDATE ")
	deleteCount := strings.Count(upperContent, "DELETE ")
	createCount := strings.Count(upperContent, "CREATE ")

	result.WriteString("â€¢ SQL Statement Counts:\n")
	result.WriteString(fmt.Sprintf("  - SELECT statements: %d\n", selectCount))
	result.WriteString(fmt.Sprintf("  - INSERT statements: %d\n", insertCount))
	result.WriteString(fmt.Sprintf("  - UPDATE statements: %d\n", updateCount))
	result.WriteString(fmt.Sprintf("  - DELETE statements: %d\n", deleteCount))
	result.WriteString(fmt.Sprintf("  - CREATE statements: %d\n", createCount))

	// Check for potential SSIS-related patterns
	if strings.Contains(upperContent, "EXECUTE") || strings.Contains(upperContent, "SP_") {
		result.WriteString("â€¢ Contains stored procedure calls\n")
	}

	if strings.Contains(upperContent, "BULK INSERT") {
		result.WriteString("â€¢ Contains bulk operations\n")
	}
}

func analyzeGenericTextFile(content string, result *strings.Builder) {
	lines := strings.Split(content, "\n")
	nonEmptyLines := 0
	totalWords := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			nonEmptyLines++
			words := strings.Fields(line)
			totalWords += len(words)
		}
	}

	result.WriteString(fmt.Sprintf("â€¢ Non-empty Lines: %d\n", nonEmptyLines))
	result.WriteString(fmt.Sprintf("â€¢ Total Words: %d\n", totalWords))
	result.WriteString(fmt.Sprintf("â€¢ Average Words per Line: %.1f\n\n", float64(totalWords)/float64(nonEmptyLines)))

	// Show first 20 lines
	result.WriteString("ðŸ“„ Content Preview (first 20 lines):\n")
	for i, line := range lines {
		if i >= 20 {
			if len(lines) > 20 {
				result.WriteString(fmt.Sprintf("... (%d more lines)\n", len(lines)-20))
			}
			break
		}
		result.WriteString(fmt.Sprintf("%4d: %s\n", i+1, strings.TrimRight(line, "\r\n")))
	}
}

type ScriptComplexityMetrics struct {
	ScriptTaskCount   int
	TotalLines        int
	AverageComplexity float64
	QualityScore      int
}

type ExpressionComplexityMetrics struct {
	TotalExpressions int
	AverageLength    float64
	ComplexityScore  int
}

type VariableUsageMetrics struct {
	TotalVariables   int
	ExpressionsCount int
	UsageScore       int
}

func calculateStructuralComplexity(pkg SSISPackage) int {
	taskCount := len(pkg.Executables.Tasks)
	connCount := len(pkg.ConnectionMgr.Connections)
	varCount := len(pkg.Variables.Vars)

	// Score based on size (smaller packages are easier to maintain)
	sizeScore := 10
	if taskCount > 50 || connCount > 20 || varCount > 100 {
		sizeScore = 3
	} else if taskCount > 30 || connCount > 10 || varCount > 50 {
		sizeScore = 5
	} else if taskCount > 15 || connCount > 5 || varCount > 25 {
		sizeScore = 7
	}

	return sizeScore
}

func calculateControlFlowComplexity(pkg SSISPackage) int {
	constraintCount := len(pkg.PrecedenceConstraints.Constraints)

	// Score based on control flow complexity
	if constraintCount > 100 {
		return 2
	} else if constraintCount > 50 {
		return 4
	} else if constraintCount > 25 {
		return 6
	} else if constraintCount > 10 {
		return 8
	}
	return 10
}

func analyzeScriptComplexity(tasks []Task) ScriptComplexityMetrics {
	var metrics ScriptComplexityMetrics
	totalComplexity := 0.0

	for _, task := range tasks {
		if getTaskType(task) == "Script Task" {
			metrics.ScriptTaskCount++
			scriptCode := task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode

			// Count lines
			lines := strings.Split(scriptCode, "\n")
			metrics.TotalLines += len(lines)

			// Calculate complexity (simplified)
			complexity := calculateScriptComplexity(scriptCode)
			totalComplexity += complexity
		}
	}

	if metrics.ScriptTaskCount > 0 {
		metrics.AverageComplexity = totalComplexity / float64(metrics.ScriptTaskCount)
		metrics.QualityScore = int(10 - metrics.AverageComplexity/2) // Scale to 0-10
		if metrics.QualityScore < 1 {
			metrics.QualityScore = 1
		}
	} else {
		metrics.QualityScore = 10 // No scripts = good
	}

	return metrics
}

func calculateScriptComplexity(scriptCode string) float64 {
	complexity := 1.0

	// Count control structures
	complexity += float64(strings.Count(scriptCode, "if "))
	complexity += float64(strings.Count(scriptCode, "for "))
	complexity += float64(strings.Count(scriptCode, "while "))
	complexity += float64(strings.Count(scriptCode, "foreach "))
	complexity += float64(strings.Count(scriptCode, "try "))
	complexity += float64(strings.Count(scriptCode, "catch "))

	// Count method calls (approximate)
	complexity += float64(strings.Count(scriptCode, "(")) * 0.1

	// Length factor
	complexity += float64(len(scriptCode)) / 1000.0

	return complexity
}

func analyzeExpressionComplexity(pkg SSISPackage) ExpressionComplexityMetrics {
	var metrics ExpressionComplexityMetrics
	totalLength := 0

	// Check task expressions
	for _, task := range pkg.Executables.Tasks {
		for _, prop := range task.Properties {
			if strings.Contains(prop.Name, "Expression") || prop.Name == "Expression" {
				metrics.TotalExpressions++
				totalLength += len(prop.Value)
			}
		}
	}

	// Check variable expressions
	for _, variable := range pkg.Variables.Vars {
		if variable.Expression != "" {
			metrics.TotalExpressions++
			totalLength += len(variable.Expression)
		}
	}

	if metrics.TotalExpressions > 0 {
		metrics.AverageLength = float64(totalLength) / float64(metrics.TotalExpressions)
	}

	// Score based on expression complexity
	if metrics.TotalExpressions > 50 || metrics.AverageLength > 200 {
		metrics.ComplexityScore = 3
	} else if metrics.TotalExpressions > 25 || metrics.AverageLength > 100 {
		metrics.ComplexityScore = 5
	} else if metrics.TotalExpressions > 10 || metrics.AverageLength > 50 {
		metrics.ComplexityScore = 7
	} else {
		metrics.ComplexityScore = 9
	}

	return metrics
}

func analyzeVariableUsage(pkg SSISPackage) VariableUsageMetrics {
	var metrics VariableUsageMetrics

	metrics.TotalVariables = len(pkg.Variables.Vars)

	for _, variable := range pkg.Variables.Vars {
		if variable.Expression != "" {
			metrics.ExpressionsCount++
		}
	}

	// Score based on variable usage patterns
	expressionRatio := float64(metrics.ExpressionsCount) / float64(metrics.TotalVariables)
	if expressionRatio > 0.8 {
		metrics.UsageScore = 7 // Good use of expressions
	} else if expressionRatio > 0.5 {
		metrics.UsageScore = 8
	} else if expressionRatio > 0.3 {
		metrics.UsageScore = 9
	} else {
		metrics.UsageScore = 6 // Could use more expressions
	}

	return metrics
}

func calculateOverallScore(structural, script, expression, variable int) int {
	// Weighted average
	weights := []float64{0.3, 0.3, 0.2, 0.2}
	scores := []int{structural, script, expression, variable}

	total := 0.0
	for i, score := range scores {
		total += float64(score) * weights[i]
	}

	return int(total + 0.5) // Round to nearest
}

func getMaintainabilityRating(score int) string {
	switch {
	case score >= 9:
		return "Excellent"
	case score >= 7:
		return "Good"
	case score >= 5:
		return "Fair"
	case score >= 3:
		return "Poor"
	default:
		return "Critical"
	}
}

func addQualityRecommendations(result *strings.Builder, overallScore int, structuralScore int, scriptMetrics ScriptComplexityMetrics, expressionMetrics ExpressionComplexityMetrics, variableMetrics VariableUsageMetrics) {
	if overallScore < 5 {
		result.WriteString("â€¢ Consider breaking down large packages into smaller, focused packages\n")
		result.WriteString("â€¢ Review and simplify complex expressions\n")
		result.WriteString("â€¢ Refactor overly complex script tasks\n")
		result.WriteString("â€¢ Implement proper error handling and logging\n")
	} else if overallScore < 7 {
		result.WriteString("â€¢ Consider using more expressions for dynamic configuration\n")
		result.WriteString("â€¢ Review script task complexity and consider alternatives\n")
		result.WriteString("â€¢ Document complex expressions and logic\n")
	} else {
		result.WriteString("â€¢ Package quality is good - continue best practices\n")
		result.WriteString("â€¢ Consider adding more comprehensive error handling\n")
		result.WriteString("â€¢ Regular code reviews recommended\n")
	}

	if structuralScore < 5 {
		result.WriteString("â€¢ Package is very large - consider splitting into multiple packages\n")
	}

	if scriptMetrics.QualityScore < 5 {
		result.WriteString("â€¢ Script tasks are complex - consider using built-in SSIS components instead\n")
	}

	if expressionMetrics.ComplexityScore < 5 {
		result.WriteString("â€¢ Expressions are very complex - consider using variables or script tasks\n")
	}

	if variableMetrics.UsageScore < 5 {
		result.WriteString("â€¢ Limited use of expressions - consider parameterizing more values\n")
	}
}

func compareProperties(props1, props2 []Property, result *strings.Builder) {
	propMap1 := make(map[string]string)
	propMap2 := make(map[string]string)

	for _, p := range props1 {
		propMap1[p.Name] = p.Value
	}
	for _, p := range props2 {
		propMap2[p.Name] = p.Value
	}

	// Added properties
	for name, value := range propMap2 {
		if _, exists := propMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s = %s\n", name, value))
		}
	}

	// Removed properties
	for name, value := range propMap1 {
		if _, exists := propMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s = %s\n", name, value))
		}
	}

	// Modified properties
	for name, value1 := range propMap1 {
		if value2, exists := propMap2[name]; exists && value1 != value2 {
			result.WriteString(fmt.Sprintf("  âœï¸ Modified: %s\n", name))
			result.WriteString(fmt.Sprintf("    File 1: %s\n", value1))
			result.WriteString(fmt.Sprintf("    File 2: %s\n", value2))
		}
	}

	if len(propMap1) == len(propMap2) && len(propMap1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareConnections(conns1, conns2 []Connection, result *strings.Builder) {
	connMap1 := make(map[string]Connection)
	connMap2 := make(map[string]Connection)

	for _, c := range conns1 {
		connMap1[c.Name] = c
	}
	for _, c := range conns2 {
		connMap2[c.Name] = c
	}

	// Added connections
	for name := range connMap2 {
		if _, exists := connMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	// Removed connections
	for name := range connMap1 {
		if _, exists := connMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

	// Modified connections
	for name, conn1 := range connMap1 {
		if conn2, exists := connMap2[name]; exists {
			connStr1 := conn1.ObjectData.ConnectionMgr.ConnectionString
			connStr2 := conn2.ObjectData.ConnectionMgr.ConnectionString
			if connStr1 == "" {
				connStr1 = conn1.ObjectData.MsmqConnMgr.ConnectionString
			}
			if connStr2 == "" {
				connStr2 = conn2.ObjectData.MsmqConnMgr.ConnectionString
			}
			if connStr1 != connStr2 {
				result.WriteString(fmt.Sprintf("  âœï¸ Modified: %s\n", name))
				result.WriteString(fmt.Sprintf("    File 1: %s\n", connStr1))
				result.WriteString(fmt.Sprintf("    File 2: %s\n", connStr2))
			}
		}
	}

	if len(conns1) == len(conns2) && len(conns1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareVariables(vars1, vars2 []Variable, result *strings.Builder) {
	varMap1 := make(map[string]Variable)
	varMap2 := make(map[string]Variable)

	for _, v := range vars1 {
		varMap1[v.Name] = v
	}
	for _, v := range vars2 {
		varMap2[v.Name] = v
	}

	// Added variables
	for name := range varMap2 {
		if _, exists := varMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	// Removed variables
	for name := range varMap1 {
		if _, exists := varMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

	// Modified variables
	for name, var1 := range varMap1 {
		if var2, exists := varMap2[name]; exists {
			if var1.Value != var2.Value || var1.Expression != var2.Expression {
				result.WriteString(fmt.Sprintf("  âœï¸ Modified: %s\n", name))
				result.WriteString(fmt.Sprintf("    File 1: Value='%s', Expression='%s'\n", var1.Value, var1.Expression))
				result.WriteString(fmt.Sprintf("    File 2: Value='%s', Expression='%s'\n", var2.Value, var2.Expression))
			}
		}
	}

	if len(vars1) == len(vars2) && len(vars1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareParameters(params1, params2 []Parameter, result *strings.Builder) {
	paramMap1 := make(map[string]Parameter)
	paramMap2 := make(map[string]Parameter)

	for _, p := range params1 {
		paramMap1[p.Name] = p
	}
	for _, p := range params2 {
		paramMap2[p.Name] = p
	}

	// Added parameters
	for name := range paramMap2 {
		if _, exists := paramMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	// Removed parameters
	for name := range paramMap1 {
		if _, exists := paramMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

	// Modified parameters
	for name, param1 := range paramMap1 {
		if param2, exists := paramMap2[name]; exists {
			if param1.DataType != param2.DataType || param1.Value != param2.Value {
				result.WriteString(fmt.Sprintf("  âœï¸ Modified: %s\n", name))
				result.WriteString(fmt.Sprintf("    File 1: Type='%s', Value='%s'\n", param1.DataType, param1.Value))
				result.WriteString(fmt.Sprintf("    File 2: Type='%s', Value='%s'\n", param2.DataType, param2.Value))
			}
		}
	}

	if len(params1) == len(params2) && len(params1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareConfigurations(configs1, configs2 []Configuration, result *strings.Builder) {
	if len(configs1) != len(configs2) {
		result.WriteString(fmt.Sprintf("  ðŸ“Š Count changed: %d â†’ %d\n", len(configs1), len(configs2)))
	} else if len(configs1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareTasks(tasks1, tasks2 []Task, result *strings.Builder) {
	taskMap1 := make(map[string]Task)
	taskMap2 := make(map[string]Task)

	for _, t := range tasks1 {
		taskMap1[t.Name] = t
	}
	for _, t := range tasks2 {
		taskMap2[t.Name] = t
	}

	// Added tasks
	for name := range taskMap2 {
		if _, exists := taskMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	// Removed tasks
	for name := range taskMap1 {
		if _, exists := taskMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

	// Modified tasks (simplified - just check if properties differ in count)
	for name, task1 := range taskMap1 {
		if task2, exists := taskMap2[name]; exists {
			if len(task1.Properties) != len(task2.Properties) {
				result.WriteString(fmt.Sprintf("  âœï¸ Modified: %s (property count changed: %d â†’ %d)\n", name, len(task1.Properties), len(task2.Properties)))
			}
		}
	}

	if len(tasks1) == len(tasks2) && len(tasks1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareEventHandlers(handlers1, handlers2 []EventHandler, result *strings.Builder) {
	if len(handlers1) != len(handlers2) {
		result.WriteString(fmt.Sprintf("  ðŸ“Š Count changed: %d â†’ %d\n", len(handlers1), len(handlers2)))
	} else if len(handlers1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func comparePrecedenceConstraints(constraints1, constraints2 []PrecedenceConstraint, result *strings.Builder) {
	if len(constraints1) != len(constraints2) {
		result.WriteString(fmt.Sprintf("  ðŸ“Š Count changed: %d â†’ %d\n", len(constraints1), len(constraints2)))
	} else if len(constraints1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func analyzeConnectionSecurity(connections []Connection) []string {
	var issues []string

	for _, conn := range connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		connStrLower := strings.ToLower(connStr)
		// Check for password patterns in connection string
		if strings.Contains(connStrLower, "password=") || strings.Contains(connStrLower, "pwd=") ||
			strings.Contains(connStrLower, "user id=") || strings.Contains(connStrLower, "uid=") {
			issues = append(issues, fmt.Sprintf("Connection '%s' contains potential credentials in connection string", conn.Name))
		}
		// Check for integrated security=false which might indicate SQL auth
		if strings.Contains(connStrLower, "integrated security=false") ||
			strings.Contains(connStrLower, "trusted_connection=false") {
			issues = append(issues, fmt.Sprintf("Connection '%s' uses SQL Server authentication (consider Windows auth)", conn.Name))
		}
	}

	return issues
}

func analyzeVariableSecurity(variables []Variable) []string {
	var issues []string

	sensitiveVarPatterns := []string{"password", "pwd", "secret", "key", "token", "credential"}

	for _, variable := range variables {
		varNameLower := strings.ToLower(variable.Name)
		for _, pattern := range sensitiveVarPatterns {
			if strings.Contains(varNameLower, pattern) {
				// Check if the value looks like a real credential (not empty, not expression)
				if variable.Value != "" && !strings.HasPrefix(variable.Value, "@") {
					issues = append(issues, fmt.Sprintf("Variable '%s' contains sensitive data: %s", variable.Name, maskSensitiveValue(variable.Value)))
				}
			}
		}
	}

	return issues
}

func analyzeScriptSecurity(tasks []Task) []string {
	var issues []string

	for _, task := range tasks {
		if getTaskType(task) == "Script Task" {
			// Check script project for hardcoded strings
			scriptCode := task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode
			if scriptCode != "" {
				scriptLower := strings.ToLower(scriptCode)
				// Check for common credential patterns in script code
				if strings.Contains(scriptLower, "password") || strings.Contains(scriptLower, "pwd") ||
					strings.Contains(scriptLower, "secret") || strings.Contains(scriptLower, "connectionstring") {
					issues = append(issues, fmt.Sprintf("Script Task '%s' may contain hardcoded credentials in script code", task.Name))
				}
			}
		}
	}

	return issues
}

func analyzeExpressionSecurity(tasks []Task, variables []Variable) []string {
	var issues []string

	for _, task := range tasks {
		for _, prop := range task.Properties {
			if prop.Name == "Expression" || strings.Contains(prop.Name, "Expression") {
				expr := strings.ToLower(prop.Value)
				// Check for hardcoded sensitive data in expressions
				if strings.Contains(expr, "password") || strings.Contains(expr, "pwd") ||
					strings.Contains(expr, "secret") || strings.Contains(expr, "key") {
					issues = append(issues, fmt.Sprintf("Task '%s' expression may contain sensitive data", task.Name))
				}
			}
		}
	}

	return issues
}

func maskSensitiveValue(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
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
	case strings.Contains(classID, "OLEDBSource") || strings.Contains(classID, "Microsoft.OLEDBSource"):
		return "OLE DB Source"
	case strings.Contains(classID, "OLEDBDestination") || strings.Contains(classID, "Microsoft.OLEDBDestination"):
		return "OLE DB Destination"
	case strings.Contains(classID, "FlatFileSource") || strings.Contains(classID, "Microsoft.FlatFileSource"):
		return "Flat File Source"
	case strings.Contains(classID, "FlatFileDestination"):
		return "Flat File Destination"
	case strings.Contains(classID, "DataConvert") || strings.Contains(classID, "Microsoft.DataConvert"):
		return "Data Conversion"
	case strings.Contains(classID, "Lookup"):
		return "Lookup"
	case strings.Contains(classID, "FuzzyLookup"):
		return "Fuzzy Lookup"
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
	case strings.Contains(classID, "Pivot"):
		return "Pivot"
	case strings.Contains(classID, "Unpivot"):
		return "Unpivot"
	case strings.Contains(classID, "TermExtraction"):
		return "Term Extraction"
	case strings.Contains(classID, "TermLookup"):
		return "Term Lookup"
	case strings.Contains(classID, "FuzzyGrouping"):
		return "Fuzzy Grouping"
	case strings.Contains(classID, "RowCount"):
		return "Row Count"
	case strings.Contains(classID, "RowSampling"):
		return "Row Sampling"
	case strings.Contains(classID, "PercentageSampling"):
		return "Percentage Sampling"
	case strings.Contains(classID, "SlowlyChangingDimension"):
		return "Slowly Changing Dimension"
	case strings.Contains(classID, "CharacterMap"):
		return "Character Map"
	case strings.Contains(classID, "CopyColumn"):
		return "Copy Column"
	case strings.Contains(classID, "DerivedColumn"):
		return "Derived Column"
	case strings.Contains(classID, "DataMiningQuery"):
		return "Data Mining Query"
	case strings.Contains(classID, "DimensionProcessing"):
		return "Dimension Processing"
	case strings.Contains(classID, "PartitionProcessing"):
		return "Partition Processing"
	case strings.Contains(classID, "ScriptComponent"):
		return "Script Component"
	case strings.Contains(classID, "Lineage"):
		return "Audit"
	case strings.Contains(classID, "ManagedComponentHost"):
		return "Script Component"
	case strings.Contains(classID, "Cache"):
		return "Cache Transform"
	// Third-party and custom components
	case strings.Contains(classID, "KingswaySoft"):
		return "KingswaySoft Component"
	case strings.Contains(classID, "CozyRoc"):
		return "CozyRoc Component"
	case strings.Contains(classID, "PragmaticWorks"):
		return "Pragmatic Works Component"
	case strings.Contains(classID, "AuntieDot"):
		return "AuntieDot Component"
	case strings.Contains(classID, "BlueSSIS"):
		return "BlueSSIS Component"
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

func extractPropertyValue(xmlContent, propertyName string) string {
	// Look for property in the format: <Property Name="PropertyName">Value</Property>
	pattern := fmt.Sprintf(`<Property Name="%s">(.*?)</Property>`, propertyName)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(xmlContent)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
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

func handleAnalyzePivot(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Pivot Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Pivot" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract pivot-specific properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "PivotKey":
							result.WriteString(fmt.Sprintf("  Pivot Key: %s\n", prop.Value))
						case "SetKey":
							result.WriteString(fmt.Sprintf("  Set Key: %s\n", prop.Value))
						case "PivotValue":
							result.WriteString(fmt.Sprintf("  Pivot Value: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Pivot components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeUnpivot(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Unpivot Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Unpivot" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract unpivot-specific properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "PivotKeyValue":
							result.WriteString(fmt.Sprintf("  Pivot Key Value: %s\n", prop.Value))
						case "DestinationColumn":
							result.WriteString(fmt.Sprintf("  Destination Column: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Unpivot components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeTermExtraction(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Term Extraction Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.TermExtraction" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract term extraction-specific properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "ExclusionTermTable":
							result.WriteString(fmt.Sprintf("  Exclusion Term Table: %s\n", prop.Value))
						case "TermTable":
							result.WriteString(fmt.Sprintf("  Term Table: %s\n", prop.Value))
						case "ScoreType":
							result.WriteString(fmt.Sprintf("  Score Type: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Term Extraction components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeFuzzyLookup(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Fuzzy Lookup Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FuzzyLookup" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract fuzzy lookup-specific properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "ReferenceTableName":
							result.WriteString(fmt.Sprintf("  Reference Table: %s\n", prop.Value))
						case "MaxOutputMatches":
							result.WriteString(fmt.Sprintf("  Max Output Matches: %s\n", prop.Value))
						case "SimilarityThreshold":
							result.WriteString(fmt.Sprintf("  Similarity Threshold: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Fuzzy Lookup components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeFuzzyGrouping(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Fuzzy Grouping Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FuzzyGrouping" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract fuzzy grouping-specific properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "GroupingKey":
							result.WriteString(fmt.Sprintf("  Grouping Key: %s\n", prop.Value))
						case "SimilarityThreshold":
							result.WriteString(fmt.Sprintf("  Similarity Threshold: %s\n", prop.Value))
						case "MinimumSimilarity":
							result.WriteString(fmt.Sprintf("  Minimum Similarity: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Fuzzy Grouping components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeRowCount(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Row Count Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.RowCount" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract row count-specific properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if prop.Name == "VariableName" {
							result.WriteString(fmt.Sprintf("  Variable Name: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Row Count components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeCharacterMap(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Character Map Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.CharacterMap" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Extract character map operations from XML
					xmlContent := string(data)
					charMapPattern := `<component name="` + comp.Name + `">(.*?)</component>`
					re := regexp.MustCompile(charMapPattern)
					matches := re.FindStringSubmatch(xmlContent)
					if len(matches) > 1 {
						componentXML := matches[1]
						operationPattern := `<property name="Operation">(.*?)</property>`
						opRe := regexp.MustCompile(operationPattern)
						opMatches := opRe.FindAllStringSubmatch(componentXML, -1)
						for i, match := range opMatches {
							if len(match) > 1 {
								result.WriteString(fmt.Sprintf("  Operation %d: %s\n", i+1, strings.TrimSpace(match[1])))
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Character Map components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeCopyColumn(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Copy Column Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.CopyColumn" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Count input and output columns
					inputCount := len(comp.Inputs.Inputs[0].InputColumns.Columns)
					outputCount := len(comp.Outputs.Outputs[0].OutputColumns.Columns)
					result.WriteString(fmt.Sprintf("  Input Columns: %d\n", inputCount))
					result.WriteString(fmt.Sprintf("  Output Columns: %d\n", outputCount))
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Copy Column components found in this package.\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeContainers(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Container Analysis:\n\n")

	containerCount := 0
	for _, task := range pkg.Executables.Tasks {
		containerType := ""
		if task.CreationName == "Microsoft.Sequence" {
			containerType = "Sequence Container"
		} else if task.CreationName == "Microsoft.ForLoop" {
			containerType = "For Loop Container"
		} else if task.CreationName == "Microsoft.ForEachLoop" {
			containerType = "Foreach Loop Container"
		}

		if containerType != "" {
			containerCount++
			result.WriteString(fmt.Sprintf("Container %d: %s (%s)\n", containerCount, task.Name, containerType))
			result.WriteString(fmt.Sprintf("  Description: %s\n", task.Description))

			// Extract container-specific properties
			for _, prop := range task.Properties {
				switch prop.Name {
				case "Disabled":
					result.WriteString(fmt.Sprintf("  Disabled: %s\n", prop.Value))
				case "FailPackageOnFailure":
					result.WriteString(fmt.Sprintf("  Fail Package On Failure: %s\n", prop.Value))
				case "FailParentOnFailure":
					result.WriteString(fmt.Sprintf("  Fail Parent On Failure: %s\n", prop.Value))
				}
			}

			// Count nested executables
			if task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode != "" {
				// This is a task with script, not a container
				continue
			}

			// For containers, count nested tasks (this is a simplified approach)
			xmlContent := string(data)
			containerPattern := fmt.Sprintf(`<Executable.*ObjectName="%s".*CreationName="%s">(.*?)</Executable>`, regexp.QuoteMeta(task.Name), regexp.QuoteMeta(task.CreationName))
			re := regexp.MustCompile(containerPattern)
			matches := re.FindStringSubmatch(xmlContent)
			if len(matches) > 1 {
				containerXML := matches[1]
				nestedTaskPattern := `<Executable.*CreationName="`
				nestedRe := regexp.MustCompile(nestedTaskPattern)
				nestedMatches := nestedRe.FindAllString(containerXML, -1)
				result.WriteString(fmt.Sprintf("  Nested Executables: %d\n", len(nestedMatches)))
			}

			result.WriteString("\n")
		}
	}

	if containerCount == 0 {
		result.WriteString("No containers found in this package.\n")
	} else {
		result.WriteString(fmt.Sprintf("Total containers found: %d\n", containerCount))
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeCustomComponents(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Custom and Third-Party Components Analysis:\n\n")

	customCount := 0
	for _, task := range pkg.Executables.Tasks {
		if task.ObjectData.DataFlow.Components.Components != nil {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				// Check if this is a custom or third-party component
				isCustom := false
				vendor := ""

				classID := strings.ToLower(comp.ComponentClassID)
				if strings.Contains(classID, "kingswaysoft") {
					isCustom = true
					vendor = "KingswaySoft"
				} else if strings.Contains(classID, "cozyroc") {
					isCustom = true
					vendor = "CozyRoc"
				} else if strings.Contains(classID, "pragmaticworks") || strings.Contains(classID, "pragmatic") {
					isCustom = true
					vendor = "Pragmatic Works"
				} else if strings.Contains(classID, "auntiedot") {
					isCustom = true
					vendor = "AuntieDot"
				} else if strings.Contains(classID, "bluessis") {
					isCustom = true
					vendor = "BlueSSIS"
				} else if !strings.HasPrefix(classID, "microsoft.sqlserver.dts.pipeline") &&
					!strings.HasPrefix(classID, "microsoft.sqlserver.dts") &&
					!strings.HasPrefix(classID, "microsoft.") {
					isCustom = true
					vendor = "Unknown/Custom"
				}

				if isCustom {
					customCount++
					result.WriteString(fmt.Sprintf("Component %d: %s\n", customCount, comp.Name))
					result.WriteString(fmt.Sprintf("  Vendor: %s\n", vendor))
					result.WriteString(fmt.Sprintf("  Class ID: %s\n", comp.ComponentClassID))
					result.WriteString(fmt.Sprintf("  Description: %s\n", comp.Description))

					// Extract key properties
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if customCount == 0 {
		result.WriteString("No custom or third-party components found in this package.\n")
		result.WriteString("All components appear to be standard Microsoft SSIS components.\n")
	} else {
		result.WriteString(fmt.Sprintf("Total custom/third-party components found: %d\n", customCount))
	}

	return mcp.NewToolResultText(result.String()), nil
}

// Advanced Security Features - Phase 2

func handleScanCredentials(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ” Advanced Credential Scanning Report:\n\n")

	issuesFound := false

	// Comprehensive credential pattern matching
	credentialPatterns := []struct {
		name     string
		patterns []string
		category string
	}{
		{
			name:     "Database Credentials",
			patterns: []string{"password=", "pwd=", "user id=", "uid=", "userid=", "trusted_connection=false", "integrated security=false"},
			category: "Database",
		},
		{
			name:     "API Keys & Tokens",
			patterns: []string{"apikey=", "api_key=", "token=", "bearer=", "authorization=", "x-api-key="},
			category: "API",
		},
		{
			name:     "Cloud Credentials",
			patterns: []string{"accesskey=", "secretkey=", "accountkey=", "sharedaccesskey=", "sas=", "connectionstring="},
			category: "Cloud",
		},
		{
			name:     "Encryption Keys",
			patterns: []string{"encryptionkey=", "key=", "certificate=", "privatekey=", "publickey="},
			category: "Encryption",
		},
		{
			name:     "Personal Data",
			patterns: []string{"ssn=", "socialsecurity=", "creditcard=", "cardnumber=", "email=", "phone="},
			category: "PII",
		},
	}

	// Check connection strings with advanced pattern matching
	result.WriteString("ðŸ”— Connection String Analysis:\n")
	connIssues := scanConnectionCredentials(pkg.ConnectionMgr.Connections, credentialPatterns)
	if len(connIssues) > 0 {
		issuesFound = true
		for _, issue := range connIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in connection strings.\n")
	}
	result.WriteString("\n")

	// Check variables for credential patterns
	result.WriteString("ðŸ“Š Variable Analysis:\n")
	varIssues := scanVariableCredentials(pkg.Variables.Vars, credentialPatterns)
	if len(varIssues) > 0 {
		issuesFound = true
		for _, issue := range varIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in variables.\n")
	}
	result.WriteString("\n")

	// Check script tasks for embedded credentials
	result.WriteString("ðŸ“œ Script Task Analysis:\n")
	scriptIssues := scanScriptCredentials(pkg.Executables.Tasks, credentialPatterns)
	if len(scriptIssues) > 0 {
		issuesFound = true
		for _, issue := range scriptIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in script tasks.\n")
	}
	result.WriteString("\n")

	// Check expressions for credential patterns
	result.WriteString("ðŸ” Expression Analysis:\n")
	exprIssues := scanExpressionCredentials(pkg.Executables.Tasks, credentialPatterns)
	if len(exprIssues) > 0 {
		issuesFound = true
		for _, issue := range exprIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in expressions.\n")
	}
	result.WriteString("\n")

	// Check raw XML content for additional patterns
	result.WriteString("ðŸ“„ Raw Content Analysis:\n")
	rawIssues := scanRawContentCredentials(string(data), credentialPatterns)
	if len(rawIssues) > 0 {
		issuesFound = true
		for _, issue := range rawIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in raw content.\n")
	}
	result.WriteString("\n")

	if !issuesFound {
		result.WriteString("âœ… No credential patterns detected in this package.\n\n")
		result.WriteString("ðŸ’¡ Credential Security Best Practices:\n")
		result.WriteString("â€¢ Use SSIS parameters for all credentials\n")
		result.WriteString("â€¢ Store sensitive data in environment variables or secure stores\n")
		result.WriteString("â€¢ Use Azure Key Vault or similar for cloud credentials\n")
		result.WriteString("â€¢ Implement proper package protection levels\n")
		result.WriteString("â€¢ Regularly rotate credentials and keys\n")
	} else {
		result.WriteString("ðŸš¨ Critical Security Recommendations:\n")
		result.WriteString("â€¢ Immediately replace all hardcoded credentials with secure alternatives\n")
		result.WriteString("â€¢ Implement proper encryption for sensitive package elements\n")
		result.WriteString("â€¢ Use SSIS package configurations or parameters\n")
		result.WriteString("â€¢ Consider using managed identity or service principals\n")
		result.WriteString("â€¢ Audit and monitor access to sensitive data\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleDetectEncryption(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ” Encryption Detection & Recommendations:\n\n")

	// Check package protection level
	result.WriteString("ðŸ“¦ Package Protection Level:\n")
	protectionLevel := "Not specified"
	for _, prop := range pkg.Properties {
		if prop.Name == "ProtectionLevel" {
			protectionLevel = prop.Value
			break
		}
	}

	switch protectionLevel {
	case "0", "DontSaveSensitive":
		result.WriteString("âš ï¸  Protection Level: DontSaveSensitive - Sensitive data will not be saved\n")
		result.WriteString("   This is generally secure but requires runtime parameter input\n")
	case "1", "EncryptSensitiveWithUserKey":
		result.WriteString("âš ï¸  Protection Level: EncryptSensitiveWithUserKey - Uses current user key\n")
		result.WriteString("   This may cause issues when deploying to different users/machines\n")
	case "2", "EncryptSensitiveWithPassword":
		result.WriteString("âœ… Protection Level: EncryptSensitiveWithPassword - Uses password protection\n")
		result.WriteString("   Good for deployment but requires secure password management\n")
	case "3", "EncryptAllWithPassword":
		result.WriteString("âœ… Protection Level: EncryptAllWithPassword - Encrypts entire package\n")
		result.WriteString("   Maximum security but requires password for execution\n")
	case "4", "EncryptAllWithUserKey":
		result.WriteString("âš ï¸  Protection Level: EncryptAllWithUserKey - Uses current user key for all data\n")
		result.WriteString("   May cause deployment issues across different users/machines\n")
	default:
		result.WriteString("â“ Protection Level: Unknown or not set\n")
		result.WriteString("   Consider setting an appropriate protection level\n")
	}
	result.WriteString("\n")

	// Check for encrypted connection strings
	result.WriteString("ðŸ”— Connection Encryption:\n")
	encryptionIssues := analyzeConnectionEncryption(pkg.ConnectionMgr.Connections)
	if len(encryptionIssues) > 0 {
		for _, issue := range encryptionIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No encryption issues detected in connections.\n")
	}
	result.WriteString("\n")

	// Check for sensitive data handling
	result.WriteString("ðŸ”’ Sensitive Data Handling:\n")
	sensitiveDataIssues := analyzeSensitiveDataHandling(pkg)
	if len(sensitiveDataIssues) > 0 {
		for _, issue := range sensitiveDataIssues {
			result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
		}
	} else {
		result.WriteString("No sensitive data handling issues detected.\n")
	}
	result.WriteString("\n")

	// Provide encryption recommendations
	result.WriteString("ðŸ’¡ Encryption Recommendations:\n")
	result.WriteString("â€¢ Use EncryptSensitiveWithPassword for most scenarios\n")
	result.WriteString("â€¢ Consider EncryptAllWithPassword for maximum security\n")
	result.WriteString("â€¢ Use Azure Key Vault for cloud deployments\n")
	result.WriteString("â€¢ Implement proper certificate management\n")
	result.WriteString("â€¢ Use SSL/TLS for all external connections\n")
	result.WriteString("â€¢ Consider column-level encryption for sensitive data\n")
	result.WriteString("â€¢ Implement proper key rotation policies\n")

	return mcp.NewToolResultText(result.String()), nil
}

func handleCheckCompliance(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	complianceStandard := request.GetString("compliance_standard", "all")

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("âš–ï¸  Compliance Check Report:\n")
	result.WriteString(fmt.Sprintf("Standard: %s\n\n", strings.ToUpper(complianceStandard)))

	issuesFound := false

	// GDPR Compliance Patterns
	if complianceStandard == "gdpr" || complianceStandard == "all" {
		result.WriteString("ðŸ‡ªðŸ‡º GDPR Compliance Analysis:\n")
		gdprIssues := checkGDPRCompliance(pkg, string(data))
		if len(gdprIssues) > 0 {
			issuesFound = true
			for _, issue := range gdprIssues {
				result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
			}
		} else {
			result.WriteString("No GDPR compliance issues detected.\n")
		}
		result.WriteString("\n")
	}

	// HIPAA Compliance Patterns
	if complianceStandard == "hipaa" || complianceStandard == "all" {
		result.WriteString("ðŸ¥ HIPAA Compliance Analysis:\n")
		hipaaIssues := checkHIPAACompliance(pkg, string(data))
		if len(hipaaIssues) > 0 {
			issuesFound = true
			for _, issue := range hipaaIssues {
				result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
			}
		} else {
			result.WriteString("No HIPAA compliance issues detected.\n")
		}
		result.WriteString("\n")
	}

	// PCI DSS Compliance Patterns
	if complianceStandard == "pci" || complianceStandard == "all" {
		result.WriteString("ðŸ’³ PCI DSS Compliance Analysis:\n")
		pciIssues := checkPCICompliance(pkg, string(data))
		if len(pciIssues) > 0 {
			issuesFound = true
			for _, issue := range pciIssues {
				result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
			}
		} else {
			result.WriteString("No PCI DSS compliance issues detected.\n")
		}
		result.WriteString("\n")
	}

	// General Data Protection Analysis
	if complianceStandard == "all" {
		result.WriteString("ðŸ”’ General Data Protection Analysis:\n")
		generalIssues := checkGeneralDataProtection(pkg, string(data))
		if len(generalIssues) > 0 {
			issuesFound = true
			for _, issue := range generalIssues {
				result.WriteString(fmt.Sprintf("âš ï¸  %s\n", issue))
			}
		} else {
			result.WriteString("No general data protection issues detected.\n")
		}
		result.WriteString("\n")
	}

	if !issuesFound {
		result.WriteString("âœ… No compliance issues detected for the specified standards.\n\n")
		result.WriteString("ðŸ’¡ Compliance Best Practices:\n")
		result.WriteString("â€¢ Implement proper data classification and labeling\n")
		result.WriteString("â€¢ Use encryption for sensitive data at rest and in transit\n")
		result.WriteString("â€¢ Implement access controls and audit logging\n")
		result.WriteString("â€¢ Regular security assessments and penetration testing\n")
		result.WriteString("â€¢ Data minimization and purpose limitation principles\n")
		result.WriteString("â€¢ Implement proper data retention and deletion policies\n")
	} else {
		result.WriteString("ðŸš¨ Compliance Remediation Required:\n")
		result.WriteString("â€¢ Address all flagged compliance issues immediately\n")
		result.WriteString("â€¢ Implement proper data protection measures\n")
		result.WriteString("â€¢ Consult with compliance officers and legal teams\n")
		result.WriteString("â€¢ Document compliance measures and controls\n")
		result.WriteString("â€¢ Regular compliance audits and monitoring\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// Helper functions for Advanced Security Features

func scanConnectionCredentials(connections []Connection, patterns []struct {
	name     string
	patterns []string
	category string
}) []string {
	var issues []string

	for _, conn := range connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		connStrLower := strings.ToLower(connStr)

		for _, patternGroup := range patterns {
			for _, pattern := range patternGroup.patterns {
				if strings.Contains(connStrLower, strings.ToLower(pattern)) {
					issues = append(issues, fmt.Sprintf("[%s] Connection '%s' contains %s pattern: %s",
						patternGroup.category, conn.Name, patternGroup.name, pattern))
				}
			}
		}
	}

	return issues
}

func scanVariableCredentials(variables []Variable, patterns []struct {
	name     string
	patterns []string
	category string
}) []string {
	var issues []string

	for _, variable := range variables {
		varNameLower := strings.ToLower(variable.Name)
		varValueLower := strings.ToLower(variable.Value)

		for _, patternGroup := range patterns {
			for _, pattern := range patternGroup.patterns {
				if strings.Contains(varNameLower, strings.ToLower(strings.TrimSuffix(pattern, "="))) ||
					strings.Contains(varValueLower, strings.ToLower(pattern)) {
					if variable.Value != "" && !strings.HasPrefix(variable.Value, "@") {
						issues = append(issues, fmt.Sprintf("[%s] Variable '%s' contains %s pattern",
							patternGroup.category, variable.Name, patternGroup.name))
					}
				}
			}
		}
	}

	return issues
}

func scanScriptCredentials(tasks []Task, patterns []struct {
	name     string
	patterns []string
	category string
}) []string {
	var issues []string

	for _, task := range tasks {
		if getTaskType(task) == "Script Task" {
			scriptCode := task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode
			if scriptCode != "" {
				scriptLower := strings.ToLower(scriptCode)

				for _, patternGroup := range patterns {
					for _, pattern := range patternGroup.patterns {
						if strings.Contains(scriptLower, strings.ToLower(pattern)) {
							issues = append(issues, fmt.Sprintf("[%s] Script Task '%s' contains %s pattern in code",
								patternGroup.category, task.Name, patternGroup.name))
						}
					}
				}
			}
		}
	}

	return issues
}

func scanExpressionCredentials(tasks []Task, patterns []struct {
	name     string
	patterns []string
	category string
}) []string {
	var issues []string

	for _, task := range tasks {
		for _, prop := range task.Properties {
			if prop.Name == "Expression" || strings.Contains(prop.Name, "Expression") {
				exprLower := strings.ToLower(prop.Value)

				for _, patternGroup := range patterns {
					for _, pattern := range patternGroup.patterns {
						if strings.Contains(exprLower, strings.ToLower(pattern)) {
							issues = append(issues, fmt.Sprintf("[%s] Task '%s' expression contains %s pattern",
								patternGroup.category, task.Name, patternGroup.name))
						}
					}
				}
			}
		}
	}

	return issues
}

func scanRawContentCredentials(content string, patterns []struct {
	name     string
	patterns []string
	category string
}) []string {
	var issues []string
	contentLower := strings.ToLower(content)

	for _, patternGroup := range patterns {
		for _, pattern := range patternGroup.patterns {
			if strings.Contains(contentLower, strings.ToLower(pattern)) {
				issues = append(issues, fmt.Sprintf("[%s] Raw content contains %s pattern: %s",
					patternGroup.category, patternGroup.name, pattern))
			}
		}
	}

	return issues
}

func analyzeConnectionEncryption(connections []Connection) []string {
	var issues []string

	for _, conn := range connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		connStrLower := strings.ToLower(connStr)

		// Check for unencrypted connections
		if strings.Contains(connStrLower, "server=") || strings.Contains(connStrLower, "data source=") {
			if !strings.Contains(connStrLower, "encrypt=true") && !strings.Contains(connStrLower, "ssl") {
				issues = append(issues, fmt.Sprintf("Connection '%s' may not be using encryption", conn.Name))
			}
		}

		// Check for HTTP connections without SSL
		if strings.Contains(connStrLower, "http://") && !strings.Contains(connStrLower, "https://") {
			issues = append(issues, fmt.Sprintf("Connection '%s' uses unencrypted HTTP", conn.Name))
		}
	}

	return issues
}

func analyzeSensitiveDataHandling(pkg SSISPackage) []string {
	var issues []string

	// Check for sensitive data in logs
	for _, task := range pkg.Executables.Tasks {
		if getTaskType(task) == "Execute SQL Task" {
			for _, prop := range task.Properties {
				if strings.Contains(strings.ToLower(prop.Name), "sqlstatement") {
					sqlLower := strings.ToLower(prop.Value)
					if strings.Contains(sqlLower, "password") || strings.Contains(sqlLower, "ssn") ||
						strings.Contains(sqlLower, "credit") || strings.Contains(sqlLower, "salary") {
						issues = append(issues, fmt.Sprintf("Execute SQL Task '%s' may be logging sensitive data", task.Name))
					}
				}
			}
		}
	}

	// Check for unencrypted flat files
	for _, conn := range pkg.ConnectionMgr.Connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		connStrLower := strings.ToLower(connStr)
		// Check for file paths in connection strings (likely flat files)
		if strings.Contains(connStrLower, ".csv") || strings.Contains(connStrLower, ".txt") ||
			strings.Contains(connStrLower, ".dat") || strings.Contains(connStrLower, "\\") {
			issues = append(issues, fmt.Sprintf("File-based connection '%s' detected - ensure file encryption if sensitive data", conn.Name))
		}
	}

	return issues
}

func checkGDPRCompliance(pkg SSISPackage, content string) []string {
	var issues []string
	contentLower := strings.ToLower(content)

	// GDPR-specific patterns
	gdprPatterns := []string{
		"email", "emailaddress", "firstname", "lastname", "fullname", "dateofbirth", "dob",
		"address", "phonenumber", "phone", "ipaddress", "ip", "location", "geolocation",
		"cookie", "tracking", "consent", "gdpr", "personaldata", "pii",
	}

	for _, pattern := range gdprPatterns {
		if strings.Contains(contentLower, pattern) {
			issues = append(issues, fmt.Sprintf("Potential GDPR-sensitive data pattern detected: %s", pattern))
		}
	}

	// Check for data retention patterns
	if strings.Contains(contentLower, "delete") || strings.Contains(contentLower, "truncate") {
		issues = append(issues, "Data deletion operations detected - ensure GDPR compliance for data retention")
	}

	return issues
}

func checkHIPAACompliance(pkg SSISPackage, content string) []string {
	var issues []string
	contentLower := strings.ToLower(content)

	// HIPAA-specific patterns
	hipaaPatterns := []string{
		"medicalrecord", "healthrecord", "diagnosis", "treatment", "medication",
		"patientid", "patient_id", "medicalid", "healthid", "phi", "protectedhealth",
		"insurance", "policy", "claim", "billing", "hipaa",
	}

	for _, pattern := range hipaaPatterns {
		if strings.Contains(contentLower, pattern) {
			issues = append(issues, fmt.Sprintf("Potential HIPAA-sensitive data pattern detected: %s", pattern))
		}
	}

	// Check for audit logging
	hasAudit := false
	for _, task := range pkg.Executables.Tasks {
		if getTaskType(task) == "Audit" || strings.Contains(strings.ToLower(task.Name), "audit") {
			hasAudit = true
			break
		}
	}
	if !hasAudit {
		issues = append(issues, "No audit logging detected - HIPAA requires audit trails for PHI access")
	}

	return issues
}

func checkPCICompliance(pkg SSISPackage, content string) []string {
	var issues []string
	contentLower := strings.ToLower(content)

	// PCI DSS-specific patterns
	pciPatterns := []string{
		"creditcard", "cardnumber", "cvv", "cvc", "expiration", "expiry",
		"cardholder", "pci", "payment", "transaction", "merchant",
	}

	for _, pattern := range pciPatterns {
		if strings.Contains(contentLower, pattern) {
			issues = append(issues, fmt.Sprintf("Potential PCI DSS-sensitive data pattern detected: %s", pattern))
		}
	}

	// Check for encryption of card data
	if strings.Contains(contentLower, "creditcard") || strings.Contains(contentLower, "cardnumber") {
		if !strings.Contains(contentLower, "encrypt") && !strings.Contains(contentLower, "mask") {
			issues = append(issues, "Card data detected without apparent encryption or masking")
		}
	}

	return issues
}

func checkGeneralDataProtection(pkg SSISPackage, content string) []string {
	var issues []string
	contentLower := strings.ToLower(content)

	// General data protection patterns
	generalPatterns := []string{
		"password", "pwd", "secret", "key", "token", "apikey", "api_key",
		"confidential", "sensitive", "private", "internal",
	}

	for _, pattern := range generalPatterns {
		if strings.Contains(contentLower, pattern) {
			issues = append(issues, fmt.Sprintf("Potential sensitive data pattern detected: %s", pattern))
		}
	}

	// Check for data masking or anonymization
	hasMasking := strings.Contains(contentLower, "mask") || strings.Contains(contentLower, "anonymize") ||
		strings.Contains(contentLower, "pseudonymize")

	if !hasMasking && (strings.Contains(contentLower, "personal") || strings.Contains(contentLower, "sensitive")) {
		issues = append(issues, "Sensitive data handling detected without apparent masking/anonymization")
	}

	return issues
}

// Performance Optimization Tools - Phase 2

func handleOptimizeBufferSize(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ”„ Buffer Size Optimization Analysis:\n\n")

	dataFlowCount := 0
	totalComponents := 0

	for _, task := range pkg.Executables.Tasks {
		if isDataFlowTask(task) {
			dataFlowCount++
			result.WriteString(fmt.Sprintf("ðŸ“Š Data Flow Task: %s\n", task.Name))

			// Analyze current buffer settings
			bufferSettings := analyzeBufferSettings(task)
			if len(bufferSettings) > 0 {
				result.WriteString("  Current Buffer Settings:\n")
				for _, setting := range bufferSettings {
					result.WriteString(fmt.Sprintf("  â€¢ %s: %s\n", setting.Name, setting.Value))
				}
			} else {
				result.WriteString("  No explicit buffer settings found (using defaults).\n")
			}

			// Count components and estimate data volume
			if task.ObjectData.DataFlow.Components.Components != nil {
				componentCount := len(task.ObjectData.DataFlow.Components.Components)
				totalComponents += componentCount

				result.WriteString(fmt.Sprintf("  Components: %d\n", componentCount))

				// Estimate buffer requirements based on component types
				bufferRecommendations := generateBufferRecommendations(task.ObjectData.DataFlow.Components.Components, bufferSettings)
				if len(bufferRecommendations) > 0 {
					result.WriteString("  ðŸ“ˆ Buffer Optimization Recommendations:\n")
					for _, rec := range bufferRecommendations {
						result.WriteString(fmt.Sprintf("    â€¢ %s\n", rec))
					}
				}
			}
			result.WriteString("\n")
		}
	}

	if dataFlowCount == 0 {
		result.WriteString("âŒ No Data Flow Tasks found in this package.\n\n")
		result.WriteString("ðŸ’¡ Buffer optimization is only applicable to Data Flow Tasks.\n")
	} else {
		result.WriteString("ðŸŽ¯ Overall Buffer Optimization Summary:\n")
		result.WriteString(fmt.Sprintf("â€¢ Analyzed %d Data Flow Task(s) with %d total components\n", dataFlowCount, totalComponents))

		// General buffer optimization guidelines
		result.WriteString("\nðŸ“‹ General Buffer Optimization Guidelines:\n")
		result.WriteString("â€¢ DefaultBufferSize: Start with 10MB, increase to 50MB+ for large datasets\n")
		result.WriteString("â€¢ DefaultBufferMaxRows: 10,000-100,000 rows based on row size\n")
		result.WriteString("â€¢ MaxBufferSize: Set to 100MB+ for very large datasets\n")
		result.WriteString("â€¢ MinBufferSize: Keep at 64KB unless processing very small datasets\n")
		result.WriteString("â€¢ AutoAdjustBufferSize: Enable for automatic optimization\n")
		result.WriteString("â€¢ BufferTempStoragePath: Use fast SSD storage for spill operations\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleAnalyzeParallelProcessing(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("âš¡ Parallel Processing Analysis:\n\n")

	// Analyze package-level parallel settings
	result.WriteString("ðŸ“¦ Package-Level Parallel Settings:\n")
	maxConcurrent := "Not set"
	for _, prop := range pkg.Properties {
		if prop.Name == "MaxConcurrentExecutables" {
			maxConcurrent = prop.Value
			break
		}
	}

	if maxConcurrent == "Not set" {
		result.WriteString("â€¢ MaxConcurrentExecutables: Not configured (using SSIS default)\n")
		result.WriteString("  âš ï¸  Consider setting this to optimize parallel execution\n")
	} else {
		result.WriteString(fmt.Sprintf("â€¢ MaxConcurrentExecutables: %s\n", maxConcurrent))
		if val, err := strconv.Atoi(maxConcurrent); err == nil && val < 4 {
			result.WriteString("  ðŸ’¡ Consider increasing for better parallel utilization\n")
		}
	}
	result.WriteString("\n")

	// Analyze task dependencies and parallel execution potential
	result.WriteString("ðŸ”— Task Execution Analysis:\n")
	taskAnalysis := analyzeTaskParallelization(pkg.Executables.Tasks)
	for _, analysis := range taskAnalysis {
		result.WriteString(fmt.Sprintf("â€¢ %s\n", analysis))
	}
	result.WriteString("\n")

	// Analyze data flow parallel processing
	result.WriteString("ðŸ”„ Data Flow Parallel Processing:\n")
	dataFlowAnalysis := analyzeDataFlowParallelization(pkg.Executables.Tasks)
	for _, analysis := range dataFlowAnalysis {
		result.WriteString(fmt.Sprintf("â€¢ %s\n", analysis))
	}
	result.WriteString("\n")

	// Container analysis for parallel execution
	result.WriteString("ðŸ“ Container Parallel Execution:\n")
	containerAnalysis := analyzeContainerParallelization(pkg.Executables.Tasks)
	for _, analysis := range containerAnalysis {
		result.WriteString(fmt.Sprintf("â€¢ %s\n", analysis))
	}
	result.WriteString("\n")

	// Performance recommendations
	result.WriteString("ðŸš€ Parallel Processing Optimization Recommendations:\n")
	result.WriteString("â€¢ Set MaxConcurrentExecutables to 2-4 times the number of CPU cores\n")
	result.WriteString("â€¢ Use Sequence Containers to group independent tasks\n")
	result.WriteString("â€¢ Configure EngineThreads (2-10) based on data flow complexity\n")
	result.WriteString("â€¢ Avoid unnecessary precedence constraints that block parallel execution\n")
	result.WriteString("â€¢ Use For Loop containers for parallel processing of multiple files\n")
	result.WriteString("â€¢ Consider partitioning large datasets for parallel processing\n")
	result.WriteString("â€¢ Monitor CPU utilization to avoid over-subscription\n")

	return mcp.NewToolResultText(result.String()), nil
}

func handleProfileMemoryUsage(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
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
	result.WriteString("ðŸ§  Memory Usage Profiling:\n\n")

	totalEstimatedMemory := int64(0)
	dataFlowCount := 0

	for _, task := range pkg.Executables.Tasks {
		if isDataFlowTask(task) {
			dataFlowCount++
			result.WriteString(fmt.Sprintf("ðŸ“Š Data Flow Task: %s\n", task.Name))

			// Analyze buffer memory usage
			bufferMemory := estimateBufferMemoryUsage(task)
			if bufferMemory > 0 {
				result.WriteString(fmt.Sprintf("  Buffer Memory: ~%s\n", formatBytes(bufferMemory)))
				totalEstimatedMemory += bufferMemory
			}

			// Analyze component memory usage
			if task.ObjectData.DataFlow.Components.Components != nil {
				result.WriteString("  Component Memory Analysis:\n")
				componentMemory := analyzeComponentMemoryUsage(task.ObjectData.DataFlow.Components.Components)
				for _, compMem := range componentMemory {
					result.WriteString(fmt.Sprintf("    â€¢ %s: %s\n", compMem.Component, compMem.Estimate))
					if compMem.Recommendation != "" {
						result.WriteString(fmt.Sprintf("      ðŸ’¡ %s\n", compMem.Recommendation))
					}
				}
			}

			// Check for memory-intensive operations
			memoryIssues := detectMemoryIntensiveOperations(task)
			if len(memoryIssues) > 0 {
				result.WriteString("  âš ï¸  Memory-Intensive Operations:\n")
				for _, issue := range memoryIssues {
					result.WriteString(fmt.Sprintf("    â€¢ %s\n", issue))
				}
			}
			result.WriteString("\n")
		}
	}

	if dataFlowCount == 0 {
		result.WriteString("âŒ No Data Flow Tasks found in this package.\n\n")
		result.WriteString("ðŸ’¡ Memory profiling is only applicable to Data Flow Tasks.\n")
	} else {
		result.WriteString("ðŸ“ˆ Overall Memory Profile:\n")
		result.WriteString(fmt.Sprintf("â€¢ Analyzed %d Data Flow Task(s)\n", dataFlowCount))
		result.WriteString(fmt.Sprintf("â€¢ Estimated Total Buffer Memory: ~%s\n", formatBytes(totalEstimatedMemory)))
		result.WriteString(fmt.Sprintf("â€¢ Recommended System Memory: ~%s+\n", formatBytes(totalEstimatedMemory*2)))

		// Memory optimization recommendations
		result.WriteString("\nðŸ§  Memory Optimization Recommendations:\n")
		result.WriteString("â€¢ Monitor actual memory usage during execution\n")
		result.WriteString("â€¢ Adjust DefaultBufferSize based on available RAM\n")
		result.WriteString("â€¢ Use 64-bit SSIS for large memory requirements\n")
		result.WriteString("â€¢ Consider data partitioning for very large datasets\n")
		result.WriteString("â€¢ Use BLOBTempStoragePath for large object processing\n")
		result.WriteString("â€¢ Optimize data types to reduce memory footprint\n")
		result.WriteString("â€¢ Consider caching strategies for reference data\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// Helper functions for Performance Optimization Tools

type BufferSetting struct {
	Name  string
	Value string
}

type ComponentMemory struct {
	Component      string
	Estimate       string
	Recommendation string
}

func analyzeBufferSettings(task Task) []BufferSetting {
	var settings []BufferSetting

	bufferProps := []string{
		"DefaultBufferSize", "DefaultBufferMaxRows", "MaxBufferSize",
		"MinBufferSize", "AutoAdjustBufferSize", "BufferTempStoragePath",
		"BLOBTempStoragePath", "EngineThreads",
	}

	for _, prop := range task.Properties {
		for _, bufferProp := range bufferProps {
			if prop.Name == bufferProp {
				settings = append(settings, BufferSetting{
					Name:  prop.Name,
					Value: prop.Value,
				})
				break
			}
		}
	}

	return settings
}

func generateBufferRecommendations(components []DataFlowComponent, bufferSettings []BufferSetting) []string {
	var recommendations []string

	// Count different component types
	sourceCount := 0
	transformCount := 0
	destCount := 0
	hasLookup := false
	hasSort := false
	hasAggregate := false

	for _, comp := range components {
		compType := getComponentType(comp.ComponentClassID)
		switch {
		case strings.Contains(strings.ToLower(compType), "source"):
			sourceCount++
		case strings.Contains(strings.ToLower(compType), "destination"):
			destCount++
		case strings.Contains(strings.ToLower(compType), "lookup"):
			hasLookup = true
			transformCount++
		case strings.Contains(strings.ToLower(compType), "sort"):
			hasSort = true
			transformCount++
		case strings.Contains(strings.ToLower(compType), "aggregate"):
			hasAggregate = true
			transformCount++
		default:
			transformCount++
		}
	}

	// Generate recommendations based on component analysis
	if sourceCount > 1 {
		recommendations = append(recommendations, "Multiple sources detected - consider increasing EngineThreads for parallel reading")
	}

	if destCount > 1 {
		recommendations = append(recommendations, "Multiple destinations detected - consider increasing EngineThreads for parallel writing")
	}

	if hasLookup {
		recommendations = append(recommendations, "Lookup transformations present - ensure sufficient memory for reference data caching")
	}

	if hasSort || hasAggregate {
		recommendations = append(recommendations, "Blocking transformations (Sort/Aggregate) detected - consider increasing DefaultBufferSize")
	}

	// Check current buffer settings and provide specific recommendations
	bufferSize := int64(0)
	bufferMaxRows := 0

	for _, setting := range bufferSettings {
		switch setting.Name {
		case "DefaultBufferSize":
			if val, err := strconv.ParseInt(setting.Value, 10, 64); err == nil {
				bufferSize = val
			}
		case "DefaultBufferMaxRows":
			if val, err := strconv.Atoi(setting.Value); err == nil {
				bufferMaxRows = val
			}
		}
	}

	if bufferSize < 10485760 { // Less than 10MB
		recommendations = append(recommendations, "DefaultBufferSize is low - consider increasing to 10MB+ for better performance")
	}

	if bufferMaxRows > 0 && bufferMaxRows < 10000 {
		recommendations = append(recommendations, "DefaultBufferMaxRows is low - consider increasing to 10,000+ for better throughput")
	}

	return recommendations
}

func analyzeTaskParallelization(tasks []Task) []string {
	var analysis []string

	// Simple analysis of task dependencies
	// In a real implementation, this would parse precedence constraints
	taskCount := len(tasks)
	if taskCount == 0 {
		return []string{"No tasks found in package"}
	}

	analysis = append(analysis, fmt.Sprintf("Total tasks: %d", taskCount))

	// Count different task types
	dataFlowCount := 0
	executeSQLCount := 0
	fileSystemCount := 0
	scriptCount := 0

	for _, task := range tasks {
		switch getTaskType(task) {
		case "Data Flow Task":
			dataFlowCount++
		case "Execute SQL Task":
			executeSQLCount++
		case "File System Task":
			fileSystemCount++
		case "Script Task":
			scriptCount++
		}
	}

	if dataFlowCount > 0 {
		analysis = append(analysis, fmt.Sprintf("Data Flow Tasks: %d (can run in parallel if no dependencies)", dataFlowCount))
	}
	if executeSQLCount > 0 {
		analysis = append(analysis, fmt.Sprintf("Execute SQL Tasks: %d (check for dependencies on same database)", executeSQLCount))
	}
	if fileSystemCount > 0 {
		analysis = append(analysis, fmt.Sprintf("File System Tasks: %d (can often run in parallel)", fileSystemCount))
	}

	return analysis
}

func analyzeDataFlowParallelization(tasks []Task) []string {
	var analysis []string

	for _, task := range tasks {
		if isDataFlowTask(task) {
			engineThreads := "Not set"
			for _, prop := range task.Properties {
				if prop.Name == "EngineThreads" {
					engineThreads = prop.Value
					break
				}
			}

			if engineThreads == "Not set" {
				analysis = append(analysis, fmt.Sprintf("Data Flow '%s': EngineThreads not configured (using default)", task.Name))
			} else {
				if val, err := strconv.Atoi(engineThreads); err == nil && val < 2 {
					analysis = append(analysis, fmt.Sprintf("Data Flow '%s': EngineThreads=%s (consider increasing for parallel processing)", task.Name, engineThreads))
				} else {
					analysis = append(analysis, fmt.Sprintf("Data Flow '%s': EngineThreads=%s (good for parallel processing)", task.Name, engineThreads))
				}
			}
		}
	}

	if len(analysis) == 0 {
		analysis = append(analysis, "No Data Flow Tasks found")
	}

	return analysis
}

func analyzeContainerParallelization(tasks []Task) []string {
	var analysis []string

	containerCount := 0
	for _, task := range tasks {
		if getTaskType(task) == "Sequence Container" || getTaskType(task) == "For Loop Container" || getTaskType(task) == "Foreach Loop Container" {
			containerCount++
			containerType := getTaskType(task)
			analysis = append(analysis, fmt.Sprintf("%s '%s': Enables parallel execution of child tasks", containerType, task.Name))
		}
	}

	if containerCount == 0 {
		analysis = append(analysis, "No containers found - consider using Sequence Containers to group independent tasks")
	} else {
		analysis = append(analysis, fmt.Sprintf("Found %d container(s) for organizing parallel execution", containerCount))
	}

	return analysis
}

func estimateBufferMemoryUsage(task Task) int64 {
	bufferSize := int64(1048576) // Default 1MB
	bufferMaxRows := 10000       // Default 10k rows

	for _, prop := range task.Properties {
		switch prop.Name {
		case "DefaultBufferSize":
			if val, err := strconv.ParseInt(prop.Value, 10, 64); err == nil {
				bufferSize = val
			}
		case "DefaultBufferMaxRows":
			if val, err := strconv.Atoi(prop.Value); err == nil {
				bufferMaxRows = val
			}
		}
	}

	// Estimate memory based on buffer settings
	// This is a simplified estimation: buffer size * estimated row overhead
	estimatedRowSize := int64(100) // Rough estimate of bytes per row
	rowOverhead := bufferMaxRows * int(estimatedRowSize)
	return bufferSize + int64(rowOverhead)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func analyzeComponentMemoryUsage(components []DataFlowComponent) []ComponentMemory {
	var memoryAnalysis []ComponentMemory

	for _, comp := range components {
		compType := getComponentType(comp.ComponentClassID)
		var estimate string
		var recommendation string

		switch {
		case strings.Contains(strings.ToLower(compType), "lookup"):
			estimate = "High (caches reference data)"
			recommendation = "Ensure sufficient memory for lookup cache"
		case strings.Contains(strings.ToLower(compType), "sort"):
			estimate = "High (requires memory for sorting)"
			recommendation = "Consider increasing buffer size for large sorts"
		case strings.Contains(strings.ToLower(compType), "aggregate"):
			estimate = "Medium-High (accumulates data)"
			recommendation = "Monitor memory usage during aggregation"
		case strings.Contains(strings.ToLower(compType), "merge"):
			estimate = "Medium (combines multiple inputs)"
			recommendation = "Ensure adequate buffer size for merge operations"
		case strings.Contains(strings.ToLower(compType), "source"):
			estimate = "Low-Medium (reading data)"
			recommendation = "Optimize source query if possible"
		case strings.Contains(strings.ToLower(compType), "destination"):
			estimate = "Low-Medium (writing data)"
			recommendation = "Configure appropriate batch sizes"
		default:
			estimate = "Medium (standard transformation)"
			recommendation = ""
		}

		memoryAnalysis = append(memoryAnalysis, ComponentMemory{
			Component:      compType,
			Estimate:       estimate,
			Recommendation: recommendation,
		})
	}

	return memoryAnalysis
}

func detectMemoryIntensiveOperations(task Task) []string {
	var issues []string

	if task.ObjectData.DataFlow.Components.Components != nil {
		for _, comp := range task.ObjectData.DataFlow.Components.Components {
			compType := getComponentType(comp.ComponentClassID)

			// Check for memory-intensive operations
			if strings.Contains(strings.ToLower(compType), "sort") {
				issues = append(issues, fmt.Sprintf("%s: Sorting operations can be memory-intensive", compType))
			}
			if strings.Contains(strings.ToLower(compType), "aggregate") {
				issues = append(issues, fmt.Sprintf("%s: Aggregation operations accumulate data in memory", compType))
			}
			if strings.Contains(strings.ToLower(compType), "lookup") {
				issues = append(issues, fmt.Sprintf("%s: Lookup caching can consume significant memory", compType))
			}
		}
	}

	return issues
}

// performPackageAnalysis performs a basic analysis of a DTSX package
func performPackageAnalysis(filePath, packageDirectory string) (map[string]interface{}, error) {
	fullPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse DTSX file: %w", err)
	}

	// Extract package name from properties
	packageName := "Unknown"
	for _, prop := range pkg.Properties {
		if prop.Name == "Name" {
			packageName = strings.TrimSpace(prop.Value)
			break
		}
	}

	// Perform basic analysis
	result := map[string]interface{}{
		"package_name":     packageName,
		"task_count":       len(pkg.Executables.Tasks),
		"connection_count": len(pkg.ConnectionMgr.Connections),
		"variable_count":   len(pkg.Variables.Vars),
		"parameter_count":  len(pkg.Parameters.Params),
	}

	// Count different task types
	taskTypes := make(map[string]int)
	for _, task := range pkg.Executables.Tasks {
		taskType := getTaskType(task)
		taskTypes[taskType]++
	}
	result["task_types"] = taskTypes

	return result, nil
}

// formatBatchSummaryAsText formats batch summary as plain text
func formatBatchSummaryAsText(summary BatchSummary) string {
	var output strings.Builder

	output.WriteString("Batch Analysis Summary\n")
	output.WriteString("=====================\n\n")
	output.WriteString(fmt.Sprintf("Total Packages: %d\n", summary.TotalPackages))
	output.WriteString(fmt.Sprintf("Successful: %d\n", summary.Successful))
	output.WriteString(fmt.Sprintf("Failed: %d\n", summary.Failed))
	output.WriteString(fmt.Sprintf("Total Duration: %v\n", summary.TotalDuration))
	output.WriteString(fmt.Sprintf("Average Duration: %v\n", summary.AverageDuration))

	if len(summary.Errors) > 0 {
		output.WriteString("\nErrors:\n")
		for _, err := range summary.Errors {
			output.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	output.WriteString("\nPackage Details:\n")
	for _, pkg := range summary.PackageSummaries {
		status := "âœ“"
		if !pkg.Success {
			status = "âœ—"
		}
		output.WriteString(fmt.Sprintf("%s %s (%v)\n", status, pkg.PackagePath, pkg.Duration))
		if !pkg.Success {
			output.WriteString(fmt.Sprintf("  Error: %s\n", pkg.Error))
		}
	}

	return output.String()
}

// formatBatchSummaryAsCSV formats batch summary as CSV
func formatBatchSummaryAsCSV(summary BatchSummary) string {
	var output strings.Builder

	// Summary header
	output.WriteString("Summary\n")
	output.WriteString("Total Packages,Successful,Failed,Total Duration,Average Duration\n")
	output.WriteString(fmt.Sprintf("%d,%d,%d,%v,%v\n\n",
		summary.TotalPackages, summary.Successful, summary.Failed,
		summary.TotalDuration, summary.AverageDuration))

	// Errors
	if len(summary.Errors) > 0 {
		output.WriteString("Errors\n")
		for _, err := range summary.Errors {
			output.WriteString(fmt.Sprintf("\"%s\"\n", strings.ReplaceAll(err, "\"", "\"\"")))
		}
		output.WriteString("\n")
	}

	// Package details
	output.WriteString("Package Details\n")
	output.WriteString("Package Path,Success,Error,Duration\n")
	for _, pkg := range summary.PackageSummaries {
		errorStr := ""
		if pkg.Error != "" {
			errorStr = strings.ReplaceAll(pkg.Error, "\"", "\"\"")
		}
		output.WriteString(fmt.Sprintf("\"%s\",%t,\"%s\",%v\n",
			pkg.PackagePath, pkg.Success, errorStr, pkg.Duration))
	}

	return output.String()
}

// formatBatchSummaryAsHTML formats batch summary as HTML
func formatBatchSummaryAsHTML(summary BatchSummary) string {
	var output strings.Builder

	output.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>Batch Analysis Summary</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f0f0f0; padding: 10px; border-radius: 5px; }
        .success { color: green; }
        .error { color: red; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .status-icon { font-weight: bold; }
    </style>
</head>
<body>
    <h1>Batch Analysis Summary</h1>
    
    <div class="summary">
        <h2>Overview</h2>
        <p>Total Packages: `)
	output.WriteString(fmt.Sprintf("%d</p>", summary.TotalPackages))
	output.WriteString(fmt.Sprintf("<p>Successful: <span class=\"success\">%d</span></p>", summary.Successful))
	output.WriteString(fmt.Sprintf("<p>Failed: <span class=\"error\">%d</span></p>", summary.Failed))
	output.WriteString(fmt.Sprintf("<p>Total Duration: %v</p>", summary.TotalDuration))
	output.WriteString(fmt.Sprintf("<p>Average Duration: %v</p>", summary.AverageDuration))
	output.WriteString(`    </div>`)

	if len(summary.Errors) > 0 {
		output.WriteString(`
    <h2>Errors</h2>
    <ul>`)
		for _, err := range summary.Errors {
			output.WriteString(fmt.Sprintf("<li>%s</li>", html.EscapeString(err)))
		}
		output.WriteString(`    </ul>`)
	}

	output.WriteString(`
    <h2>Package Details</h2>
    <table>
        <tr>
            <th>Package Path</th>
            <th>Status</th>
            <th>Duration</th>
            <th>Error</th>
        </tr>`)

	for _, pkg := range summary.PackageSummaries {
		status := `<span class="status-icon success">âœ“</span>`
		if !pkg.Success {
			status = `<span class="status-icon error">âœ—</span>`
		}
		errorCell := ""
		if pkg.Error != "" {
			errorCell = html.EscapeString(pkg.Error)
		}
		output.WriteString(fmt.Sprintf(`
        <tr>
            <td>%s</td>
            <td>%s</td>
            <td>%v</td>
            <td>%s</td>
        </tr>`, html.EscapeString(pkg.PackagePath), status, pkg.Duration, errorCell))
	}

	output.WriteString(`
    </table>
</body>
</html>`)

	return output.String()
}

// formatBatchSummaryAsMarkdown formats batch summary as Markdown
func formatBatchSummaryAsMarkdown(summary BatchSummary) string {
	var output strings.Builder

	output.WriteString("# Batch Analysis Summary\n\n")
	output.WriteString("## Overview\n\n")
	output.WriteString(fmt.Sprintf("- **Total Packages**: %d\n", summary.TotalPackages))
	output.WriteString(fmt.Sprintf("- **Successful**: %d\n", summary.Successful))
	output.WriteString(fmt.Sprintf("- **Failed**: %d\n", summary.Failed))
	output.WriteString(fmt.Sprintf("- **Total Duration**: %v\n", summary.TotalDuration))
	output.WriteString(fmt.Sprintf("- **Average Duration**: %v\n", summary.AverageDuration))

	if len(summary.Errors) > 0 {
		output.WriteString("\n## Errors\n\n")
		for _, err := range summary.Errors {
			output.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	output.WriteString("\n## Package Details\n\n")
	output.WriteString("| Package Path | Status | Duration | Error |\n")
	output.WriteString("|--------------|--------|----------|-------|\n")

	for _, pkg := range summary.PackageSummaries {
		status := "âœ…"
		if !pkg.Success {
			status = "âŒ"
		}
		errorCell := ""
		if pkg.Error != "" {
			errorCell = pkg.Error
		}
		output.WriteString(fmt.Sprintf("| %s | %s | %v | %s |\n",
			pkg.PackagePath, status, pkg.Duration, errorCell))
	}

	return output.String()
}
