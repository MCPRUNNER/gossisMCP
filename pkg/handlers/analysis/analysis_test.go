package analysis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func createRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate repository root")
	return ""
}

func testdataFile(t *testing.T, name string) string {
	root := repoRoot(t)
	candidates := []string{
		filepath.Join(root, "testdata", name),
		filepath.Join(root, "Documents", "SSIS_EXAMPLES", name),
		filepath.Join(root, "Documents", "Query_EXAMPLES", "SSIS_EXAMPLES", name),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return candidates[len(candidates)-1]
}

func TestResolveFilePath(t *testing.T) {
	base := t.TempDir()
	absolute := filepath.Join(base, "file.dtsx")
	if got := ResolveFilePath(absolute, base); got != absolute {
		t.Fatalf("expected absolute path to pass through, got %s", got)
	}
	relative := "file.dtsx"
	expected := filepath.Join(base, relative)
	if got := ResolveFilePath(relative, base); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
}

func TestGetComponentType(t *testing.T) {
	cases := map[string]string{
		"Microsoft.OLEDBSource":         "Source",
		"Microsoft.FlatFileDestination": "Destination",
		"Custom.Component":              "Unknown",
	}
	for classID, expected := range cases {
		if got := getComponentType(classID); got != expected {
			t.Fatalf("expected %s for %s, got %s", expected, classID, got)
		}
	}
}

func TestHandleAnalyzeDataFlowSuccess(t *testing.T) {
	dir := filepath.Dir(testdataFile(t, "Expressions.dtsx"))
	request := createRequest(map[string]interface{}{
		"file_path": "Expressions.dtsx",
		"format":    "text",
	})
	result, err := HandleAnalyzeDataFlow(context.Background(), request, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatalf("expected analysis result content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(textContent.Text, "Data Flow Analysis") {
		t.Fatalf("expected report header, got %q", textContent.Text)
	}
}

func TestHandleAnalyzeDataFlowMissingFile(t *testing.T) {
	missingPath := filepath.Join(repoRoot(t), "testdata", "missing.dtsx")
	request := createRequest(map[string]interface{}{
		"file_path": missingPath,
		"format":    "text",
	})
	result, err := HandleAnalyzeDataFlow(context.Background(), request, "")
	if err != nil {
		t.Fatalf("expected handler to wrap error in result, got %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected error result content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(textContent.Text, "Error:") {
		t.Fatalf("expected error message, got %q", textContent.Text)
	}
}
