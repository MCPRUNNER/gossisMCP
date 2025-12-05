package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFile_InvokesRunnerAndReturnsResults(t *testing.T) {
	dir, err := os.MkdirTemp("", "wftest")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	wfPath := filepath.Join(dir, "workflow.json")
	content := `{"Steps":[{"Name":"S1","Type":"#dummy","Parameters":{},"Enabled":true}]}`
	if err := os.WriteFile(wfPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	runner := func(ctx context.Context, tool string, params map[string]interface{}) (string, error) {
		if tool != "dummy" {
			t.Fatalf("unexpected tool: %s", tool)
		}
		return "hello", nil
	}

	wf, results, err := RunFile(context.Background(), wfPath, runner)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}
	if wf == nil {
		t.Fatalf("expected workflow to be returned")
	}
	out, ok := results["S1"]["Result"]
	if !ok {
		t.Fatalf("expected result for step S1")
	}
	if out.Value != "hello" {
		t.Fatalf("unexpected result value: %s", out.Value)
	}
}

func TestWriteCombinedStepOutputs_WritesJSONArray(t *testing.T) {
	dir, err := os.MkdirTemp("", "wfout")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	wf := &Workflow{
		Steps: []Step{
			{
				Name:           "StepA",
				Output:         &StepOutput{Name: "Result", Format: "json"},
				OutputFilePath: "output/combined.json",
			},
		},
	}

	// concatenated JSON objects from a loop - simulates analyze_logging_configuration loop output
	results := map[string]map[string]StepResult{
		"StepA": {
			"Result": {Value: `{"file":"test1.dtsx","analysis":"result1"}
{"file":"test2.dtsx","analysis":"result2"}`, Format: "json"},
		},
	}

	wfPath := filepath.Join(dir, "wf.json")
	written, err := WriteCombinedStepOutputs(wfPath, wf, results)
	if err != nil {
		t.Fatalf("WriteCombinedStepOutputs failed: %v", err)
	}
	if len(written) == 0 {
		t.Fatalf("expected at least one written file")
	}

	combinedPath := filepath.Join(dir, "output", "combined.json")
	data, err := os.ReadFile(combinedPath)
	if err != nil {
		t.Fatalf("failed to read combined file: %v", err)
	}

	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("combined file is not valid JSON object: %v", err)
	}
	arr, ok := wrapper["data"].([]interface{})
	if !ok {
		t.Fatalf("combined file did not contain a top-level 'data' array")
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements in combined 'data' array, got %d", len(arr))
	}

	// Verify that analysis field was normalized to results
	obj1, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first element to be an object")
	}
	if _, hasResults := obj1["results"]; !hasResults {
		t.Fatalf("expected 'results' field in first element")
	}
	if _, hasAnalysis := obj1["analysis"]; hasAnalysis {
		t.Fatalf("expected 'analysis' field to be removed from first element")
	}

	// Verify package field was added
	if pkg, ok := obj1["package"].(string); !ok || pkg != "test1" {
		t.Fatalf("expected 'package' field to be 'test1', got %v", obj1["package"])
	}
}

func TestWriteCombinedStepOutputs_PreservesSingleJSONArray(t *testing.T) {
	dir, err := os.MkdirTemp("", "wfout2")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	wf := &Workflow{
		Steps: []Step{
			{
				Name:           "StepA",
				Output:         &StepOutput{Name: "Result", Format: "json"},
				OutputFilePath: "out/single_array.json",
			},
		},
	}

	// single JSON array string
	results := map[string]map[string]StepResult{
		"StepA": {
			"Result": {Value: `[1,2,3]`, Format: "json"},
		},
	}

	wfPath := filepath.Join(dir, "wf.json")
	written, err := WriteCombinedStepOutputs(wfPath, wf, results)
	if err != nil {
		t.Fatalf("WriteCombinedStepOutputs failed: %v", err)
	}
	if len(written) == 0 {
		t.Fatalf("expected at least one written file")
	}

	combinedPath := filepath.Join(dir, "out", "single_array.json")
	data, err := os.ReadFile(combinedPath)
	if err != nil {
		t.Fatalf("failed to read combined file: %v", err)
	}

	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		t.Fatalf("combined file is not valid JSON object: %v", err)
	}
	arr, ok := wrapper["data"].([]interface{})
	if !ok {
		t.Fatalf("combined file did not contain a top-level 'data' array")
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements in 'data' array, got %d", len(arr))
	}
}

func TestWriteCombinedStepOutputs_WritesPlainTextUnchanged(t *testing.T) {
	dir, err := os.MkdirTemp("", "wfout3")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	wf := &Workflow{
		Steps: []Step{
			{
				Name:           "StepB",
				Output:         &StepOutput{Name: "Result", Format: "text"},
				OutputFilePath: "out/plain.txt",
			},
		},
	}

	results := map[string]map[string]StepResult{
		"StepB": {
			"Result": {Value: `This is plain text output`, Format: "text"},
		},
	}

	wfPath := filepath.Join(dir, "wf.json")
	written, err := WriteCombinedStepOutputs(wfPath, wf, results)
	if err != nil {
		t.Fatalf("WriteCombinedStepOutputs failed: %v", err)
	}
	if len(written) == 0 {
		t.Fatalf("expected at least one written file")
	}

	combinedPath := filepath.Join(dir, "out", "plain.txt")
	data, err := os.ReadFile(combinedPath)
	if err != nil {
		t.Fatalf("failed to read combined file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "plain text output") {
		t.Fatalf("plain text not preserved, got: %s", content)
	}
	if !strings.HasSuffix(content, "\n") {
		t.Fatalf("expected newline suffix in written file")
	}
}
