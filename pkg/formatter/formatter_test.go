package formatter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetFormatterDefault(t *testing.T) {
	formatter := GetFormatter(OutputFormat("unknown"))
	if _, ok := formatter.(*TextFormatter); !ok {
		t.Fatalf("expected fallback to TextFormatter, got %T", formatter)
	}
}

func TestCreateAnalysisResultError(t *testing.T) {
	result := CreateAnalysisResult("test", "file", nil, assertError{})
	if result.Status != "error" {
		t.Fatalf("expected status error, got %s", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected error message to be populated")
	}
}

type assertError struct{}

func (assertError) Error() string {
	return "failed"
}

func TestFormatAnalysisResultText(t *testing.T) {
	result := CreateAnalysisResult("Data Flow", "path", "analysis complete", nil)
	output := FormatAnalysisResult(result, FormatText)
	if !strings.Contains(output, "Data Flow Analysis Report") {
		t.Fatalf("expected output to contain report header, got %q", output)
	}
}

func TestCSVFormatterWithTable(t *testing.T) {
	table := &TableData{
		Headers: []string{"Column"},
		Rows:    [][]string{{"value"}},
	}
	result := &AnalysisResult{Data: table}
	output := (&CSVFormatter{}).Format(result)
	if !strings.Contains(output, "Column") || !strings.Contains(output, "value") {
		t.Fatalf("unexpected CSV output: %q", output)
	}
}

func TestHTMLFormatterError(t *testing.T) {
	result := &AnalysisResult{Error: "boom"}
	output := (&HTMLFormatter{}).Format(result)
	if !strings.Contains(output, "class=\"error\"") {
		t.Fatalf("expected error styling in HTML, got %q", output)
	}
}

func TestMarkdownFormatterList(t *testing.T) {
	result := &AnalysisResult{ToolName: "Lister", FilePath: "file", Timestamp: "now", Data: []string{"one", "two"}}
	output := (&MarkdownFormatter{}).Format(result)
	if !strings.Contains(output, "- one") || !strings.Contains(output, "# Lister Analysis Report") {
		t.Fatalf("unexpected markdown output: %q", output)
	}
}

func TestJSONFormatter(t *testing.T) {
	result := &AnalysisResult{ToolName: "JSON", FilePath: "file", Timestamp: "now"}
	output := (&JSONFormatter{}).Format(result)
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("expected valid JSON, got error %v", err)
	}
	if decoded["tool_name"] != "JSON" {
		t.Fatalf("expected tool_name JSON, got %v", decoded["tool_name"])
	}
}
