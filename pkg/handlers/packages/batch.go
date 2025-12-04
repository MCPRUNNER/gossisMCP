package packages

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/MCPRUNNER/gossisMCP/pkg/types"
)

type batchAnalysisResult struct {
	PackagePath    string                 `json:"package_path"`
	Success        bool                   `json:"success"`
	Error          string                 `json:"error,omitempty"`
	AnalysisResult map[string]interface{} `json:"analysis_result,omitempty"`
	Duration       time.Duration          `json:"duration"`
}

type batchSummary struct {
	TotalPackages    int                   `json:"total_packages"`
	Successful       int                   `json:"successful"`
	Failed           int                   `json:"failed"`
	TotalDuration    time.Duration         `json:"total_duration"`
	AverageDuration  time.Duration         `json:"average_duration"`
	Errors           []string              `json:"errors,omitempty"`
	PackageSummaries []batchAnalysisResult `json:"package_summaries"`
}

func HandleBatchAnalyze(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	rawPaths, ok := args["file_paths"].([]interface{})
	if !ok {
		return mcp.NewToolResultError("file_paths parameter is required and must be an array"), nil
	}

	var paths []string
	for _, raw := range rawPaths {
		if pathStr, ok := raw.(string); ok && pathStr != "" {
			paths = append(paths, pathStr)
		}
	}

	if len(paths) == 0 {
		return mcp.NewToolResultError("no valid file paths provided"), nil
	}

	format := "text"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	maxConcurrency := 4
	if mc, ok := args["max_concurrent"].(float64); ok && mc > 0 {
		maxConcurrency = int(mc)
	}

	sem := make(chan struct{}, maxConcurrency)
	results := make(chan batchAnalysisResult, len(paths))
	startTime := time.Now()

	for _, path := range paths {
		go func(filePath string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			result := batchAnalysisResult{PackagePath: filePath}
			resultStart := time.Now()

			analysisResult, err := performBatchPackageAnalysis(filePath, packageDirectory)
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

	var batchResults []batchAnalysisResult
	for range paths {
		select {
		case result := <-results:
			batchResults = append(batchResults, result)
		case <-ctx.Done():
			return mcp.NewToolResultError("batch analysis cancelled"), nil
		}
	}

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

	summary := batchSummary{
		TotalPackages:    len(paths),
		Successful:       successful,
		Failed:           failed,
		TotalDuration:    totalDuration,
		Errors:           errors,
		PackageSummaries: batchResults,
	}

	if summary.TotalPackages > 0 {
		summary.AverageDuration = totalDuration / time.Duration(summary.TotalPackages)
	}

	switch format {
	case "json":
		jsonData, _ := json.MarshalIndent(summary, "", "  ")
		return mcp.NewToolResultText(string(jsonData)), nil
	case "csv":
		return mcp.NewToolResultText(formatBatchSummaryAsCSV(summary)), nil
	case "html":
		return mcp.NewToolResultText(formatBatchSummaryAsHTML(summary)), nil
	case "markdown":
		return mcp.NewToolResultText(formatBatchSummaryAsMarkdown(summary)), nil
	default:
		return mcp.NewToolResultText(formatBatchSummaryAsText(summary)), nil
	}
}

func performBatchPackageAnalysis(filePath, packageDirectory string) (map[string]interface{}, error) {
	fullPath := resolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse DTSX file: %w", err)
	}

	packageName := "Unknown"
	for _, prop := range pkg.Properties {
		if prop.Name == "Name" {
			packageName = strings.TrimSpace(prop.Value)
			break
		}
	}

	result := map[string]interface{}{
		"package_name":     packageName,
		"task_count":       len(pkg.Executables.Tasks),
		"connection_count": len(pkg.ConnectionMgr.Connections),
		"variable_count":   len(pkg.Variables.Vars),
		"parameter_count":  len(pkg.Parameters.Params),
	}

	taskTypes := make(map[string]int)
	for _, task := range pkg.Executables.Tasks {
		taskType := describeTaskType(task)
		taskTypes[taskType]++
	}
	result["task_types"] = taskTypes

	return result, nil
}

func describeTaskType(task types.Task) string {
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
	if task.CreationName != "" {
		return task.CreationName
	}
	return "Unknown Task Type"
}

func formatBatchSummaryAsText(summary batchSummary) string {
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
		status := "[OK]"
		if !pkg.Success {
			status = "[FAIL]"
		}
		output.WriteString(fmt.Sprintf("%s %s (%v)\n", status, pkg.PackagePath, pkg.Duration))
		if !pkg.Success {
			output.WriteString(fmt.Sprintf("  Error: %s\n", pkg.Error))
		}
	}

	return output.String()
}

func formatBatchSummaryAsCSV(summary batchSummary) string {
	var output strings.Builder

	output.WriteString("Summary\n")
	output.WriteString("Total Packages,Successful,Failed,Total Duration,Average Duration\n")
	output.WriteString(fmt.Sprintf("%d,%d,%d,%v,%v\n\n",
		summary.TotalPackages, summary.Successful, summary.Failed,
		summary.TotalDuration, summary.AverageDuration))

	if len(summary.Errors) > 0 {
		output.WriteString("Errors\n")
		for _, err := range summary.Errors {
			output.WriteString(fmt.Sprintf("\"%s\"\n", strings.ReplaceAll(err, "\"", "\"\"")))
		}
		output.WriteString("\n")
	}

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

func formatBatchSummaryAsHTML(summary batchSummary) string {
	var output strings.Builder

	output.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>Batch Analysis Summary</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f0f0f0; padding: 10px; border-radius: 5px; }
        .ok { color: #2d7a2d; }
        .fail { color: #b22222; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>Batch Analysis Summary</h1>

    <div class="summary">
        <h2>Overview</h2>
        <p>Total Packages: `)
	output.WriteString(fmt.Sprintf("%d</p>", summary.TotalPackages))
	output.WriteString(fmt.Sprintf("<p>Successful: <span class=\"ok\">%d</span></p>", summary.Successful))
	output.WriteString(fmt.Sprintf("<p>Failed: <span class=\"fail\">%d</span></p>", summary.Failed))
	output.WriteString(fmt.Sprintf("<p>Total Duration: %v</p>", summary.TotalDuration))
	output.WriteString(fmt.Sprintf("<p>Average Duration: %v</p>", summary.AverageDuration))
	output.WriteString("    </div>")

	if len(summary.Errors) > 0 {
		output.WriteString(`
    <h2>Errors</h2>
    <ul>`)
		for _, err := range summary.Errors {
			output.WriteString(fmt.Sprintf("<li>%s</li>", html.EscapeString(err)))
		}
		output.WriteString("    </ul>")
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
		status := "<span class=\"ok\">OK</span>"
		if !pkg.Success {
			status = "<span class=\"fail\">FAIL</span>"
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

func formatBatchSummaryAsMarkdown(summary batchSummary) string {
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
		status := "OK"
		if !pkg.Success {
			status = "FAIL"
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
