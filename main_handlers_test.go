package main

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/analysis"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/extraction"
	packagehandlers "github.com/MCPRUNNER/gossisMCP/pkg/handlers/packages"
	"github.com/MCPRUNNER/gossisMCP/pkg/handlers/validation"
)

// TestParseDtsxFile tests the core DTSX parsing functionality
func TestParseDtsxFile(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{
			name:     "valid DTSX file",
			filePath: "testdata/Package1.dtsx",
		},
		{
			name:        "nonexistent file",
			filePath:    "testdata/NonExistent.dtsx",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.filePath)
			if tt.expectError {
				require.Error(t, err)
				assert.True(t, os.IsNotExist(err), "expected file not found error, got: %v", err)
				return
			}

			require.NoError(t, err)

			// Remove namespace prefixes for easier parsing
			data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
			data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

			var pkg SSISPackage
			err = xml.Unmarshal(data, &pkg)
			require.NoError(t, err)

			// Verify basic structure - for empty package, slices may be nil but should be accessible
			assert.NotNil(t, pkg.Properties)
			// Variables and Executables may be nil for empty packages, but Properties should exist
			_ = pkg.Variables.Vars    // Just access to ensure no panic
			_ = pkg.Executables.Tasks // Just access to ensure no panic
		})
	}
}

// TestExtractTasksFromPackage tests task extraction logic
func TestExtractTasksFromPackage(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test task extraction (this mirrors the logic in handleExtractTasks)
	tasks := pkg.Executables.Tasks

	// For empty packages, tasks should be empty (nil slice has len 0)
	assert.Empty(t, tasks)
}

// TestExtractConnectionsFromPackage tests connection extraction logic
func TestExtractConnectionsFromPackage(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test connection extraction
	connections := pkg.ConnectionMgr.Connections

	// For empty packages, connections should be empty
	assert.Empty(t, connections)
}

// TestExtractVariablesFromPackage tests variable extraction logic
func TestExtractVariablesFromPackage(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test variable extraction
	variables := pkg.Variables.Vars

	// For empty packages, variables should be empty
	assert.Empty(t, variables)
}

// TestExtractParametersFromPackage tests parameter extraction logic
func TestExtractParametersFromPackage(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test parameter extraction
	parameters := pkg.Parameters.Params

	// For empty packages, parameters should be empty
	assert.Empty(t, parameters)
}

// TestExtractPrecedenceFromPackage tests precedence constraint extraction logic
func TestExtractPrecedenceFromPackage(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test precedence constraint extraction
	constraints := pkg.PrecedenceConstraints.Constraints

	// For empty packages, constraints should be empty
	assert.Empty(t, constraints)
}

// TestExtractScriptFromPackage tests script code extraction logic
func TestExtractScriptFromPackage(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test script extraction - look for script tasks
	scriptFound := false
	for _, task := range pkg.Executables.Tasks {
		if task.CreationName == "Script Task" {
			scriptFound = true
			// In a real script task, this would have script code
			// Note: The struct path might be different, but for this test we just check if script tasks exist
		}
	}

	// Package1.dtsx doesn't have script tasks
	assert.False(t, scriptFound)
}

// TestValidateBestPracticesLogic tests best practices validation logic
func TestValidateBestPracticesLogic(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test basic validation - package should be valid
	assert.NotNil(t, pkg.Properties)
	// Just verify we can access the fields without panic
	_ = pkg.Variables.Vars
	_ = pkg.Executables.Tasks
}

// TestAnalyzeDataFlowLogic tests data flow analysis logic
func TestAnalyzeDataFlowLogic(t *testing.T) {
	// Read test file
	data, err := os.ReadFile("testdata/Package1.dtsx")
	require.NoError(t, err)

	// Parse the package
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg SSISPackage
	err = xml.Unmarshal(data, &pkg)
	require.NoError(t, err)

	// Test data flow analysis - look for data flow tasks
	dataFlowFound := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			dataFlowFound = true
			// In a real data flow task, this would have components
			assert.NotNil(t, task.ObjectData.DataFlow.Components.Components)
		}
	}

	// Package1.dtsx doesn't have data flow tasks
	assert.False(t, dataFlowFound)
}

// TestListPackagesLogic tests package listing logic
func TestListPackagesLogic(t *testing.T) {
	// Test listing files in testdata directory
	files, err := os.ReadDir("testdata")
	require.NoError(t, err)

	// Should find our test DTSX files
	dtsxFiles := 0
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".dtsx") {
			dtsxFiles++
		}
	}

	assert.Greater(t, dtsxFiles, 0, "Should find at least one DTSX file")
}

