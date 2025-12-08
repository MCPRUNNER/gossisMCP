package extraction

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MCPRUNNER/gossisMCP/pkg/formatter"
	"github.com/MCPRUNNER/gossisMCP/pkg/types"
	"github.com/MCPRUNNER/gossisMCP/pkg/util/analysis"
	"github.com/MCPRUNNER/gossisMCP/pkg/util/file"
	"github.com/antchfx/xmlquery"
	"github.com/mark3labs/mcp-go/mcp"
)

// ResolveFilePath resolves a file path against the package directory if it's relative
func ResolveFilePath(filePath, packageDirectory string) string {
	if packageDirectory == "" || filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(packageDirectory, filePath)
}

// HandleParseDtsx handles parsing DTSX files
func HandleParseDtsx(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("parse_dtsx", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
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

// HandleExtractTasks handles task extraction from DTSX files
func HandleExtractTasks(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
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
			}
		}
	}

	return mcp.NewToolResultText(tasks), nil
}

// HandleExtractConnections handles connection extraction from DTSX files
func HandleExtractConnections(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
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

// HandleExtractVariables handles variable extraction from DTSX files
func HandleExtractVariables(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	variables := "Variables:\n"
	for i, v := range pkg.Variables.Vars {
		variables += fmt.Sprintf("%d. %s = %s\n", i+1, v.Name, v.Value)
		if v.Expression != "" {
			variables += fmt.Sprintf("   Expression: %s\n", v.Expression)
		}
	}

	return mcp.NewToolResultText(variables), nil
}

// HandleExtractPrecedenceConstraints handles precedence constraint extraction from DTSX files
func HandleExtractPrecedenceConstraints(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
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

// HandleExtractParameters handles parameter extraction from DTSX files
func HandleExtractParameters(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
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

// HandleExtractScriptCode handles script code extraction from DTSX files
func HandleExtractScriptCode(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
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

// resolveVariableExpressions resolves SSIS variable expressions by substituting variable references
func resolveVariableExpressions(value string, variables []types.Variable, maxDepth int) string {
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

// findVariableValue finds the value of a variable by name
func findVariableValue(name string, variables []types.Variable) string {
	for _, v := range variables {
		if v.Name == name {
			return v.Value
		}
	}
	return ""
}

// HandleXPathQuery handles XPath queries on XML data
func HandleXPathQuery(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	xpathExpr, err := request.RequireString("xpath")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	var xmlData string

	// Check for file_path parameter
	if filePath := request.GetString("file_path", ""); filePath != "" {
		resolvedPath := ResolveFilePath(filePath, packageDirectory)
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			result := formatter.CreateAnalysisResult("xpath_query", filePath, nil, err)
			return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
		}
		xmlData = string(data)
	} else if xmlString := request.GetString("xml", ""); xmlString != "" {
		xmlData = xmlString
	} else if jsonXml := request.GetString("json_xml", ""); jsonXml != "" {
		// Handle JSONified XML - this would be the content from read_text_file
		// For now, assume it's raw XML content
		xmlData = jsonXml
	} else {
		return mcp.NewToolResultError("Either file_path, xml, or json_xml parameter must be provided"), nil
	}

	// Parse the XML
	doc, err := xmlquery.Parse(strings.NewReader(xmlData))
	if err != nil {
		result := formatter.CreateAnalysisResult("xpath_query", xpathExpr, nil, fmt.Errorf("failed to parse XML: %v", err))
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Execute XPath query
	nodes, err := xmlquery.QueryAll(doc, xpathExpr)
	if err != nil {
		result := formatter.CreateAnalysisResult("xpath_query", xpathExpr, nil, fmt.Errorf("failed to execute XPath: %v", err))
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Collect results
	var results []map[string]interface{}
	for _, node := range nodes {
		result := map[string]interface{}{
			"type": node.Type,
			"data": node.Data,
		}

		// Add attributes if any
		if len(node.Attr) > 0 {
			attrs := make(map[string]string)
			for _, attr := range node.Attr {
				attrs[attr.Name.Local] = attr.Value
			}
			result["attributes"] = attrs
		}

		// Add inner text
		if node.FirstChild != nil {
			result["inner_text"] = node.InnerText()
		}

		results = append(results, result)
	}

	// Create analysis result
	analysisData := map[string]interface{}{
		"xpath":      xpathExpr,
		"matches":    len(results),
		"results":    results,
		"xml_source": "provided",
	}

	result := formatter.CreateAnalysisResult("xpath_query", xpathExpr, analysisData, nil)

	// Handle output file if specified
	if outputPath := request.GetString("output_file_path", ""); outputPath != "" {
		resolvedOutputPath := ResolveFilePath(outputPath, packageDirectory)
		outputDir := filepath.Dir(resolvedOutputPath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create output directory: %v", err)), nil
		}
		if err := os.WriteFile(resolvedOutputPath, []byte(formatter.FormatAnalysisResult(result, format)), 0644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write output file: %v", err)), nil
		}
	}

	return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
}

// HandleReadTextFile handles reading and analyzing text files
func HandleReadTextFile(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	isLineNumberNeeded := request.GetBool("line_numbers", true)

	// Resolve the file path against the package directory
	resolvedPath := file.ResolveFilePath(filePath, packageDirectory)
	isBinary, err := file.IsFileBinary(resolvedPath)
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
	result.WriteString("📄 Options\n\n")
	result.WriteString(fmt.Sprintf("Line Numbers: %t\n", isLineNumberNeeded))

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
		analysis.AnalyzeBatchFile(content, isLineNumberNeeded, &result)
	case ".config", ".cfg":
		result.WriteString("⚙️ Configuration File Analysis:\n")
		analysis.AnalyzeConfigFile(content, isLineNumberNeeded, &result)
	case ".sql":
		result.WriteString("🗄️ SQL File Analysis:\n")
		analysis.AnalyzeSQLFile(content, isLineNumberNeeded, &result)
	default:
		result.WriteString("🗄️ Text File Analysis:\n")
		analysis.AnalyzeGenericTextFile(content, isLineNumberNeeded, &result)
	}
	result.WriteString("📘 File Content:\n")
	for i, line := range lines {
		if isLineNumberNeeded {
			result.WriteString(fmt.Sprintf("%d  %v\n", i, line))
		} else {
			result.WriteString(fmt.Sprintf("%v\n", line))
		}

	}
	return mcp.NewToolResultText(result.String()), nil
}
