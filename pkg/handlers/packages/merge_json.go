package packages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// MergeJSONFilesHandler merges multiple JSON files into a single JSON object
func MergeJSONFilesHandler(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	// Get file paths array
	filePathsArg, ok := args["file_paths"]
	if !ok {
		return mcp.NewToolResultError("file_paths is required"), nil
	}

	// Convert to string slice
	var filePaths []string
	switch v := filePathsArg.(type) {
	case []interface{}:
		for _, path := range v {
			if pathStr, ok := path.(string); ok {
				filePaths = append(filePaths, pathStr)
			}
		}
	case string:
		// Single file path
		filePaths = []string{v}
	default:
		return mcp.NewToolResultError("file_paths must be an array of strings"), nil
	}

	if len(filePaths) == 0 {
		return mcp.NewToolResultError("at least one file path is required"), nil
	}

	// Get output file path (optional)
	outputFilePath := ""
	if outputArg, ok := args["output_file_path"].(string); ok {
		outputFilePath = outputArg
	}

	// Create root object
	root := make(map[string]interface{})
	mergedData := make(map[string]interface{})

	// Process each file
	for _, filePath := range filePaths {
		// Resolve file path
		resolvedPath := resolveFilePath(filePath, packageDirectory)

		// Read JSON file
		fileData, err := os.ReadFile(resolvedPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read file %s: %v", filePath, err)), nil
		}

		// Parse JSON
		var jsonData interface{}
		if err := json.Unmarshal(fileData, &jsonData); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse JSON from %s: %v", filePath, err)), nil
		}

		// Get base name without extension
		baseName := filepath.Base(filePath)
		baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))

		// Add to merged data with base name as key
		mergedData[baseName] = jsonData
	}

	// Create root object
	root["root"] = mergedData

	// Format output
	var outputContent string

	jsonBytes, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal JSON: %v", err)), nil
	}
	outputContent = string(jsonBytes)

	// Write to output file if specified
	if outputFilePath != "" {
		resolvedOutputPath := resolveFilePath(outputFilePath, packageDirectory)

		// Create directory if it doesn't exist
		outputDir := filepath.Dir(resolvedOutputPath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create output directory: %v", err)), nil
		}

		if err := os.WriteFile(resolvedOutputPath, []byte(outputContent), 0644); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write output file: %v", err)), nil
		}
	}

	// Return result
	result := map[string]interface{}{
		"tool_name":    "merge_json_files",
		"files_merged": len(filePaths),
		"file_paths":   filePaths,
		"status":       "success",
	}

	if outputFilePath != "" {
		result["output_file"] = outputFilePath
	}

	return mcp.NewToolResultText(outputFilePath), nil
}
