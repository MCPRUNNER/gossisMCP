package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MCPRUNNER/gossisMCP/pkg/workflow"
)

func TestNormalizeWorkflowPathArrayArg(t *testing.T) {
	// Use platform-specific absolute paths for testing
	absPath1 := filepath.Join(string(filepath.Separator), "absolute", "file1.dtsx")
	absPath2 := filepath.Join(string(filepath.Separator), "absolute", "file2.dtsx")
	workflowPath := filepath.Join(string(filepath.Separator), "workflows", "test.yaml")
	workflowDir := filepath.Dir(workflowPath)

	tests := []struct {
		name         string
		args         map[string]interface{}
		workflowPath string
		key          string
		expected     map[string]interface{}
	}{
		{
			name: "interface{} slice with relative paths",
			args: map[string]interface{}{
				"file_paths": []interface{}{"file1.dtsx", "./file2.dtsx", "../file3.dtsx"},
			},
			workflowPath: workflowPath,
			key:          "file_paths",
			expected: map[string]interface{}{
				"file_paths": []interface{}{"file1.dtsx", filepath.Join(workflowDir, "./file2.dtsx"), filepath.Join(workflowDir, "../file3.dtsx")},
			},
		},
		{
			name: "string slice with absolute paths",
			args: map[string]interface{}{
				"file_paths": []string{absPath1, absPath2},
			},
			workflowPath: workflowPath,
			key:          "file_paths",
			expected: map[string]interface{}{
				"file_paths": []string{absPath1, absPath2},
			},
		},
		{
			name: "mixed relative and absolute paths",
			args: map[string]interface{}{
				"file_paths": []interface{}{absPath1, "./relative.dtsx"},
			},
			workflowPath: workflowPath,
			key:          "file_paths",
			expected: map[string]interface{}{
				"file_paths": []interface{}{absPath1, filepath.Join(workflowDir, "./relative.dtsx")},
			},
		},
		{
			name: "empty workflow path",
			args: map[string]interface{}{
				"file_paths": []interface{}{"./file.dtsx"},
			},
			workflowPath: "",
			key:          "file_paths",
			expected: map[string]interface{}{
				"file_paths": []interface{}{"./file.dtsx"},
			},
		},
		{
			name: "key does not exist",
			args: map[string]interface{}{
				"other_key": "value",
			},
			workflowPath: "/workflows/test.yaml",
			key:          "file_paths",
			expected: map[string]interface{}{
				"other_key": "value",
			},
		},
		{
			name: "empty strings in array",
			args: map[string]interface{}{
				"file_paths": []interface{}{"", "file.dtsx", ""},
			},
			workflowPath: "/workflows/test.yaml",
			key:          "file_paths",
			expected: map[string]interface{}{
				"file_paths": []interface{}{"", "file.dtsx", ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of args
			args := make(map[string]interface{})
			for k, v := range tt.args {
				args[k] = v
			}

			NormalizeWorkflowPathArrayArg(args, tt.workflowPath, tt.key)

			// Compare the results
			if val, exists := args[tt.key]; exists {
				expectedVal := tt.expected[tt.key]

				// Handle different slice types
				switch v := val.(type) {
				case []interface{}:
					expected, ok := expectedVal.([]interface{})
					if !ok {
						t.Errorf("Expected type mismatch")
						return
					}
					if len(v) != len(expected) {
						t.Errorf("Length mismatch: got %d, want %d", len(v), len(expected))
						return
					}
					for i := range v {
						if v[i] != expected[i] {
							t.Errorf("Element %d: got %v, want %v", i, v[i], expected[i])
						}
					}
				case []string:
					expected, ok := expectedVal.([]string)
					if !ok {
						t.Errorf("Expected type mismatch")
						return
					}
					if len(v) != len(expected) {
						t.Errorf("Length mismatch: got %d, want %d", len(v), len(expected))
						return
					}
					for i := range v {
						if v[i] != expected[i] {
							t.Errorf("Element %d: got %v, want %v", i, v[i], expected[i])
						}
					}
				}
			} else if _, expectedExists := tt.expected[tt.key]; expectedExists {
				t.Errorf("Key %s missing from result", tt.key)
			}
		})
	}
}

func TestWriteWorkflowOutput(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		outputPath  string
		content     string
		expectError bool
	}{
		{
			name:        "write to new file",
			outputPath:  filepath.Join(tempDir, "output.txt"),
			content:     "test content",
			expectError: false,
		},
		{
			name:        "write to file in nested directory",
			outputPath:  filepath.Join(tempDir, "nested", "dir", "output.txt"),
			content:     "nested content",
			expectError: false,
		},
		{
			name:        "overwrite existing file",
			outputPath:  filepath.Join(tempDir, "overwrite.txt"),
			content:     "new content",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For overwrite test, create the file first
			if tt.name == "overwrite existing file" {
				_ = os.WriteFile(tt.outputPath, []byte("old content"), 0o644)
			}

			err := WriteWorkflowOutput(tt.outputPath, tt.content)

			if (err != nil) != tt.expectError {
				t.Errorf("WriteWorkflowOutput() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				// Verify file was written correctly
				data, err := os.ReadFile(tt.outputPath)
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
					return
				}
				if string(data) != tt.content {
					t.Errorf("File content = %q, want %q", string(data), tt.content)
				}
			}
		})
	}
}

