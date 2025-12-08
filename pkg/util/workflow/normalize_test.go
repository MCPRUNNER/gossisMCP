package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeWorkflowPathArgWithResolution(t *testing.T) {
	// Create platform-agnostic workflow path using temp dir
	workflowPath := filepath.Join(os.TempDir(), "workflows", "workflow_merge_example.json")
	workflowDir := filepath.Dir(workflowPath)
	absolutePath := filepath.Join(os.TempDir(), "absolute", "path", "file.json")
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute path unchanged",
			input:    absolutePath,
			expected: absolutePath,
		},
		{
			name:     "relative path with ../",
			input:    "../reports/dataflow_analysis.json",
			expected: filepath.Join(workflowDir, "..", "reports", "dataflow_analysis.json"),
		},
		{
			name:     "relative path with ./",
			input:    "./file.json",
			expected: filepath.Join(workflowDir, ".", "file.json"),
		},
		{
			name:     "bare relative path",
			input:    "file.json",
			expected: "file.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"test_path": tt.input,
			}

			NormalizeWorkflowPathArg(args, workflowPath, "test_path")

			result, ok := args["test_path"].(string)
			if !ok {
				t.Fatalf("expected string result")
			}

			// Clean both paths for comparison
			result = filepath.Clean(result)
			expected := filepath.Clean(tt.expected)

			if result != expected {
				t.Errorf("expected %s, got %s", expected, result)
			}
		})
	}
}
