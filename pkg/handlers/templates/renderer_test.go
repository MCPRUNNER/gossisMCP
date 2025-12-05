package templatehandlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleRenderTemplateWithInlineJSON(t *testing.T) {
	baseDir := t.TempDir()

	templatePath := filepath.Join(baseDir, "report.tmpl")
	if err := os.WriteFile(templatePath, []byte("Hello {{.name}}!"), 0o644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"template_file_path": "report.tmpl",
				"output_file_path":   "output/report.txt",
				"json_data":          `{"name":"World"}`,
			},
		},
	}

	result, err := HandleRenderTemplate(context.Background(), request, baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected tool result")
	}
	if result.IsError {
		t.Fatalf("expected success, got error result: %+v", result)
	}

	output := filepath.Join(baseDir, "output", "report.txt")
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read rendered output: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "Hello World!" {
		t.Fatalf("unexpected rendered output: %q", got)
	}
}

func TestHandleRenderTemplateWithJSONFile(t *testing.T) {
	baseDir := t.TempDir()

	templatePath := filepath.Join(baseDir, "report.tmpl")
	if err := os.WriteFile(templatePath, []byte("Name: {{.name}}"), 0o644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	jsonPath := filepath.Join(baseDir, "data.json")
	if err := os.WriteFile(jsonPath, []byte(`{"name":"Data"}`), 0o644); err != nil {
		t.Fatalf("failed to write json: %v", err)
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"template_file_path": templatePath,
				"output_file_path":   filepath.Join(baseDir, "output.txt"),
				"json_file_path":     jsonPath,
			},
		},
	}

	result, err := HandleRenderTemplate(context.Background(), request, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful tool result, got %+v", result)
	}

	data, err := os.ReadFile(filepath.Join(baseDir, "output.txt"))
	if err != nil {
		t.Fatalf("failed to read rendered output: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "Name: Data" {
		t.Fatalf("unexpected rendered output: %q", got)
	}
}