// TestResolveFilePath tests the file path resolution logic
func TestResolveFilePath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.dtsx")
	err := os.WriteFile(testFile, []byte("<test></test>"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		filePath       string
		packageDir     string
		expectedSuffix string
	}{
		{
			name:           "absolute path",
			filePath:       testFile,
			packageDir:     "",
			expectedSuffix: "test.dtsx",
		},
		{
			name:           "relative path with package directory",
			filePath:       "test.dtsx",
			packageDir:     tempDir,
			expectedSuffix: "test.dtsx",
		},
		{
			name:           "relative path without package directory",
			filePath:       "test.dtsx",
			packageDir:     "",
			expectedSuffix: "test.dtsx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := resolveFilePath(tt.filePath, tt.packageDir)
			assert.True(t, strings.HasSuffix(resolved, tt.expectedSuffix), "Expected path to end with %s, got %s", tt.expectedSuffix, resolved)
			assert.True(t, strings.Contains(resolved, "test.dtsx"), "Expected path to contain test.dtsx")
		})
	}
}

// createTestCallToolRequest creates a proper mcp.CallToolRequest for testing
func createTestCallToolRequest(toolName string, args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}
}

// Phase 3: Integration Tests - Testing full MCP tool handler workflows

// TestHandleParseDtsxIntegration tests the full MCP integration for parse_dtsx tool
func TestHandleParseDtsxIntegration(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		format      string
		expectError bool
	}{
		{
			name:     "parse valid DTSX file with text format",
			filePath: "testdata/Package1.dtsx",
			format:   "text",
		},
		{
			name:     "parse valid DTSX file with json format",
			filePath: "testdata/Package1.dtsx",
			format:   "json",
		},
		{
			name:        "parse nonexistent file",
			filePath:    "testdata/NonExistent.dtsx",
			format:      "text",
			expectError: false, // Handler returns error result, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"file_path": tt.filePath,
				"format":    tt.format,
			}
			request := createTestCallToolRequest("parse_dtsx", params)

			result, err := extraction.HandleParseDtsx(context.Background(), request, "")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if result != nil {
					t.Logf("Result content length: %d", len(result.Content))
					assert.NotEmpty(t, result.Content)

					// Check that result contains expected content
					if len(result.Content) > 0 {
						content := result.Content[0]
						t.Logf("Content type: %T", content)
						textContent, ok := content.(mcp.TextContent)
						assert.True(t, ok, "Expected TextContent, got %T", content)
						if ok {
							if tt.format == "json" {
								// For JSON format, should contain JSON structure
								assert.Contains(t, textContent.Text, `"tool_name": "parse_dtsx"`)
							} else if strings.Contains(tt.filePath, "NonExistent") {
								// For error cases, should contain error message
								assert.Contains(t, textContent.Text, "Error:")
							} else {
								// For successful text parsing, should contain analysis results
								assert.Contains(t, textContent.Text, "parse_dtsx Analysis Report")
							}
						}
					}
				}
			}
		})
	}
}

// TestHandleExtractTasksIntegration tests the full MCP integration for extract_tasks tool
func TestHandleExtractTasksIntegration(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "extract tasks from valid DTSX file",
			filePath: "testdata/Package1.dtsx",
		},
		{
			name:     "extract tasks from nonexistent file",
			filePath: "testdata/NonExistent.dtsx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{
				"file_path": tt.filePath,
			}
			request := createTestCallToolRequest("extract_tasks", params)

			result, err := extraction.HandleExtractTasks(context.Background(), request, "")

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Content)

			content := result.Content[0]
			textContent, ok := content.(mcp.TextContent)
			assert.True(t, ok, "Expected TextContent")
			if strings.Contains(tt.filePath, "NonExistent") {
				assert.Contains(t, textContent.Text, "Failed to read file:")
			} else {
				assert.Contains(t, textContent.Text, "Tasks:")
			}
		})
	}
}

// TestHandleExtractConnectionsIntegration tests the full MCP integration for extract_connections tool
func TestHandleExtractConnectionsIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("extract_connections", params)

	result, err := extraction.HandleExtractConnections(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Connections:")
}

// TestHandleExtractVariablesIntegration tests the full MCP integration for extract_variables tool
func TestHandleExtractVariablesIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("extract_variables", params)

	result, err := extraction.HandleExtractVariables(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Variables:")
}

// TestHandleExtractParametersIntegration tests the full MCP integration for extract_parameters tool
func TestHandleExtractParametersIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("extract_parameters", params)

	result, err := extraction.HandleExtractParameters(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Parameters:")
}

