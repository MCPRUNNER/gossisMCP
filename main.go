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
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// resolveFilePath resolves a file path against the package directory if it's relative
func resolveFilePath(filePath, packageDirectory string) string {
	if packageDirectory == "" || filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(packageDirectory, filePath)
}

// SSISPackage represents the root of a DTSX file
type SSISPackage struct {
	XMLName       xml.Name      `xml:"Executable"`
	Properties    []Property    `xml:"Property"`
	ConnectionMgr ConnectionMgr `xml:"ConnectionManagers"`
	Variables     Variables     `xml:"Variables"`
	Executables   Executables   `xml:"Executables"`
}

type Property struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:",innerxml"`
}

type ConnectionMgr struct {
	Connections []Connection `xml:"ConnectionManager"`
}

type Connection struct {
	Name       string     `xml:"ObjectName,attr"`
	ObjectData ObjectData `xml:"ObjectData"`
}

type ObjectData struct {
	ConnectionMgr InnerConnection `xml:"ConnectionManager"`
	MsmqConnMgr   MsmqConnection  `xml:"MsmqConnectionManager"`
}

type InnerConnection struct {
	ConnectionString string `xml:"ConnectionString,attr"`
}

type MsmqConnection struct {
	ConnectionString string `xml:"ConnectionString,attr"`
}

type Executables struct {
	Tasks []Task `xml:"Executable"`
}

type Task struct {
	Name       string         `xml:"ObjectName,attr"`
	Properties []Property     `xml:"Property"`
	ObjectData TaskObjectData `xml:"ObjectData"`
}

type TaskObjectData struct {
	Task       TaskDetails       `xml:"Task"`
	ScriptTask ScriptTaskDetails `xml:"ScriptTask"`
}

type TaskDetails struct {
	MessageQueueTask MessageQueueTaskDetails `xml:"MessageQueueTask"`
}

type MessageQueueTaskDetails struct {
	MessageQueueTaskData MessageQueueTaskData `xml:"MessageQueueTaskData"`
}

type MessageQueueTaskData struct {
	MessageType string `xml:"MessageType,attr"`
	Message     string `xml:"Message"`
}

type ScriptTaskDetails struct {
	ScriptTaskData ScriptTaskData `xml:"ScriptTaskData"`
}

type ScriptTaskData struct {
	ScriptProject ScriptProject `xml:"ScriptProject"`
}

type ScriptProject struct {
	ScriptCode string `xml:",innerxml"`
}

type Variables struct {
	Vars []Variable `xml:"Variable"`
}

type Variable struct {
	Name  string `xml:"ObjectName,attr"`
	Value string `xml:"VariableValue"`
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
		mcp.WithDescription("Extract and list all tasks from a DTSX file"),
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
		mcp.WithDescription("Extract and list all connection managers from a DTSX file"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractConnectionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractConnections(ctx, request, packageDirectory)
	})

	// Tool to extract variables
	extractVariablesTool := mcp.NewTool("extract_variables",
		mcp.WithDescription("Extract and list all variables from a DTSX file"),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Path to the DTSX file (relative to package directory if set)"),
		),
	)
	s.AddTool(extractVariablesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleExtractVariables(ctx, request, packageDirectory)
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
	}

	return mcp.NewToolResultText(connections), nil
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
		variables += fmt.Sprintf("%d. %s = %s\n", i+1, v.Name, v.Value)
	}

	return mcp.NewToolResultText(variables), nil
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
