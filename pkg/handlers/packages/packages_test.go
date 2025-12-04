package packages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MCPRUNNER/gossisMCP/pkg/types"
)

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

func locateTestdata(t *testing.T, name string) (string, string) {
	root := repoRoot(t)
	candidates := []string{
		filepath.Join(root, "testdata", name),
		filepath.Join(root, "Documents", "SSIS_EXAMPLES", name),
		filepath.Join(root, "Documents", "Query_EXAMPLES", "SSIS_EXAMPLES", name),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return filepath.Dir(path), filepath.Base(path)
		}
	}
	fallback := candidates[len(candidates)-1]
	return filepath.Dir(fallback), filepath.Base(fallback)
}

func TestListPackages(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}
	files := []string{
		filepath.Join(dir, "one.dtsx"),
		filepath.Join(nested, "two.dtsx"),
		filepath.Join(dir, "ignore.txt"),
	}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("<xml />"), 0o644); err != nil {
			t.Fatalf("failed to create file %s: %v", file, err)
		}
	}
	results, err := ListPackages(dir, "")
	if err != nil {
		t.Fatalf("unexpected error listing packages: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected two DTSX files, got %v", results)
	}

	excludePath := filepath.Join(dir, ".gossisignore")
	if err := os.WriteFile(excludePath, []byte("nested/\n"), 0o644); err != nil {
		t.Fatalf("failed to write exclude file: %v", err)
	}
	filtered, err := ListPackages(dir, "")
	if err != nil {
		t.Fatalf("unexpected error listing packages with exclude: %v", err)
	}
	if len(filtered) != 1 || !strings.Contains(filtered[0], "one.dtsx") {
		t.Fatalf("expected exclude file to remove nested package, got %v", filtered)
	}
}

func TestResolveFilePath(t *testing.T) {
	base := t.TempDir()
	file := "sample.dtsx"
	want := filepath.Join(base, file)
	if got := resolveFilePath(file, base); got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestPerformBatchPackageAnalysis(t *testing.T) {
	dir, file := locateTestdata(t, "Expressions.dtsx")
	data, err := performBatchPackageAnalysis(file, dir)
	if err != nil {
		t.Fatalf("unexpected error performing analysis: %v", err)
	}
	if data["task_count"].(int) == 0 {
		t.Fatal("expected task count to be reported")
	}
}

func TestDescribeTaskType(t *testing.T) {
	task := types.Task{Properties: []types.Property{{Name: "CreationName", Value: "Microsoft.ExecuteSQLTask"}}}
	if desc := describeTaskType(task); desc != "Execute SQL Task" {
		t.Fatalf("expected friendly description, got %s", desc)
	}
}

func TestFormatters(t *testing.T) {
	summary := batchSummary{
		TotalPackages:   1,
		Successful:      1,
		Failed:          0,
		TotalDuration:   time.Second,
		AverageDuration: time.Second,
		PackageSummaries: []batchAnalysisResult{{
			PackagePath: "pkg.dtsx",
			Success:     true,
			Duration:    time.Millisecond,
		}},
	}

	text := formatBatchSummaryAsText(summary)
	if !strings.Contains(text, "Total Packages: 1") {
		t.Fatalf("unexpected text summary: %q", text)
	}

	csv := formatBatchSummaryAsCSV(summary)
	if !strings.Contains(csv, "pkg.dtsx") {
		t.Fatalf("unexpected csv summary: %q", csv)
	}

	html := formatBatchSummaryAsHTML(summary)
	if !strings.Contains(html, "<table>") {
		t.Fatalf("unexpected html summary")
	}

	markdown := formatBatchSummaryAsMarkdown(summary)
	if !strings.Contains(markdown, "# Batch Analysis Summary") {
		t.Fatalf("unexpected markdown summary")
	}
}

func TestExtractPropertyAndSection(t *testing.T) {
	xml := `<Parent><Property Name="ReadOnlyVariables">User::A</Property></Parent>`
	if value := extractPropertyValue(xml, "ReadOnlyVariables"); value != "User::A" {
		t.Fatalf("expected property value, got %s", value)
	}
	section := extractSection("<A>content</A>", "<A>", "</A>")
	if section != "<A>content</A>" {
		t.Fatalf("expected full section, got %s", section)
	}
}
