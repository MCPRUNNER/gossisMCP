package workflow

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCloneArguments(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: map[string]interface{}{},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "map with values",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CloneArguments(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("CloneArguments() = %v, want %v", result, tt.expected)
			}

			// Ensure it's a deep copy by modifying the result
			if len(result) > 0 {
				result["new_key"] = "new_value"
				if reflect.DeepEqual(result, tt.input) {
					t.Error("CloneArguments() did not create a deep copy")
				}
			}
		})
	}
}

func TestNormalizeWorkflowPathArg(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		workflowPath string
		key          string
		expected     map[string]interface{}
	}{
		{
			name: "relative path without prefix",
			args: map[string]interface{}{
				"file_path": "test.dtsx",
			},
			workflowPath: "/workflow",
			key:          "file_path",
			expected: map[string]interface{}{
				"file_path": "test.dtsx",
			},
		},
		{
			name: "already has ./ prefix",
			args: map[string]interface{}{
				"file_path": "./test.dtsx",
			},
			workflowPath: "/workflow",
			key:          "file_path",
			expected: map[string]interface{}{
				"file_path": filepath.Join("/", "./test.dtsx"),
			},
		},
		{
			name: "absolute path",
			args: map[string]interface{}{
				"file_path": "/absolute/path/test.dtsx",
			},
			workflowPath: "/workflow",
			key:          "file_path",
			expected: map[string]interface{}{
				"file_path": "/absolute/path/test.dtsx",
			},
		},
		{
			name: "empty value",
			args: map[string]interface{}{
				"file_path": "",
			},
			workflowPath: "/workflow",
			key:          "file_path",
			expected: map[string]interface{}{
				"file_path": "",
			},
		},
		{
			name: "non-string value",
			args: map[string]interface{}{
				"file_path": 123,
			},
			workflowPath: "/workflow",
			key:          "file_path",
			expected: map[string]interface{}{
				"file_path": 123,
			},
		},
		{
			name: "key does not exist",
			args: map[string]interface{}{
				"other_key": "value",
			},
			workflowPath: "/workflow",
			key:          "file_path",
			expected: map[string]interface{}{
				"other_key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of args to avoid modifying the original
			args := make(map[string]interface{})
			for k, v := range tt.args {
				args[k] = v
			}

			NormalizeWorkflowPathArg(args, tt.workflowPath, tt.key)

			if !reflect.DeepEqual(args, tt.expected) {
				t.Errorf("NormalizeWorkflowPathArg() = %v, want %v", args, tt.expected)
			}
		})
	}
}

func TestStringFromAny(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
		},
		{
			name:     "string value",
			input:    "test string",
			expected: "test string",
		},
		{
			name:     "int value",
			input:    42,
			expected: "42",
		},
		{
			name:     "bool value",
			input:    true,
			expected: "true",
		},
		{
			name:     "float value",
			input:    3.14,
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringFromAny(tt.input)
			if result != tt.expected {
				t.Errorf("StringFromAny() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractStringArg(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected string
	}{
		{
			name: "existing string key",
			args: map[string]interface{}{
				"file_path": "test.dtsx",
			},
			key:      "file_path",
			expected: "test.dtsx",
		},
		{
			name: "existing non-string key",
			args: map[string]interface{}{
				"count": 42,
			},
			key:      "count",
			expected: "42",
		},
		{
			name:     "non-existing key",
			args:     map[string]interface{}{},
			key:      "missing",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractStringArg(tt.args, tt.key)
			if result != tt.expected {
				t.Errorf("ExtractStringArg() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractFilePathsFromJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonText    string
		expected    []string
		expectError bool
	}{
		{
			name:     "single file_path",
			jsonText: `{"file_path": "test.dtsx"}`,
			expected: []string{"test.dtsx"},
		},
		{
			name:     "multiple file paths",
			jsonText: `{"file_path": "test.dtsx", "output_file_path": "output.txt", "template_file_path": "template.html"}`,
			expected: []string{"test.dtsx", "output.txt", "template.html"},
		},
		{
			name:     "path field",
			jsonText: `{"path": "/some/path/file.txt"}`,
			expected: []string{"/some/path/file.txt"},
		},
		{
			name:     "no file paths",
			jsonText: `{"name": "test", "value": 123}`,
			expected: []string{},
		},
		{
			name:     "empty JSON",
			jsonText: `{}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractFilePathsFromJSON(tt.jsonText)
			if (err != nil) != tt.expectError {
				t.Errorf("ExtractFilePathsFromJSON() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("ExtractFilePathsFromJSON() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("ExtractFilePathsFromJSON()[%d] = %v, want %v", i, result[i], expected)
				}
			}
		})
	}
}

func TestToInterfaceSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []interface{}
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []interface{}{},
		},
		{
			name:     "single element",
			input:    []string{"test"},
			expected: []interface{}{"test"},
		},
		{
			name:     "multiple elements",
			input:    []string{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToInterfaceSlice(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ToInterfaceSlice() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractJSONObjects(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single object",
			input:    `{"key": "value"}`,
			expected: []string{`{"key": "value"}`},
		},
		{
			name:     "multiple objects",
			input:    `{"key1": "value1"}{"key2": "value2"}`,
			expected: []string{`{"key1": "value1"}`, `{"key2": "value2"}`},
		},
		{
			name:     "object with nested braces",
			input:    `{"data": {"nested": "value"}}`,
			expected: []string{`{"data": {"nested": "value"}}`},
		},
		{
			name:     "mixed content",
			input:    `text {"key": "value"} more text`,
			expected: []string{`text {"key": "value"}`},
		},
		{
			name:     "no objects",
			input:    "just plain text",
			expected: []string{},
		},
		{
			name:     "array",
			input:    `["item1", "item2"]`,
			expected: []string{`["item1", "item2"]`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSONObjects(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ExtractJSONObjects() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("ExtractJSONObjects()[%d] = %v, want %v", i, result[i], expected)
				}
			}
		})
	}
}

func TestParseTopLevelJSONValues(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []interface{}
		expectError bool
	}{
		{
			name:     "single object",
			input:    `{"key": "value"}`,
			expected: []interface{}{map[string]interface{}{"key": "value"}},
		},
		{
			name:     "single array",
			input:    `["item1", "item2"]`,
			expected: []interface{}{[]interface{}{"item1", "item2"}},
		},
		{
			name:  "multiple objects",
			input: `{"key1": "value1"}{"key2": "value2"}`,
			expected: []interface{}{
				map[string]interface{}{"key1": "value1"},
				map[string]interface{}{"key2": "value2"},
			},
		},
		{
			name:        "invalid JSON",
			input:       `{"invalid": json}`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "no JSON",
			input:       "plain text",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTopLevelJSONValues(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("ParseTopLevelJSONValues() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !tt.expectError && !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseTopLevelJSONValues() = %v, want %v", result, tt.expected)
			}
		})
	}
}
