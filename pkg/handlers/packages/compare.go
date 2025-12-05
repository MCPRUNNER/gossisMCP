package packages

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/MCPRUNNER/gossisMCP/pkg/formatter"
	"github.com/MCPRUNNER/gossisMCP/pkg/types"
)

func HandleComparePackages(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath1, err := request.RequireString("file_path1")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	filePath2, err := request.RequireString("file_path2")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath1 := resolveFilePath(filePath1, packageDirectory)
	resolvedPath2 := resolveFilePath(filePath2, packageDirectory)

	data1, err := os.ReadFile(resolvedPath1)
	if err != nil {
		result := formatter.CreateAnalysisResult("compare_packages", filePath1, nil, fmt.Errorf("failed to read first file: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "compare_packages",
				"file_path": filePath1,
				"package":   filepath.Base(filePath1),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("failed to read first file: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "Package comparison error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}
	data1 = []byte(strings.ReplaceAll(string(data1), "DTS:", ""))
	var pkg1 types.SSISPackage
	if err := xml.Unmarshal(data1, &pkg1); err != nil {
		result := formatter.CreateAnalysisResult("compare_packages", filePath1, nil, fmt.Errorf("failed to parse first file: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "compare_packages",
				"file_path": filePath1,
				"package":   filepath.Base(filePath1),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("failed to parse first file: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "Package comparison error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data2, err := os.ReadFile(resolvedPath2)
	if err != nil {
		result := formatter.CreateAnalysisResult("compare_packages", filePath2, nil, fmt.Errorf("failed to read second file: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "compare_packages",
				"file_path": filePath2,
				"package":   filepath.Base(filePath2),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("failed to read second file: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "Package comparison error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}
	data2 = []byte(strings.ReplaceAll(string(data2), "DTS:", ""))
	var pkg2 types.SSISPackage
	if err := xml.Unmarshal(data2, &pkg2); err != nil {
		result := formatter.CreateAnalysisResult("compare_packages", filePath2, nil, fmt.Errorf("failed to parse second file: %v", err))
		if format == formatter.FormatJSON {
			jsonResult := map[string]interface{}{
				"tool_name": "compare_packages",
				"file_path": filePath2,
				"package":   filepath.Base(filePath2),
				"timestamp": time.Now().Format(time.RFC3339),
				"status":    "error",
				"error":     fmt.Sprintf("failed to parse second file: %v", err),
			}
			return mcp.NewToolResultStructured(jsonResult, "Package comparison error"), nil
		}
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("ðŸ“Š Package Comparison Report\n\n")
	result.WriteString(fmt.Sprintf("File 1: %s\n", filepath.Base(resolvedPath1)))
	result.WriteString(fmt.Sprintf("File 2: %s\n\n", filepath.Base(resolvedPath2)))

	result.WriteString("ðŸ“‹ Package Properties:\n")
	compareProperties(pkg1.Properties, pkg2.Properties, &result)

	result.WriteString("\nðŸ”— Connection Managers:\n")
	compareConnections(pkg1.ConnectionMgr.Connections, pkg2.ConnectionMgr.Connections, &result)

	result.WriteString("\nðŸ“Š Variables:\n")
	compareVariables(pkg1.Variables.Vars, pkg2.Variables.Vars, &result)

	result.WriteString("\nâš™ï¸ Parameters:\n")
	compareParameters(pkg1.Parameters.Params, pkg2.Parameters.Params, &result)

	result.WriteString("\nðŸ”§ Configurations:\n")
	compareConfigurations(pkg1.Configurations.Configs, pkg2.Configurations.Configs, &result)

	result.WriteString("\nðŸŽ¯ Tasks:\n")
	compareTasks(pkg1.Executables.Tasks, pkg2.Executables.Tasks, &result)

	result.WriteString("\nðŸš¨ Event Handlers:\n")
	compareEventHandlers(pkg1.EventHandlers.EventHandlers, pkg2.EventHandlers.EventHandlers, &result)

	result.WriteString("\n📀 Precedence Constraints:\n")
	comparePrecedenceConstraints(pkg1.PrecedenceConstraints.Constraints, pkg2.PrecedenceConstraints.Constraints, &result)

	analysisResult := formatter.CreateAnalysisResult("compare_packages", fmt.Sprintf("%s vs %s", filePath1, filePath2), result.String(), nil)

	// For JSON format, return structured data
	if format == formatter.FormatJSON {
		jsonResult := map[string]interface{}{
			"tool_name":  analysisResult.ToolName,
			"file_path1": filePath1,
			"file_path2": filePath2,
			"package1":   filepath.Base(filePath1),
			"package2":   filepath.Base(filePath2),
			"timestamp":  analysisResult.Timestamp,
			"status":     analysisResult.Status,
			"analysis":   analysisResult.Data,
		}
		if analysisResult.Error != "" {
			jsonResult["error"] = analysisResult.Error
		}
		return mcp.NewToolResultStructured(jsonResult, "Package comparison"), nil
	}

	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

func compareProperties(props1, props2 []types.Property, result *strings.Builder) {
	propMap1 := make(map[string]string)
	propMap2 := make(map[string]string)

	for _, p := range props1 {
		propMap1[p.Name] = p.Value
	}
	for _, p := range props2 {
		propMap2[p.Name] = p.Value
	}

	for name, value := range propMap2 {
		if _, exists := propMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s = %s\n", name, value))
		}
	}

	for name, value := range propMap1 {
		if _, exists := propMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s = %s\n", name, value))
		}
	}

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

func compareConnections(conns1, conns2 []types.Connection, result *strings.Builder) {
	connMap1 := make(map[string]types.Connection)
	connMap2 := make(map[string]types.Connection)

	for _, c := range conns1 {
		connMap1[c.Name] = c
	}
	for _, c := range conns2 {
		connMap2[c.Name] = c
	}

	for name := range connMap2 {
		if _, exists := connMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	for name := range connMap1 {
		if _, exists := connMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

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

func compareVariables(vars1, vars2 []types.Variable, result *strings.Builder) {
	varMap1 := make(map[string]types.Variable)
	varMap2 := make(map[string]types.Variable)

	for _, v := range vars1 {
		varMap1[v.Name] = v
	}
	for _, v := range vars2 {
		varMap2[v.Name] = v
	}

	for name := range varMap2 {
		if _, exists := varMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	for name := range varMap1 {
		if _, exists := varMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

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

func compareParameters(params1, params2 []types.Parameter, result *strings.Builder) {
	paramMap1 := make(map[string]types.Parameter)
	paramMap2 := make(map[string]types.Parameter)

	for _, p := range params1 {
		paramMap1[p.Name] = p
	}
	for _, p := range params2 {
		paramMap2[p.Name] = p
	}

	for name := range paramMap2 {
		if _, exists := paramMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	for name := range paramMap1 {
		if _, exists := paramMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

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

func compareConfigurations(configs1, configs2 []types.Configuration, result *strings.Builder) {
	if len(configs1) != len(configs2) {
		result.WriteString(fmt.Sprintf("  ðŸ“Š Count changed: %d â†’ %d\n", len(configs1), len(configs2)))
	} else if len(configs1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func compareTasks(tasks1, tasks2 []types.Task, result *strings.Builder) {
	taskMap1 := make(map[string]types.Task)
	taskMap2 := make(map[string]types.Task)

	for _, t := range tasks1 {
		taskMap1[t.Name] = t
	}
	for _, t := range tasks2 {
		taskMap2[t.Name] = t
	}

	for name := range taskMap2 {
		if _, exists := taskMap1[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž• Added: %s\n", name))
		}
	}

	for name := range taskMap1 {
		if _, exists := taskMap2[name]; !exists {
			result.WriteString(fmt.Sprintf("  âž– Removed: %s\n", name))
		}
	}

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

func compareEventHandlers(handlers1, handlers2 []types.EventHandler, result *strings.Builder) {
	if len(handlers1) != len(handlers2) {
		result.WriteString(fmt.Sprintf("  ðŸ“Š Count changed: %d â†’ %d\n", len(handlers1), len(handlers2)))
	} else if len(handlers1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func comparePrecedenceConstraints(constraints1, constraints2 []types.PrecedenceConstraint, result *strings.Builder) {
	if len(constraints1) != len(constraints2) {
		result.WriteString(fmt.Sprintf("  ðŸ“Š Count changed: %d â†’ %d\n", len(constraints1), len(constraints2)))
	} else if len(constraints1) > 0 {
		result.WriteString("  âœ… No differences found\n")
	}
}

func resolveFilePath(filePath, packageDirectory string) string {
	if packageDirectory == "" || filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(packageDirectory, filePath)
}