func TestCreateWorkflowExecutionSummary(t *testing.T) {
	tests := []struct {
		name         string
		workflowPath string
		workflow     *workflow.Workflow
		results      map[string]map[string]workflow.StepResult
		files        []string
		expected     WorkflowExecutionSummary
	}{
		{
			name:         "simple workflow with one step",
			workflowPath: "/workflows/test.yaml",
			workflow: &workflow.Workflow{
				Steps: []workflow.Step{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: true,
					},
				},
			},
			results: map[string]map[string]workflow.StepResult{
				"step1": {
					"output": workflow.StepResult{
						Value:  "result data",
						Format: "text",
					},
				},
			},
			files: []string{"/output/file1.txt"},
			expected: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: true,
						Outputs: map[string]workflow.StepResult{
							"output": {
								Value:  "result data",
								Format: "text",
							},
						},
					},
				},
				FilesWritten: []string{"/output/file1.txt"},
			},
		},
		{
			name:         "workflow with disabled step",
			workflowPath: "/workflows/test.yaml",
			workflow: &workflow.Workflow{
				Steps: []workflow.Step{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: false,
					},
				},
			},
			results: map[string]map[string]workflow.StepResult{},
			files:   []string{},
			expected: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: false,
					},
				},
				FilesWritten: []string{},
			},
		},
		{
			name:         "multiple steps",
			workflowPath: "/workflows/test.yaml",
			workflow: &workflow.Workflow{
				Steps: []workflow.Step{
					{Name: "step1", Type: "parse_dtsx", Enabled: true},
					{Name: "step2", Type: "analyze_data_flow", Enabled: true},
				},
			},
			results: map[string]map[string]workflow.StepResult{
				"step1": {"out1": workflow.StepResult{Value: "data1", Format: "text"}},
				"step2": {"out2": workflow.StepResult{Value: "data2", Format: "json"}},
			},
			files: []string{"/output/file1.txt", "/output/file2.txt"},
			expected: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: true,
						Outputs: map[string]workflow.StepResult{
							"out1": {Value: "data1", Format: "text"},
						},
					},
					{
						Name:    "step2",
						Type:    "analyze_data_flow",
						Enabled: true,
						Outputs: map[string]workflow.StepResult{
							"out2": {Value: "data2", Format: "json"},
						},
					},
				},
				FilesWritten: []string{"/output/file1.txt", "/output/file2.txt"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateWorkflowExecutionSummary(tt.workflowPath, tt.workflow, tt.results, tt.files)

			if result.WorkflowPath != tt.expected.WorkflowPath {
				t.Errorf("WorkflowPath = %q, want %q", result.WorkflowPath, tt.expected.WorkflowPath)
			}

			if len(result.Steps) != len(tt.expected.Steps) {
				t.Errorf("Steps length = %d, want %d", len(result.Steps), len(tt.expected.Steps))
				return
			}

			for i, step := range result.Steps {
				expectedStep := tt.expected.Steps[i]
				if step.Name != expectedStep.Name {
					t.Errorf("Step %d Name = %q, want %q", i, step.Name, expectedStep.Name)
				}
				if step.Type != expectedStep.Type {
					t.Errorf("Step %d Type = %q, want %q", i, step.Type, expectedStep.Type)
				}
				if step.Enabled != expectedStep.Enabled {
					t.Errorf("Step %d Enabled = %v, want %v", i, step.Enabled, expectedStep.Enabled)
				}
			}

			if len(result.FilesWritten) != len(tt.expected.FilesWritten) {
				t.Errorf("FilesWritten length = %d, want %d", len(result.FilesWritten), len(tt.expected.FilesWritten))
			}
		})
	}
}

func TestFormatWorkflowSummaryMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		summary  WorkflowExecutionSummary
		contains []string
	}{
		{
			name: "basic summary",
			summary: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: true,
					},
				},
				FilesWritten: []string{"/output/file1.txt"},
			},
			contains: []string{
				"# Workflow Execution Summary",
				"**Workflow:** /workflows/test.yaml",
				"## Steps",
				"1. ✅ step1 (parse_dtsx)",
				"## Files Written",
				"- /output/file1.txt",
			},
		},
		{
			name: "disabled step",
			summary: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: false,
					},
				},
			},
			contains: []string{
				"1. ⏭️ step1 (parse_dtsx)",
			},
		},
		{
			name: "step with outputs",
			summary: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{
						Name:    "step1",
						Type:    "parse_dtsx",
						Enabled: true,
						Outputs: map[string]workflow.StepResult{
							"result": {
								Value:  "test data",
								Format: "text",
							},
						},
					},
				},
			},
			contains: []string{
				"1. ✅ step1 (parse_dtsx)",
				"- Outputs:",
			},
		},
		{
			name: "multiple steps and files",
			summary: WorkflowExecutionSummary{
				WorkflowPath: "/workflows/test.yaml",
				Steps: []WorkflowStepSummary{
					{Name: "step1", Type: "parse_dtsx", Enabled: true},
					{Name: "step2", Type: "analyze_data_flow", Enabled: true},
				},
				FilesWritten: []string{"/output/file1.txt", "/output/file2.txt"},
			},
			contains: []string{
				"1. ✅ step1 (parse_dtsx)",
				"2. ✅ step2 (analyze_data_flow)",
				"- /output/file1.txt",
				"- /output/file2.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatWorkflowSummaryMarkdown(tt.summary)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("FormatWorkflowSummaryMarkdown() output missing expected string %q", expected)
					t.Logf("Output:\n%s", result)
				}
			}
		})
	}
}
