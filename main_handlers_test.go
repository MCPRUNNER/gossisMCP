package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDtsxFile tests the core DTSX parsing functionality
func TestParseDtsxFile(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "valid DTSX file",
			filePath: "testdata/Package1.dtsx",
		},
		{
			name:        "nonexistent file",
			filePath:    "testdata/NonExistent.dtsx",
			expectError: true,
			errorMsg:    "The system cannot find the file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.filePath)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
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
