package templatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	projecttemplates "github.com/MCPRUNNER/gossisMCP/pkg/templates"
)

// HandleRenderTemplate renders an HTML template using JSON data and writes the output file.
func HandleRenderTemplate(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})

	templatePath := getFirstString(args, "template_file_path", "templateFilePath")
	if templatePath == "" {
		return mcp.NewToolResultError("template_file_path is required"), nil
	}

	outputPath := getFirstString(args, "output_file_path", "outputFilePath")
	if outputPath == "" {
		return mcp.NewToolResultError("output_file_path is required"), nil
	}

	templatePath = resolveAgainstPackage(templatePath, packageDirectory)
	outputPath = resolveAgainstPackage(outputPath, packageDirectory)

	jsonData, err := extractJSONPayload(args, packageDirectory)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Enhance payload: if it contains a top-level "data" array of objects
	// and those objects have a "file" field, add a computed "package" field
	// (basename without extension) so templates can render concise package names.
	var payload map[string]interface{}
	if err := json.Unmarshal(jsonData, &payload); err == nil {
		if raw, ok := payload["data"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for i := range arr {
					if m, ok := arr[i].(map[string]interface{}); ok {
						if fRaw, ok := m["file"]; ok {
							if fstr, ok := fRaw.(string); ok && fstr != "" {
								base := filepath.Base(fstr)
								name := strings.TrimSuffix(base, filepath.Ext(base))
								m["package"] = name
							}
						}
					}
				}
				// re-marshal payload to jsonData for rendering
				if b, merr := json.MarshalIndent(payload, "", "  "); merr == nil {
					jsonData = b
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create output directory: %v", err)), nil
	}

	if err := projecttemplates.RenderTemplateFromJSON(jsonData, templatePath, outputPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to render template: %v", err)), nil
	}

	message := fmt.Sprintf("Rendered template %s -> %s", templatePath, outputPath)
	return mcp.NewToolResultText(message), nil
}

func extractJSONPayload(args map[string]interface{}, packageDirectory string) ([]byte, error) {
	if raw := getFirstString(args, "json_data", "jsonData"); raw != "" {
		return []byte(raw), nil
	}

	jsonFile := getFirstString(args, "json_file_path", "jsonFilePath")
	if jsonFile == "" {
		return nil, fmt.Errorf("either json_data or json_file_path must be provided")
	}

	resolved := resolveAgainstPackage(jsonFile, packageDirectory)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file %s: %w", resolved, err)
	}
	return data, nil
}

func getFirstString(args map[string]interface{}, keys ...string) string {
	if args == nil {
		return ""
	}
	for _, key := range keys {
		if value, exists := args[key]; exists {
			if text, ok := value.(string); ok {
				trimmed := strings.TrimSpace(text)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func resolveAgainstPackage(pathValue, packageDirectory string) string {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	if packageDirectory != "" {
		return filepath.Clean(filepath.Join(packageDirectory, trimmed))
	}
	if abs, err := filepath.Abs(trimmed); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(trimmed)
}
