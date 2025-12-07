package packages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMergeJSONFilesHandler(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create test JSON files
	file1Data := map[string]interface{}{
		"name":  "ConfigFile",
		"count": 1,
	}
	file1Path := filepath.Join(tempDir, "dataflow_analysis.json")
	file1JSON, _ := json.MarshalIndent(file1Data, "", "  ")
	if err := os.WriteFile(file1Path, file1JSON, 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	file2Data := map[string]interface{}{
		"status":  "success",
		"entries": 5,
	}
	file2Path := filepath.Join(tempDir, "logging_analysis.json")
	file2JSON, _ := json.MarshalIndent(file2Data, "", "  ")
	if err := os.WriteFile(file2Path, file2JSON, 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	file3Data := map[string]interface{}{
		"warnings": 3,
		"errors":   0,
	}
	file3Path := filepath.Join(tempDir, "best_practices.json")
	file3JSON, _ := json.MarshalIndent(file3Data, "", "  ")
	if err := os.WriteFile(file3Path, file3JSON, 0644); err != nil {
		t.Fatalf("Failed to create test file 3: %v", err)
	}

	// Test merging files
	outputPath := filepath.Join(tempDir, "merged_output.json")

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"file_paths": []interface{}{
					file1Path,
					file2Path,
					file3Path,
				},
				"output_file_path": outputPath,
				"format":           "json",
			},
		},
	}

	ctx := context.Background()
	result, err := MergeJSONFilesHandler(ctx, request, "")

	if err != nil {
		t.Fatalf("MergeJSONFilesHandler returned error: %v", err)
	}

	if result == nil {
		t.Fatal("MergeJSONFilesHandler returned nil result")
	}

	// Verify output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Output file was not created: %s", outputPath)
	}

	// Read and verify the merged JSON
	mergedData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read merged output: %v", err)
	}

	var merged map[string]interface{}
	if err := json.Unmarshal(mergedData, &merged); err != nil {
		t.Fatalf("Failed to parse merged JSON: %v", err)
	}

	// Verify structure
	root, ok := merged["root"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing or invalid 'root' key in merged JSON")
	}

	// Verify each file's data is present under its base name
	if _, ok := root["dataflow_analysis"]; !ok {
		t.Error("Missing 'dataflow_analysis' in root")
	}
	if _, ok := root["logging_analysis"]; !ok {
		t.Error("Missing 'logging_analysis' in root")
	}
	if _, ok := root["best_practices"]; !ok {
		t.Error("Missing 'best_practices' in root")
	}

	// Verify data integrity
	df := root["dataflow_analysis"].(map[string]interface{})
	if df["name"] != "ConfigFile" {
		t.Errorf("Expected name 'ConfigFile', got %v", df["name"])
	}

	la := root["logging_analysis"].(map[string]interface{})
	if la["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", la["status"])
	}

	bp := root["best_practices"].(map[string]interface{})
	if bp["warnings"] != float64(3) {
		t.Errorf("Expected warnings 3, got %v", bp["warnings"])
	}
}

func TestMergeJSONFilesHandlerWithRelativePaths(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create test JSON files
	file1Data := map[string]interface{}{"id": 1}
	file1Path := filepath.Join(tempDir, "file1.json")
	file1JSON, _ := json.Marshal(file1Data)
	if err := os.WriteFile(file1Path, file1JSON, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file2Data := map[string]interface{}{"id": 2}
	file2Path := filepath.Join(tempDir, "file2.json")
	file2JSON, _ := json.Marshal(file2Data)
	if err := os.WriteFile(file2Path, file2JSON, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"file_paths": []interface{}{
					"file1.json",
					"file2.json",
				},
				"format": "json",
			},
		},
	}

	ctx := context.Background()
	result, err := MergeJSONFilesHandler(ctx, request, tempDir)

	if err != nil {
		t.Fatalf("MergeJSONFilesHandler returned error: %v", err)
	}

	if result == nil {
		t.Fatal("MergeJSONFilesHandler returned nil result")
	}

	// Verify the result contains merged data
	if len(result.Content) == 0 {
		t.Fatal("Result content is empty")
	}
}

func TestMergeJSONFilesHandlerErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]interface{}
	}{
		{
			name: "missing file_paths",
			args: map[string]interface{}{},
		},
		{
			name: "empty file_paths",
			args: map[string]interface{}{
				"file_paths": []interface{}{},
			},
		},
		{
			name: "invalid file path",
			args: map[string]interface{}{
				"file_paths": []interface{}{
					"/nonexistent/file.json",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: tt.args,
				},
			}

			result, err := MergeJSONFilesHandler(ctx, request, "")
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if result == nil {
				t.Fatal("Expected error result, got nil")
			}

			if !result.IsError {
				t.Error("Expected IsError to be true")
			}
		})
	}
}
