package extraction

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/MCPRUNNER/gossisMCP/pkg/types"
)

func createRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: args},
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
	file := "sample.dtsx"
	expected := filepath.Join(base, file)
	if got := ResolveFilePath(file, base); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
	absolute := filepath.Join(base, "absolute.dtsx")
	if got := ResolveFilePath(absolute, base); got != absolute {
		t.Fatalf("expected absolute path %s, got %s", absolute, got)
	}
}

func TestResolveVariableExpressions(t *testing.T) {
	vars := []types.Variable{
		{Name: "CSV_DIRECTORY", Value: "C:/data"},
		{Name: "CSV_FILENAME", Value: "file.csv"},
	}
	value := "@[User::CSV_DIRECTORY]/@[User::CSV_FILENAME]"
	resolved := resolveVariableExpressions(value, vars, 5)
	if resolved != "C:/data/file.csv" {
		t.Fatalf("expected resolved expression, got %s", resolved)
	}
}

func TestFindVariableValue(t *testing.T) {
	vars := []types.Variable{{Name: "Target", Value: "value"}}
	if got := findVariableValue("Target", vars); got != "value" {
		t.Fatalf("expected value, got %s", got)
	}
	if missing := findVariableValue("Missing", vars); missing != "" {
		t.Fatalf("expected empty string for missing variable, got %s", missing)
	}
}

func TestHandleExtractConnections(t *testing.T) {
	path := testdataFile(t, "Expressions.dtsx")
	request := createRequest(map[string]interface{}{
		"file_path": filepath.Base(path),
	})
	result, err := HandleExtractConnections(context.Background(), request, filepath.Dir(path))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected connection extraction result")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(textContent.Text, "Connections:") {
		t.Fatalf("expected connection list, got %q", textContent.Text)
	}
}