// TestHandleExtractPrecedenceConstraintsIntegration tests the full MCP integration for extract_precedence_constraints tool
func TestHandleExtractPrecedenceConstraintsIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("extract_precedence_constraints", params)

	result, err := extraction.HandleExtractPrecedenceConstraints(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Precedence Constraints:")
}

// TestHandleExtractScriptCodeIntegration tests the full MCP integration for extract_script_code tool
func TestHandleExtractScriptCodeIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("extract_script_code", params)

	result, err := extraction.HandleExtractScriptCode(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Script Tasks Code:")
}

// TestHandleValidateBestPracticesIntegration tests the full MCP integration for validate_best_practices tool
func TestHandleValidateBestPracticesIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("validate_best_practices", params)

	result, err := packagehandlers.HandleValidateBestPractices(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Best Practices Validation Report:")
}

// TestHandleListPackagesIntegration tests the full MCP integration for list_packages tool
func TestHandleListPackagesIntegration(t *testing.T) {
	params := map[string]interface{}{}
	request := createTestCallToolRequest("list_packages", params)

	result, err := packagehandlers.HandleListPackages(context.Background(), request, "testdata")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Found")
}

// TestHandleAnalyzeDataFlowIntegration tests the full MCP integration for analyze_data_flow tool
func TestHandleAnalyzeDataFlowIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("analyze_data_flow", params)

	result, err := analysis.HandleAnalyzeDataFlow(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Data Flow Analysis:")
}

// TestHandleDetectHardcodedValuesIntegration tests the full MCP integration for detect_hardcoded_values tool
func TestHandleDetectHardcodedValuesIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("detect_hardcoded_values", params)

	result, err := packagehandlers.HandleDetectHardcodedValues(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Hard-coded Values Detection Report:")
}

// TestHandleAskAboutDtsxIntegration tests the full MCP integration for ask_about_dtsx tool
func TestHandleAskAboutDtsxIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
		"question":  "What is this package?",
	}
	request := createTestCallToolRequest("ask_about_dtsx", params)

	result, err := packagehandlers.HandleAskAboutDtsx(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Package Summary:")
}

// TestHandleAnalyzeMessageQueueTasksIntegration tests the full MCP integration for analyze_message_queue_tasks tool
func TestHandleAnalyzeMessageQueueTasksIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("analyze_message_queue_tasks", params)

	result, err := packagehandlers.HandleAnalyzeMessageQueueTasks(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Message Queue Tasks Analysis:")
}

// TestHandleAnalyzeScriptTaskIntegration tests the full MCP integration for analyze_script_task tool
func TestHandleAnalyzeScriptTaskIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("analyze_script_task", params)

	result, err := packagehandlers.HandleAnalyzeScriptTask(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Script Tasks Analysis:")
}

// TestHandleAnalyzeLoggingConfigurationIntegration tests the full MCP integration for analyze_logging_configuration tool
func TestHandleAnalyzeLoggingConfigurationIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("analyze_logging_configuration", params)

	result, err := packagehandlers.HandleAnalyzeLoggingConfiguration(context.Background(), request, "")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Logging Configuration Analysis:")
}

// TestHandleValidateDtsxIntegration tests the full MCP integration for validate_dtsx tool
func TestHandleValidateDtsxIntegration(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "testdata/Package1.dtsx",
	}
	request := createTestCallToolRequest("validate_dtsx", params)

	result, err := validation.HandleValidateDtsx(context.Background(), request)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "Validation")
}

// TestMCPToolHandlerErrorHandling tests error handling in MCP tool handlers
func TestMCPToolHandlerErrorHandling(t *testing.T) {
	// Test missing required parameter
	params := map[string]interface{}{}
	request := createTestCallToolRequest("parse_dtsx", params)

	result, err := extraction.HandleParseDtsx(context.Background(), request, "")

	assert.NoError(t, err) // Handler returns error result, not error
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "required argument") // Should contain error information
}

// TestMCPToolHandlerWithPackageDirectory tests handlers with package directory parameter
func TestMCPToolHandlerWithPackageDirectory(t *testing.T) {
	params := map[string]interface{}{
		"file_path": "Package1.dtsx", // Relative path
	}
	request := createTestCallToolRequest("parse_dtsx", params)

	// Use testdata directory as package directory
	result, err := extraction.HandleParseDtsx(context.Background(), request, "testdata")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)

	content := result.Content[0]
	textContent, ok := content.(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
	assert.Contains(t, textContent.Text, "parse_dtsx")
}
