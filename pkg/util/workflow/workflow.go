package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MCPRUNNER/gossisMCP/pkg/workflow"
)

// WorkflowExecutionSummary represents a summary of workflow execution
type WorkflowExecutionSummary struct {
	WorkflowPath string                `json:"workflow_path"`
	Steps        []WorkflowStepSummary `json:"steps"`
	FilesWritten []string              `json:"files_written,omitempty"`
}

// WorkflowStepSummary represents a summary of a workflow step
type WorkflowStepSummary struct {
	Name    string                         `json:"name"`
	Type    string                         `json:"type"`
	Enabled bool                           `json:"enabled"`
	Outputs map[string]workflow.StepResult `json:"outputs,omitempty"`
}

// CloneArguments creates a deep copy of map[string]interface{} arguments
func CloneArguments(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

// NormalizeWorkflowPathArg normalizes a workflow path argument
func NormalizeWorkflowPathArg(args map[string]interface{}, workflowPath, key string) {
	raw, exists := args[key]
	if !exists {
		return
	}
	value, ok := raw.(string)
	if !ok {
		return
	}
	if value == "" {
		return
	}
	if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "./") && !strings.HasPrefix(value, "../") {
		args[key] = "./" + value
	}
}

// NormalizeWorkflowPathArrayArg normalizes an array of workflow path arguments
func NormalizeWorkflowPathArrayArg(args map[string]interface{}, workflowPath, key string) {
	raw, exists := args[key]
	if !exists {
		return
	}

	switch v := raw.(type) {
	case []interface{}:
		for i, item := range v {
			if str, ok := item.(string); ok && str != "" {
				if !strings.HasPrefix(str, "/") && !strings.HasPrefix(str, "./") && !strings.HasPrefix(str, "../") {
					v[i] = "./" + str
				}
			}
		}
	case []string:
		for i, str := range v {
			if str != "" && !strings.HasPrefix(str, "/") && !strings.HasPrefix(str, "./") && !strings.HasPrefix(str, "../") {
				v[i] = "./" + str
			}
		}
	}
}

// StringFromAny converts an interface{} to string safely
func StringFromAny(value interface{}) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", value)
}

// ExtractStringArg extracts a string argument from the args map
func ExtractStringArg(args map[string]interface{}, key string) string {
	if val, exists := args[key]; exists {
		return StringFromAny(val)
	}
	return ""
}

// ExtractFilePathsFromJSON extracts file paths from JSON text
func ExtractFilePathsFromJSON(jsonText string) ([]string, error) {
	var filePaths []string

	// Pattern to match file paths in JSON strings
	patterns := []string{
		`"file_path"\s*:\s*"([^"]+)"`,
		`"path"\s*:\s*"([^"]+)"`,
		`"output_file_path"\s*:\s*"([^"]+)"`,
		`"template_file_path"\s*:\s*"([^"]+)"`,
		`"json_file_path"\s*:\s*"([^"]+)"`,
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllStringSubmatch(jsonText, -1)
		for _, match := range matches {
			if len(match) > 1 && match[1] != "" {
				filePaths = append(filePaths, match[1])
			}
		}
	}

	return filePaths, nil
}

// ToInterfaceSlice converts []string to []interface{}
func ToInterfaceSlice(values []string) []interface{} {
	result := make([]interface{}, len(values))
	for i, v := range values {
		result[i] = v
	}
	return result
}

// WriteWorkflowOutput writes content to a workflow output file
func WriteWorkflowOutput(outputPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create workflow output directory %s: %w", filepath.Dir(outputPath), err)
	}
	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write workflow output %s: %w", outputPath, err)
	}
	return nil
}

// CreateWorkflowExecutionSummary creates a summary of workflow execution
func CreateWorkflowExecutionSummary(workflowPath string, wf *workflow.Workflow, results map[string]map[string]workflow.StepResult, files []string) WorkflowExecutionSummary {
	summary := WorkflowExecutionSummary{
		WorkflowPath: workflowPath,
		Steps:        make([]WorkflowStepSummary, 0, len(wf.Steps)),
		FilesWritten: files,
	}

	for _, step := range wf.Steps {
		stepSummary := WorkflowStepSummary{
			Name:    step.Name,
			Type:    step.Type,
			Enabled: step.Enabled,
		}

		if stepResults, exists := results[step.Name]; exists {
			stepSummary.Outputs = stepResults
		}

		summary.Steps = append(summary.Steps, stepSummary)
	}

	return summary
}

// FormatWorkflowSummaryMarkdown formats workflow execution summary as markdown
func FormatWorkflowSummaryMarkdown(summary WorkflowExecutionSummary) string {
	var sb strings.Builder

	sb.WriteString("# Workflow Execution Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Workflow:** %s\n\n", summary.WorkflowPath))

	sb.WriteString("## Steps\n\n")
	for i, step := range summary.Steps {
		status := "✅"
		if !step.Enabled {
			status = "⏭️"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s (%s)\n", i+1, status, step.Name, step.Type))

		if len(step.Outputs) > 0 {
			sb.WriteString("   - Outputs:\n")
			for outputKey, output := range step.Outputs {
				sb.WriteString(fmt.Sprintf("     - %s: %s\n", outputKey, StringFromAny(output)))
			}
		}
		sb.WriteString("\n")
	}

	if len(summary.FilesWritten) > 0 {
		sb.WriteString("## Files Written\n\n")
		for _, file := range summary.FilesWritten {
			sb.WriteString(fmt.Sprintf("- %s\n", file))
		}
	}

	return sb.String()
}

// ExtractJSONObjects scans input for top-level JSON objects and returns them as raw JSON strings
func ExtractJSONObjects(s string) []string {
	var objects []string
	var current strings.Builder
	braceCount := 0
	bracketCount := 0
	inString := false
	escaped := false

	for _, r := range s {
		current.WriteRune(r)

		switch r {
		case '"':
			if !escaped {
				inString = !inString
			}
		case '\\':
			escaped = !escaped
		case '{':
			if !inString {
				braceCount++
			}
		case '}':
			if !inString {
				braceCount--
				if braceCount == 0 && bracketCount == 0 {
					objects = append(objects, strings.TrimSpace(current.String()))
					current.Reset()
				}
			}
		case '[':
			if !inString {
				bracketCount++
			}
		case ']':
			if !inString {
				bracketCount--
				if braceCount == 0 && bracketCount == 0 {
					objects = append(objects, strings.TrimSpace(current.String()))
					current.Reset()
				}
			}
		default:
			escaped = false
		}

		// Safety check to prevent infinite accumulation
		if current.Len() > 10000 {
			current.Reset()
			braceCount = 0
			bracketCount = 0
			inString = false
			escaped = false
		}
	}

	return objects
}

// ParseTopLevelJSONValues decodes one or more top-level JSON values from the provided string
func ParseTopLevelJSONValues(s string) ([]interface{}, error) {
	var values []interface{}

	// Try to parse as a single JSON value first
	var singleValue interface{}
	if err := json.Unmarshal([]byte(s), &singleValue); err == nil {
		return []interface{}{singleValue}, nil
	}

	// If that fails, try to extract multiple JSON objects
	jsonStrings := ExtractJSONObjects(s)
	if len(jsonStrings) == 0 {
		return nil, fmt.Errorf("no valid JSON found in input")
	}

	for _, jsonStr := range jsonStrings {
		var value interface{}
		if err := json.Unmarshal([]byte(jsonStr), &value); err != nil {
			// Skip invalid JSON objects
			continue
		}
		values = append(values, value)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no valid JSON objects found")
	}

	return values, nil
}
