package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderTemplateFromJSON(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create a simple template file
	templateContent := `<html><body><h1>{{.Name}}</h1><p>{{.Description}}</p></body></html>`
	templatePath := filepath.Join(tempDir, "test.tmpl")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to create template file: %v", err)
	}

	// JSON data
	jsonData := `{"Name": "Test Package", "Description": "A test description"}`

	// Output path
	outputPath := filepath.Join(tempDir, "output.html")

	// Call the function
	err := RenderTemplateFromJSON([]byte(jsonData), templatePath, outputPath)
	if err != nil {
		t.Fatalf("RenderTemplateFromJSON failed: %v", err)
	}

	// Read the output file
	outputContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	expected := `<html><body><h1>Test Package</h1><p>A test description</p></body></html>`
	if string(outputContent) != expected {
		t.Fatalf("expected %q, got %q", expected, string(outputContent))
	}
}

func TestRenderTemplateFromJSON_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "test.tmpl")
	if err := os.WriteFile(templatePath, []byte("{{.Name}}"), 0644); err != nil {
		t.Fatalf("failed to create template file: %v", err)
	}

	outputPath := filepath.Join(tempDir, "output.txt")

	// Invalid JSON
	err := RenderTemplateFromJSON([]byte(`{"Name":`), templatePath, outputPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRenderTemplateFromJSON_NonExistentTemplate(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "nonexistent.tmpl")
	outputPath := filepath.Join(tempDir, "output.txt")

	jsonData := `{"Name": "Test"}`
	err := RenderTemplateFromJSON([]byte(jsonData), templatePath, outputPath)
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
}

func TestRenderTemplateFromJSON_TemplateFunctions(t *testing.T) {
	tempDir := t.TempDir()

	// Template using custom functions
	templateContent := `<p>{{upper .Name}}</p><p>{{basename .Path}}</p><p>{{trimExt .File}}</p>`
	templatePath := filepath.Join(tempDir, "test.tmpl")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to create template file: %v", err)
	}

	jsonData := `{"Name": "test", "Path": "/path/to/file.txt", "File": "file.txt"}`
	outputPath := filepath.Join(tempDir, "output.html")

	err := RenderTemplateFromJSON([]byte(jsonData), templatePath, outputPath)
	if err != nil {
		t.Fatalf("RenderTemplateFromJSON failed: %v", err)
	}

	outputContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	expected := `<p>TEST</p><p>file.txt</p><p>file</p>`
	if string(outputContent) != expected {
		t.Fatalf("expected %q, got %q", expected, string(outputContent))
	}
}