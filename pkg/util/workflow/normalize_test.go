package workflow

import (
	"path/filepath"
	"testing"
)

func TestNormalizeWorkflowPathArgWithResolution(t *testing.T) {
	workflowPath := `C:\Users\U00001\source\repos\gossisMCP\.gossismcp\workflows\workflow_merge_example.json`

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute path unchanged",
			input:    `C:\absolute\path\file.json`,
			expected: `C:\absolute\path\file.json`,
		},
		{
			name:     "relative path with ../",
			input:    `../reports/dataflow_analysis.json`,
			expected: `C:\Users\U00001\source\repos\gossisMCP\.gossismcp\reports\dataflow_analysis.json`,
		},
		{
			name:     "relative path with ./",
			input:    `./file.json`,
			expected: `C:\Users\U00001\source\repos\gossisMCP\.gossismcp\workflows\file.json`,
		},
		{
			name:     "bare relative path",
			input:    `file.json`,
			expected: `./file.json`,
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
