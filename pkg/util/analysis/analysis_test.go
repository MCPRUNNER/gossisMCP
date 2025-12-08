package analysis

import (
	"strings"
	"testing"
)

func TestAnalyzeBatchFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "basic batch file",
			content: `REM This is a comment
SET MY_VAR=value
CALL another.bat
some.exe /param
:: Another comment
SET ANOTHER_VAR=123`,
			expected: "Batch File Analysis:\n- Variables found: 2\n  Variables:\n    - MY_VAR\n    - ANOTHER_VAR\n- Commands found: 2\n  Commands:\n    - CALL another.bat\n    - some.exe /param\n- CALL statements: 1\n  Calls:\n    - another.bat\n",
		},
		{
			name:     "empty file",
			content:  "",
			expected: "Batch File Analysis:\n- Variables found: 0\n- Commands found: 0\n- CALL statements: 0\n",
		},
		{
			name:     "only comments",
			content:  "REM comment\n:: comment\n# not a comment",
			expected: "Batch File Analysis:\n- Variables found: 0\n- Commands found: 0\n- CALL statements: 0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result strings.Builder
			AnalyzeBatchFile(tt.content, false, &result)
			if result.String() != tt.expected {
				t.Errorf("AnalyzeBatchFile() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}

func TestAnalyzeConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "basic config file",
			content: `# Comment
[section1]
key1=value1
key2=value2

[section2]
key3=value3`,
			expected: "Configuration File Analysis:\n- Sections found: 2\n  Section [section1]: 2 entries\n    - key1=value1\n    - key2=value2\n  Section [section2]: 1 entries\n    - key3=value3\n",
		},
		{
			name:     "empty config",
			content:  "",
			expected: "Configuration File Analysis:\n- Sections found: 0\n",
		},
		{
			name:     "only comments",
			content:  "# comment\n; comment",
			expected: "Configuration File Analysis:\n- Sections found: 0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result strings.Builder
			AnalyzeConfigFile(tt.content, false, &result)
			if result.String() != tt.expected {
				t.Errorf("AnalyzeConfigFile() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}

func TestAnalyzeSQLFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "basic SQL file",
			content: `SELECT * FROM users;
INSERT INTO users VALUES (1, 'test');
UPDATE users SET name = 'new' WHERE id = 1;
DELETE FROM users WHERE id = 1;
CREATE TABLE test (id INT);
DROP TABLE test;
EXECUTE sp_test;`,
			expected: "SQL File Analysis:\n- SELECT statements: 1\n- INSERT statements: 1\n- UPDATE statements: 1\n- DELETE statements: 1\n- CREATE statements: 1\n- DROP statements: 1\n- SSIS-related patterns found:\n  - EXECUTE: 1\n",
		},
		{
			name:     "no SQL statements",
			content:  "This is not SQL",
			expected: "SQL File Analysis:\n- SELECT statements: 0\n- INSERT statements: 0\n- UPDATE statements: 0\n- DELETE statements: 0\n- CREATE statements: 0\n- DROP statements: 0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result strings.Builder
			AnalyzeSQLFile(tt.content, false, &result)
			if result.String() != tt.expected {
				t.Errorf("AnalyzeSQLFile() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}

func TestAnalyzeGenericTextFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "basic text file",
			content:  "line 1\nline 2\n\nline 4",
			expected: "Generic Text File Analysis:\n- Total lines: 4\n- Non-empty lines: 3\n- Total characters: 18\n- Average line length: 4.5 characters\n",
		},
		{
			name:     "XML content",
			content:  "<?xml version=\"1.0\"?><root><item>test</item></root>",
			expected: "Generic Text File Analysis:\n- Total lines: 1\n- Non-empty lines: 1\n- Total characters: 51\n- Average line length: 51.0 characters\n- Appears to be XML content\n",
		},
		{
			name:     "JSON content",
			content:  "{\"key\": \"value\"}",
			expected: "Generic Text File Analysis:\n- Total lines: 1\n- Non-empty lines: 1\n- Total characters: 16\n- Average line length: 16.0 characters\n- Appears to contain JSON-like structures\n",
		},
		{
			name:     "IP addresses",
			content:  "Server IP: 192.168.1.1 and 10.0.0.1",
			expected: "Generic Text File Analysis:\n- Total lines: 1\n- Non-empty lines: 1\n- Total characters: 35\n- Average line length: 35.0 characters\n- Contains IP addresses\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result strings.Builder
			AnalyzeGenericTextFile(tt.content, false, &result)
			if result.String() != tt.expected {
				t.Errorf("AnalyzeGenericTextFile() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}
