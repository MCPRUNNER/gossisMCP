package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"

	"github.com/MCPRUNNER/gossisMCP/pkg/config"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/analysis"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/extraction"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/optimization"
	packagehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/packages"
	templatehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/templates"
	"github.com/MCPRUNNER/gossisMCP/pkg/workflow"
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
	Directory   string `json:"directory" yaml:"directory"`
	ExcludeFile string `json:"exclude_file" yaml:"exclude_file"`
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
			Directory:   "",
			ExcludeFile: "",
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

	analyzeCodeQualityTool := mcp.NewTool("mcp_ssis-analyzer_analyze_code_quality",
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

	// Batch Processing Tools

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
		runHTTPServer(s, config.Server.Port)
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

type workflowStepSummary struct {
	Name    string                         `json:"name"`
	Type    string                         `json:"type"`
	Enabled bool                           `json:"enabled"`
	Outputs map[string]workflow.StepResult `json:"outputs,omitempty"`
}

type workflowExecutionSummary struct {
	WorkflowPath string                `json:"workflow_path"`
	Steps        []workflowStepSummary `json:"steps"`
	FilesWritten []string              `json:"files_written,omitempty"`
}

func handleWorkflowRunner(ctx context.Context, request mcp.CallToolRequest, packageDirectory, excludeFile string) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})

	workflowPath := extractStringArg(args, "file_path")
	if workflowPath == "" {
		workflowPath = extractStringArg(args, "workflow_file")
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
		normalized := cloneArguments(params)

		normalizeWorkflowPathArg(normalized, workflowPath, "directory")
		normalizeWorkflowPathArg(normalized, workflowPath, "file_path")
		normalizeWorkflowPathArg(normalized, workflowPath, "outputFilePath")
		normalizeWorkflowPathArg(normalized, workflowPath, "templateFilePath")
		normalizeWorkflowPathArg(normalized, workflowPath, "output_file_path")
		normalizeWorkflowPathArg(normalized, workflowPath, "template_file_path")
		normalizeWorkflowPathArg(normalized, workflowPath, "json_file_path")
		normalizeWorkflowPathArg(normalized, workflowPath, "jsonFilePath")

		if tool == "list_packages" {
			if format := stringFromAny(normalized["format"]); format == "" {
				normalized["format"] = "json"
			}
			if dir := stringFromAny(normalized["directory"]); dir == "" && packageDirectory != "" {
				normalized["directory"] = packageDirectory
			}
		}

		if tool == "batch_analyze" {
			if rawJSON, ok := normalized["jsonData"]; ok {
				jsonText, ok := rawJSON.(string)
				if !ok {
					return "", fmt.Errorf("batch_analyze: jsonData must be a string value")
				}
				files, err := extractFilePathsFromJSON(jsonText)
				if err != nil {
					return "", err
				}
				normalized["file_paths"] = toInterfaceSlice(files)
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
		default:
			return "", fmt.Errorf("workflow runner: tool %q is not supported", tool)
		}

		text, err := workflow.ToolResultToString(result)
		if err != nil {
			return "", err
		}

		renderedPath := ""
		if tool == "render_template" {
			renderedPath = stringFromAny(normalized["outputFilePath"])
			if renderedPath == "" {
				renderedPath = stringFromAny(normalized["output_file_path"])
			}
		}

		outputPath := stringFromAny(normalized["outputFilePath"])
		if outputPath == "" {
			outputPath = stringFromAny(normalized["output_file_path"])
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
					if err := writeWorkflowOutput(outputPath, text); err != nil {
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

	summary := createWorkflowExecutionSummary(workflowPath, wf, results, writtenOutputs)

	format := strings.ToLower(extractStringArg(args, "format"))
	switch format {
	case "json":
		data, marshalErr := json.MarshalIndent(summary, "", "  ")
		if marshalErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal workflow summary: %v", marshalErr)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	default:
		markdown := formatWorkflowSummaryMarkdown(summary)
		return mcp.NewToolResultText(markdown), nil
	}
}

func cloneArguments(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

func normalizeWorkflowPathArg(args map[string]interface{}, workflowPath, key string) {
	raw, exists := args[key]
	if !exists {
		return
	}
	value, ok := raw.(string)
	if !ok {
		return
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	args[key] = workflow.ResolveRelativePath(workflowPath, trimmed)
}

func stringFromAny(value interface{}) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func extractStringArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	return stringFromAny(args[key])
}

func extractFilePathsFromJSON(jsonText string) ([]string, error) {
	var payload struct {
		Directory        string   `json:"directory"`
		Packages         []string `json:"packages"`
		PackagesAbsolute []string `json:"packages_absolute"`
	}

	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse jsonData payload: %w", err)
	}

	var files []string
	if len(payload.PackagesAbsolute) > 0 {
		for _, path := range payload.PackagesAbsolute {
			if path == "" {
				continue
			}
			files = append(files, filepath.Clean(path))
		}
	} else {
		base := payload.Directory
		for _, pkg := range payload.Packages {
			if pkg == "" {
				continue
			}
			if filepath.IsAbs(pkg) || base == "" {
				files = append(files, filepath.Clean(pkg))
			} else {
				files = append(files, filepath.Clean(filepath.Join(base, pkg)))
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("jsonData did not contain any package paths")
	}

	return files, nil
}

func toInterfaceSlice(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func writeWorkflowOutput(outputPath, content string) error {
	if outputPath == "" {
		return nil
	}
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create workflow output directory %s: %w", dir, err)
	}
	// Overwrite the output file so each workflow run produces consistent,
	// self-contained JSON files rather than appending mixed content.
	if err := os.WriteFile(outputPath, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write workflow output %s: %w", outputPath, err)
	}
	return nil
}

func createWorkflowExecutionSummary(workflowPath string, wf *workflow.Workflow, results map[string]map[string]workflow.StepResult, files []string) workflowExecutionSummary {
	summaries := make([]workflowStepSummary, 0, len(wf.Steps))
	for _, step := range wf.Steps {
		summary := workflowStepSummary{
			Name:    step.Name,
			Type:    step.Type,
			Enabled: step.Enabled,
		}
		if step.Enabled {
			if outputs, ok := results[step.Name]; ok && len(outputs) > 0 {
				summary.Outputs = outputs
			}
		}
		summaries = append(summaries, summary)
	}

	return workflowExecutionSummary{
		WorkflowPath: workflowPath,
		Steps:        summaries,
		FilesWritten: files,
	}
}

func formatWorkflowSummaryMarkdown(summary workflowExecutionSummary) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("# Workflow Execution: %s\n\n", filepath.Base(summary.WorkflowPath)))
	builder.WriteString(fmt.Sprintf("- **Workflow Path**: %s\n", summary.WorkflowPath))
	builder.WriteString(fmt.Sprintf("- **Total Steps**: %d\n", len(summary.Steps)))

	executed := 0
	for _, step := range summary.Steps {
		if step.Enabled {
			executed++
		}
	}
	builder.WriteString(fmt.Sprintf("- **Steps Executed**: %d\n\n", executed))

	if len(summary.FilesWritten) > 0 {
		builder.WriteString("## Files Written\n\n")
		for _, file := range summary.FilesWritten {
			builder.WriteString(fmt.Sprintf("- %s\n", file))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Step Outputs\n\n")
	for _, step := range summary.Steps {
		builder.WriteString(fmt.Sprintf("### %s (%s)\n\n", step.Name, strings.TrimPrefix(step.Type, "#")))
		if !step.Enabled {
			builder.WriteString("_Step disabled_\n\n")
			continue
		}
		if len(step.Outputs) == 0 {
			builder.WriteString("_No outputs captured._\n\n")
			continue
		}

		keys := make([]string, 0, len(step.Outputs))
		for key := range step.Outputs {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			output := step.Outputs[key]
			builder.WriteString(fmt.Sprintf("- **%s**", key))
			if output.Format != "" {
				builder.WriteString(fmt.Sprintf(" (%s)", output.Format))
			}
			builder.WriteString("\n\n```")
			builder.WriteString("\n")
			builder.WriteString(output.Value)
			builder.WriteString("\n```")
			builder.WriteString("\n\n")
		}
	}

	return builder.String()
}

// extractJSONObjects scans input for top-level JSON objects and returns them
// as raw JSON strings. It attempts to be resilient to braces inside strings
// by tracking string quoting and escape characters.
func extractJSONObjects(s string) []string {
	var out []string
	i := 0
	length := len(s)
	for i < length {
		// Find next '{'
		start := strings.IndexByte(s[i:], '{')
		if start == -1 {
			break
		}
		start += i
		depth := 0
		inString := false
		escaped := false
		found := false
		for j := start; j < length; j++ {
			c := s[j]
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					obj := strings.TrimSpace(s[start : j+1])
					out = append(out, obj)
					i = j + 1
					found = true
					break
				}
			}
		}
		if !found {
			break
		}
	}
	return out
}

// parseTopLevelJSONValues decodes one or more top-level JSON values from the
// provided string. It returns a slice with each decoded value. This handles
// concatenated JSON objects, arrays, and primitive values robustly.
func parseTopLevelJSONValues(s string) ([]interface{}, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var out []interface{}
	for {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func isFileBinary(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 512 bytes (or less if file is smaller)
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Check for null bytes
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true, nil
		}
	}

	return false, nil
}
func convertToLines(content string, isLineNumberNeeded bool) []string {
	lines := strings.Split(content, "\n")
	var rawLines []string
	if len(lines) == 0 {
		return rawLines
	}
	if !isLineNumberNeeded {
		for _, line := range lines {
			rawLines = append(rawLines, fmt.Sprintf(" %s", line))
		}
	} else {
		for i, line := range lines {
			rawLines = append(rawLines, fmt.Sprintf("%4d: %s", i+1, line))
		}
	}

	return rawLines
}
func handleReadTextFile(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	isLineNumberNeeded := request.GetBool("line_numbers", true)

	// Resolve the file path against the package directory
	resolvedPath := resolveFilePath(filePath, packageDirectory)
	isBinary, err := isFileBinary(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to check if file is binary: %v", err)), nil
	}
	if isBinary {
		return mcp.NewToolResultError("File is binary, cannot read as text"), nil
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("📄 Text File Analysis\n\n")
	result.WriteString(fmt.Sprintf("File: %s\n", filepath.Base(resolvedPath)))
	result.WriteString(fmt.Sprintf("Path: %s\n\n", resolvedPath))

	content := string(data)
	lines := strings.Split(content, "\n")
	result.WriteString("📊 File Statistics:\n")
	result.WriteString(fmt.Sprintf("• Total Lines: %d\n", len(lines)))
	result.WriteString(fmt.Sprintf("• Total Characters: %d\n", len(content)))
	result.WriteString(fmt.Sprintf("• File Size: %d bytes\n\n", len(data)))

	// Detect file type and parse accordingly
	ext := strings.ToLower(filepath.Ext(resolvedPath))
	switch ext {
	case ".bat", ".cmd":
		result.WriteString("🛠 Batch File Analysis:\n")
		analyzeBatchFile(content, isLineNumberNeeded, &result)
	case ".config", ".cfg":
		result.WriteString("⚙️ Configuration File Analysis:\n")
		analyzeConfigFile(content, isLineNumberNeeded, &result)
	case ".sql":
		result.WriteString("🗄️ SQL File Analysis:\n")
		analyzeSQLFile(content, isLineNumberNeeded, &result)
	default:
		result.WriteString("📘 Text File Content:\n")
		analyzeGenericTextFile(content, isLineNumberNeeded, &result)
	}

	return mcp.NewToolResultText(result.String()), nil
}

func analyzeBatchFile(content string, isLineNumberNeeded bool, result *strings.Builder) {
	lines := strings.Split(content, "\n")
	//var lines []string
	//lines = convertToLines(content,isLineNumberNeeded)

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

	result.WriteString(fmt.Sprintf("• Variables Set: %d\n", len(variables)))
	if len(variables) > 0 {
		result.WriteString("  Variables:\n")
		for _, v := range variables {
			result.WriteString(fmt.Sprintf("    %s\n", v))
		}
	}

	result.WriteString(fmt.Sprintf("• Function Calls: %d\n", len(calls)))
	if len(calls) > 0 {
		result.WriteString("  Calls:\n")
		for _, c := range calls {
			result.WriteString(fmt.Sprintf("    %s\n", c))
		}
	}

	result.WriteString(fmt.Sprintf("• Executable Commands: %d\n", len(commands)))
	if len(commands) > 0 {
		result.WriteString("  Commands:\n")
		for _, c := range commands {
			result.WriteString(fmt.Sprintf("    %s\n", c))
		}
	}

	result.WriteString(fmt.Sprintf("• Content Lines: %d\n", len(lines)))
	for i, c := range lines {
		if isLineNumberNeeded {
			result.WriteString(fmt.Sprintf("    %d: %s\n", i+1, c))
			continue
		}
		result.WriteString(fmt.Sprintf("    %s\n", c))
	}

}

func analyzeConfigFile(content string, isLineNumberNeeded bool, result *strings.Builder) {
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

	result.WriteString(fmt.Sprintf("• Configuration Sections: %d\n", len(sections)))
	if len(sections) > 0 {
		result.WriteString("  Sections:\n")
		for _, s := range sections {
			result.WriteString(fmt.Sprintf("    %s\n", s))
		}
	}
	result.WriteString(fmt.Sprintf("• Content Lines: %d\n", len(lines)))
	for i, c := range lines {
		if isLineNumberNeeded {
			result.WriteString(fmt.Sprintf("    %d: %s\n", i+1, c))
			continue
		}
		result.WriteString(fmt.Sprintf("    %s\n", c))
	}
	result.WriteString(fmt.Sprintf("• Key-Value Pairs: %d\n", len(keyValues)))
	if len(keyValues) > 0 {
		result.WriteString("  Settings:\n")
		for _, kv := range keyValues {
			result.WriteString(fmt.Sprintf("    %s\n", kv))
		}
	}
}

func analyzeSQLFile(content string, isLineNumberNeeded bool, result *strings.Builder) {

	lines := strings.Split(content, "\n")
	upperContent := strings.ToUpper(content)

	// Count different types of SQL statements
	selectCount := strings.Count(upperContent, "SELECT ")
	insertCount := strings.Count(upperContent, "INSERT ")
	updateCount := strings.Count(upperContent, "UPDATE ")
	deleteCount := strings.Count(upperContent, "DELETE ")
	createCount := strings.Count(upperContent, "CREATE ")

	result.WriteString("• SQL Statement Counts:\n")
	result.WriteString(fmt.Sprintf("  - SELECT statements: %d\n", selectCount))
	result.WriteString(fmt.Sprintf("  - INSERT statements: %d\n", insertCount))
	result.WriteString(fmt.Sprintf("  - UPDATE statements: %d\n", updateCount))
	result.WriteString(fmt.Sprintf("  - DELETE statements: %d\n", deleteCount))
	result.WriteString(fmt.Sprintf("  - CREATE statements: %d\n", createCount))

	// Check for potential SSIS-related patterns
	if strings.Contains(upperContent, "EXECUTE") || strings.Contains(upperContent, "SP_") {
		result.WriteString("• Contains stored procedure calls\n")
	}

	if strings.Contains(upperContent, "BULK INSERT") {
		result.WriteString("• Contains bulk operations\n")
	}
	result.WriteString(fmt.Sprintf("• Content Lines: %d\n", len(lines)))
	for i, c := range lines {
		if isLineNumberNeeded {
			result.WriteString(fmt.Sprintf("    %d: %s\n", i+1, c))
			continue
		}
		result.WriteString(fmt.Sprintf("    %s\n", c))
	}
}

func analyzeGenericTextFile(content string, isLineNumberNeeded bool, result *strings.Builder) {
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

	result.WriteString(fmt.Sprintf("• Non-empty Lines: %d\n", nonEmptyLines))
	result.WriteString(fmt.Sprintf("• Total Words: %d\n", totalWords))
	result.WriteString(fmt.Sprintf("• Average Words per Line: %.1f\n\n", float64(totalWords)/float64(nonEmptyLines)))

	result.WriteString("📄 Content:\n")
	for i, line := range lines {
		if isLineNumberNeeded {
			result.WriteString(fmt.Sprintf("%4d: %s\n", i+1, strings.TrimRight(line, "\r\n")))
			continue
		}
		result.WriteString(fmt.Sprintf("%s\n", strings.TrimRight(line, "\r\n")))
	}
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
