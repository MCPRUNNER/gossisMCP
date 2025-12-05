package packages

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/MCPRUNNER/gossisMCP/pkg/formatter"
	"github.com/MCPRUNNER/gossisMCP/pkg/types"
)

// HandleValidateBestPractices performs a simple best-practices sweep of an SSIS package.
func HandleValidateBestPractices(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("validate_best_practices", filePath, nil, err)
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "validate_best_practices",
				"file_path": filePath,
				"package":   filepath.Base(filePath),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     err.Error(),
			}
			return mcp.NewToolResultStructured(jsonResult, "Best practices validation error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	cleaned := strings.ReplaceAll(string(data), "DTS:", "")
	cleaned = strings.ReplaceAll(cleaned, `xmlns="www.microsoft.com/SqlServer/Dts"`, "")

	var pkg types.SSISPackage
	if err := xml.Unmarshal([]byte(cleaned), &pkg); err != nil {
		result := formatter.CreateAnalysisResult("validate_best_practices", filePath, nil, fmt.Errorf("failed to parse XML: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "validate_best_practices",
				"file_path": filePath,
				"package":   filepath.Base(filePath),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("Failed to parse XML: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "Best practices validation error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var report strings.Builder
	report.WriteString("Best Practices Validation Report:\n")

	if len(pkg.Variables.Vars) == 0 {
		report.WriteString("- WARNING: No user-defined variables found\n")
	} else {
		report.WriteString(fmt.Sprintf("- OK: %d variables defined\n", len(pkg.Variables.Vars)))
	}

	if len(pkg.ConnectionMgr.Connections) == 0 {
		report.WriteString("- WARNING: No connection managers defined\n")
	} else {
		report.WriteString(fmt.Sprintf("- OK: %d connection managers defined\n", len(pkg.ConnectionMgr.Connections)))
	}

	if len(pkg.Executables.Tasks) == 0 {
		report.WriteString("- ERROR: No executable tasks found\n")
	} else {
		report.WriteString(fmt.Sprintf("- OK: %d tasks defined\n", len(pkg.Executables.Tasks)))
	}

	if strings.Contains(cleaned, "LoggingOptions") {
		report.WriteString("- OK: Logging configuration detected\n")
	} else {
		report.WriteString("- WARNING: No logging configuration found\n")
	}

	report.WriteString("- Note: This is a basic validation. Review SSIS best-practices for deeper guidance.\n")

	analysisResult := formatter.CreateAnalysisResult("validate_best_practices", filePath, report.String(), nil)

	// For JSON format, return structured data
	if format == formatter.FormatJSON {
		jsonResult := map[string]interface{}{
			"tool_name": analysisResult.ToolName,
			"file_path": analysisResult.FilePath,
			"package":   filepath.Base(analysisResult.FilePath),
			"timestamp": analysisResult.Timestamp,
			"status":    analysisResult.Status,
			"analysis":  analysisResult.Data,
		}
		if analysisResult.Error != "" {
			jsonResult["error"] = analysisResult.Error
		}
		return mcp.NewToolResultStructured(jsonResult, "Best practices validation"), nil
	}

	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAskAboutDtsx answers lightweight questions about a DTSX file.
func HandleAskAboutDtsx(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	question, err := request.RequireString("question")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("ask_about_dtsx", filePath, nil, err)
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "ask_about_dtsx",
				"file_path": filePath,
				"package":   filepath.Base(filePath),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     err.Error(),
			}
			return mcp.NewToolResultStructured(jsonResult, "DTSX query error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	cleaned := strings.ReplaceAll(string(data), "DTS:", "")
	cleaned = strings.ReplaceAll(cleaned, `xmlns="www.microsoft.com/SqlServer/Dts"`, "")

	var pkg types.SSISPackage
	if err := xml.Unmarshal([]byte(cleaned), &pkg); err != nil {
		result := formatter.CreateAnalysisResult("ask_about_dtsx", filePath, nil, fmt.Errorf("failed to parse XML: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "ask_about_dtsx",
				"file_path": filePath,
				"package":   filepath.Base(filePath),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("failed to parse XML: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "DTSX query error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	answer := strings.Builder{}
	prompt := strings.ToLower(question)

	switch {
	case strings.Contains(prompt, "task") || strings.Contains(prompt, "executables"):
		answer.WriteString("Tasks:\n")
		for i, task := range pkg.Executables.Tasks {
			answer.WriteString(fmt.Sprintf("%d. %s\n", i+1, task.Name))
			for _, prop := range task.Properties {
				if prop.Name == "Description" {
					answer.WriteString(fmt.Sprintf("   Description: %s\n", strings.TrimSpace(prop.Value)))
				}
			}
		}
	case strings.Contains(prompt, "connection"):
		answer.WriteString("Connections:\n")
		for i, conn := range pkg.ConnectionMgr.Connections {
			answer.WriteString(fmt.Sprintf("%d. %s\n", i+1, conn.Name))
			connStr := conn.ObjectData.ConnectionMgr.ConnectionString
			if connStr == "" {
				connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
			}
			answer.WriteString(fmt.Sprintf("   Connection String: %s\n", connStr))
		}
	case strings.Contains(prompt, "variable"):
		answer.WriteString("Variables:\n")
		for i, v := range pkg.Variables.Vars {
			answer.WriteString(fmt.Sprintf("%d. %s = %s\n", i+1, v.Name, v.Value))
		}
	case strings.Contains(prompt, "validate") || strings.Contains(prompt, "valid"):
		if len(pkg.Properties) == 0 {
			answer.WriteString("Validation: Warning - No properties found\n")
		} else {
			answer.WriteString("Validation: DTSX file structure appears valid\n")
		}
	default:
		answer.WriteString("Package Summary:\n")
		answer.WriteString(fmt.Sprintf("- Properties: %d\n", len(pkg.Properties)))
		answer.WriteString(fmt.Sprintf("- Connections: %d\n", len(pkg.ConnectionMgr.Connections)))
		answer.WriteString(fmt.Sprintf("- Tasks: %d\n", len(pkg.Executables.Tasks)))
		answer.WriteString(fmt.Sprintf("- Variables: %d\n", len(pkg.Variables.Vars)))

		for _, prop := range pkg.Properties {
			if prop.Name == "Name" || prop.Name == "Description" {
				answer.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, strings.TrimSpace(prop.Value)))
			}
		}

		for _, v := range pkg.Variables.Vars {
			answer.WriteString(fmt.Sprintf("  Variable %s: %s\n", v.Name, v.Value))
		}
	}

	analysisResult := formatter.CreateAnalysisResult("ask_about_dtsx", filePath, answer.String(), nil)

	// For JSON format, return structured data
	if format == formatter.FormatJSON {
		jsonResult := map[string]interface{}{
			"tool_name": analysisResult.ToolName,
			"file_path": analysisResult.FilePath,
			"package":   filepath.Base(analysisResult.FilePath),
			"timestamp": analysisResult.Timestamp,
			"status":    analysisResult.Status,
			"analysis":  analysisResult.Data,
		}
		if analysisResult.Error != "" {
			jsonResult["error"] = analysisResult.Error
		}
		return mcp.NewToolResultStructured(jsonResult, "DTSX query"), nil
	}

	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeMessageQueueTasks inspects Message Queue tasks inside a package.
func HandleAnalyzeMessageQueueTasks(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	cleaned := strings.ReplaceAll(string(data), "DTS:", "")

	var pkg types.SSISPackage
	if err := xml.Unmarshal([]byte(cleaned), &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var report strings.Builder
	report.WriteString("Message Queue Tasks Analysis:\n")

	found := false
	for i, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "message queue") {
			found = true
			report.WriteString(fmt.Sprintf("Task %d: %s\n", i+1, task.Name))

			mqData := task.ObjectData.Task.MessageQueueTask.MessageQueueTaskData
			if mqData.MessageType != "" {
				report.WriteString(fmt.Sprintf("  Message Type: %s\n", mqData.MessageType))
			}
			if mqData.Message != "" {
				report.WriteString(fmt.Sprintf("  Message Content: %s\n", mqData.Message))
			}

			for _, prop := range task.Properties {
				if prop.Name == "Description" {
					report.WriteString(fmt.Sprintf("  Description: %s\n", strings.TrimSpace(prop.Value)))
				}
			}
		}
	}

	if !found {
		report.WriteString("No Message Queue Tasks found in this package.\n")
	}

	return mcp.NewToolResultText(report.String()), nil
}

// HandleAnalyzeScriptTask extracts script task details.
func HandleAnalyzeScriptTask(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	cleaned := strings.ReplaceAll(string(data), "DTS:", "")

	var pkg types.SSISPackage
	if err := xml.Unmarshal([]byte(cleaned), &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var report strings.Builder
	report.WriteString("Script Tasks Analysis:\n")

	found := false
	for i, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "script") {
			found = true
			report.WriteString(fmt.Sprintf("Task %d: %s\n", i+1, task.Name))

			for _, prop := range task.Properties {
				if prop.Name == "Description" {
					report.WriteString(fmt.Sprintf("  Description: %s\n", strings.TrimSpace(prop.Value)))
				}
			}

			scriptTaskData := task.ObjectData.ScriptTask.ScriptTaskData
			if scriptTaskData.ScriptProject.ScriptCode != "" {
				code := strings.TrimSpace(scriptTaskData.ScriptProject.ScriptCode)
				code = strings.ReplaceAll(code, "&lt;", "<")
				code = strings.ReplaceAll(code, "&gt;", ">")
				code = strings.ReplaceAll(code, "&amp;", "&")
				report.WriteString("  Script Code:\n")
				report.WriteString(fmt.Sprintf("    %s\n", code))
			} else {
				report.WriteString("  Script Code: not present\n")
			}

			rawData := string(data)
			taskStart := strings.Index(rawData, fmt.Sprintf("<Executable Name=\"%s\"", task.Name))
			if taskStart != -1 {
				taskEnd := strings.Index(rawData[taskStart:], "</Executable>")
				if taskEnd != -1 {
					taskXML := rawData[taskStart : taskStart+taskEnd+len("</Executable>")]

					if strings.Contains(taskXML, "ReadOnlyVariables") {
						value := extractPropertyValue(taskXML, "ReadOnlyVariables")
						if value != "" {
							report.WriteString(fmt.Sprintf("  ReadOnly Variables: %s\n", value))
						}
					}
					if strings.Contains(taskXML, "ReadWriteVariables") {
						value := extractPropertyValue(taskXML, "ReadWriteVariables")
						if value != "" {
							report.WriteString(fmt.Sprintf("  ReadWrite Variables: %s\n", value))
						}
					}
					if strings.Contains(taskXML, "EntryPoint") {
						value := extractPropertyValue(taskXML, "EntryPoint")
						if value != "" {
							report.WriteString(fmt.Sprintf("  Entry Point: %s\n", value))
						}
					}
					if strings.Contains(taskXML, "ScriptLanguage") {
						value := extractPropertyValue(taskXML, "ScriptLanguage")
						if value != "" {
							report.WriteString(fmt.Sprintf("  Script Language: %s\n", value))
						}
					}
				}
			}

			report.WriteString("\n")
		}
	}

	if !found {
		report.WriteString("No Script Tasks found in this package.\n")
	}

	return mcp.NewToolResultText(report.String()), nil
}

// HandleDetectHardcodedValues scans for obvious literal values.
func HandleDetectHardcodedValues(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("detect_hardcoded_values", filePath, nil, err)
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "detect_hardcoded_values",
				"file_path": filePath,
				"package":   filepath.Base(filePath),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     err.Error(),
			}
			return mcp.NewToolResultStructured(jsonResult, "Hard-coded values detection error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	cleaned := strings.ReplaceAll(string(data), "DTS:", "")

	var pkg types.SSISPackage
	if err := xml.Unmarshal([]byte(cleaned), &pkg); err != nil {
		result := formatter.CreateAnalysisResult("detect_hardcoded_values", filePath, nil, fmt.Errorf("failed to parse XML: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "detect_hardcoded_values",
				"file_path": filePath,
				"package":   filepath.Base(filePath),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("failed to parse XML: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "Hard-coded values detection error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var report strings.Builder
	report.WriteString("Hard-coded Values Detection Report:\n")

	found := false

	for _, conn := range pkg.ConnectionMgr.Connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		if strings.Contains(connStr, "localhost") || strings.Contains(connStr, "127.0.0.1") || strings.Contains(strings.ToLower(connStr), "hardcoded") {
			report.WriteString(fmt.Sprintf("- WARNING: Connection '%s' contains literal value: %s\n", conn.Name, connStr))
			found = true
		}
	}

	for _, v := range pkg.Variables.Vars {
		valueLower := strings.ToLower(v.Value)
		if strings.Contains(valueLower, "c:\\") || strings.Contains(valueLower, "localhost") {
			report.WriteString(fmt.Sprintf("- WARNING: Variable '%s' contains literal path/value: %s\n", v.Name, v.Value))
			found = true
		}
	}

	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(strings.ToLower(task.Name), "message queue") {
			message := task.ObjectData.Task.MessageQueueTask.MessageQueueTaskData.Message
			if message != "" && !strings.Contains(message, "@[") {
				report.WriteString(fmt.Sprintf("- WARNING: Message Queue Task '%s' contains literal message: %s\n", task.Name, message))
				found = true
			}
		}
		for _, prop := range task.Properties {
			valueLower := strings.ToLower(prop.Value)
			if strings.Contains(valueLower, "localhost") || strings.Contains(prop.Value, "127.0.0.1") {
				report.WriteString(fmt.Sprintf("- WARNING: Task '%s' property '%s' contains literal value: %s\n", task.Name, prop.Name, prop.Value))
				found = true
			}
		}
	}

	if !found {
		report.WriteString("No obvious hard-coded values detected. Manual review recommended for sensitive scenarios.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("detect_hardcoded_values", filePath, report.String(), nil)

	// For JSON format, return structured data
	if format == formatter.FormatJSON {
		jsonResult := map[string]interface{}{
			"tool_name": analysisResult.ToolName,
			"file_path": analysisResult.FilePath,
			"package":   filepath.Base(analysisResult.FilePath),
			"timestamp": analysisResult.Timestamp,
			"status":    analysisResult.Status,
			"analysis":  analysisResult.Data,
		}
		if analysisResult.Error != "" {
			jsonResult["error"] = analysisResult.Error
		}
		return mcp.NewToolResultStructured(jsonResult, "Hard-coded values detection"), nil
	}

	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeLoggingConfiguration reviews logging configuration blocks.
func HandleAnalyzeLoggingConfiguration(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resolvedPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	cleaned := strings.ReplaceAll(string(data), "DTS:", "")

	var pkg types.SSISPackage
	if err := xml.Unmarshal([]byte(cleaned), &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var report strings.Builder
	report.WriteString("Logging Configuration Analysis:\n")

	if !strings.Contains(cleaned, "LoggingOptions") {
		report.WriteString("[WARN] No logging configuration found in this package.\n")
		// Do not early-return; continue to produce a JSON result so workflow outputs
		// are consistently JSON formatted for every package.
	}

	report.WriteString("[OK] Logging configuration detected.\n\n")

	rawData := string(data)

	providersSection := extractSection(rawData, "<LogProviders>", "</LogProviders>")
	if providersSection != "" {
		report.WriteString("Log Providers:\n")
		if strings.Contains(providersSection, `CreationName="Microsoft.LogProviderSQLServer"`) {
			report.WriteString("  - SQL Server Log Provider\n")
			connMatch := regexp.MustCompile(`ConfigString="([^"]*)"`).FindStringSubmatch(providersSection)
			if len(connMatch) > 1 {
				report.WriteString(fmt.Sprintf("    Connection: %s\n", connMatch[1]))
			}
		}
		if strings.Contains(providersSection, `CreationName="Microsoft.LogProviderTextFile"`) {
			report.WriteString("  - Text File Log Provider\n")
			connMatch := regexp.MustCompile(`ConfigString="([^"]*)"`).FindStringSubmatch(providersSection)
			if len(connMatch) > 1 {
				report.WriteString(fmt.Sprintf("    File Path: %s\n", connMatch[1]))
			}
		}
		if strings.Contains(providersSection, `CreationName="Microsoft.LogProviderEventLog"`) {
			report.WriteString("  - Windows Event Log Provider\n")
		}
		report.WriteString("\n")
	}

	loggingSection := extractSection(rawData, "<LoggingOptions", "</LoggingOptions>")
	if loggingSection != "" {
		report.WriteString("Package-Level Logging Settings:\n")
		if strings.Contains(loggingSection, `LoggingMode="1"`) {
			report.WriteString("  Mode: Enabled\n")
		} else {
			report.WriteString("  Mode: Disabled\n")
		}

		if matches := regexp.MustCompile(`EventFilter">([^<]+)</`).FindStringSubmatch(loggingSection); len(matches) > 1 {
			report.WriteString(fmt.Sprintf("  Events Logged: %s\n", matches[1]))
		}

		selectedProviders := regexp.MustCompile(`SelectedLogProvider[^}]*InstanceID="([^"]*)"`).FindAllStringSubmatch(loggingSection, -1)
		if len(selectedProviders) > 0 {
			report.WriteString("  Selected Providers:\n")
			for _, match := range selectedProviders {
				if len(match) > 1 {
					report.WriteString(fmt.Sprintf("    - %s\n", match[1]))
				}
			}
		}
		report.WriteString("\n")
	}

	overrides := strings.Count(rawData, "<LoggingOptions")
	if overrides > 1 {
		report.WriteString(fmt.Sprintf("Task-Level Overrides: %d task(s) define custom logging.\n\n", overrides-1))
	}

	report.WriteString("Recommendations:\n")
	if strings.Contains(loggingSection, `LoggingMode="1"`) {
		report.WriteString("- Ensure captured events align with operational requirements.\n")
	} else {
		report.WriteString("- Enable package logging to aid troubleshooting.\n")
	}

	if strings.Contains(providersSection, `CreationName="Microsoft.LogProviderTextFile"`) {
		report.WriteString("- Validate file storage location and retention.\n")
	}
	if strings.Contains(providersSection, `CreationName="Microsoft.LogProviderSQLServer"`) {
		report.WriteString("- Confirm SQL log tables are monitored and maintained.\n")
	}

	args := request.Params.Arguments

	// Standardize output structure to match analyze_data_flow
	jsonResult := map[string]interface{}{
		"tool_name": "analyze_logging_configuration",
		"file_path": filePath,
		"package":   filepath.Base(filePath),
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "success",
		"analysis":  report.String(),
	}

	jsonBytes, err := json.Marshal(jsonResult)
	if err != nil {
		return mcp.NewToolResultText("JSON marshal error: " + err.Error()), nil
	}
	if argsMap, ok := args.(map[string]interface{}); ok {
		if outputFilePath, ok := argsMap["output_file_path"].(string); ok && outputFilePath != "" {
			resolvedPath := resolveFilePath(outputFilePath, packageDirectory)
			os.MkdirAll(filepath.Dir(resolvedPath), 0755)
			os.WriteFile(resolvedPath, jsonBytes, 0644)
		}
	}
	// Return a structured result so callers (workflow runner) can detect and
	// format JSON consistently when format=="json".
	return mcp.NewToolResultStructured(jsonResult, "Logging analysis"), nil
}

func extractPropertyValue(xmlContent, propertyName string) string {
	startTag := fmt.Sprintf("<Property Name=\"%s\">", propertyName)
	start := strings.Index(xmlContent, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(xmlContent[start:], "</Property>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(xmlContent[start : start+end])
}

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
