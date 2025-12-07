package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFilePath(t *testing.T) {
	tests := []struct {
		name             string
		filePath         string
		packageDirectory string
		expected         string
	}{
		{
			name:             "absolute path",
			filePath:         "C:\\absolute\\path\\file.txt",
			packageDirectory: "/pkg/dir",
			expected:         "C:\\absolute\\path\\file.txt",
		},
		{
			name:             "relative path with package directory",
			filePath:         "relative/file.txt",
			packageDirectory: "/pkg/dir",
			expected:         filepath.Join("/pkg/dir", "relative/file.txt"),
		},
		{
			name:             "relative path without package directory",
			filePath:         "relative/file.txt",
			packageDirectory: "",
			expected:         "relative/file.txt",
		},
		{
			name:             "empty file path",
			filePath:         "",
			packageDirectory: "/pkg/dir",
			expected:         `\pkg\dir`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveFilePath(tt.filePath, tt.packageDirectory)
			if result != tt.expected {
				t.Errorf("ResolveFilePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsFileBinary(t *testing.T) {
	// Create temporary files for testing
	tempDir := t.TempDir()

	// Create a text file
	textFile := filepath.Join(tempDir, "text.txt")
	err := os.WriteFile(textFile, []byte("This is a text file\nwith multiple lines"), 0644)
	if err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}

	// Create a binary file (with null bytes)
	binaryFile := filepath.Join(tempDir, "binary.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x00, 0x03, 0x04}
	err = os.WriteFile(binaryFile, binaryContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	tests := []struct {
		name     string
		filePath string
		expected bool
		hasError bool
	}{
		{
			name:     "text file",
			filePath: textFile,
			expected: false,
			hasError: false,
		},
		{
			name:     "binary file",
			filePath: binaryFile,
			expected: true,
			hasError: false,
		},
		{
			name:     "nonexistent file",
			filePath: filepath.Join(tempDir, "nonexistent.txt"),
			expected: false,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsFileBinary(tt.filePath)
			if (err != nil) != tt.hasError {
				t.Errorf("IsFileBinary() error = %v, hasError %v", err, tt.hasError)
				return
			}
			if result != tt.expected {
				t.Errorf("IsFileBinary() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertToLines(t *testing.T) {
	tests := []struct {
		name               string
		content            string
		isLineNumberNeeded bool
		expected           []string
	}{
		{
			name:               "without line numbers",
			content:            "line 1\nline 2\nline 3",
			isLineNumberNeeded: false,
			expected:           []string{" line 1", " line 2", " line 3"},
		},
		{
			name:               "with line numbers",
			content:            "line 1\nline 2\nline 3",
			isLineNumberNeeded: true,
			expected:           []string{"   1: line 1", "   2: line 2", "   3: line 3"},
		},
		{
			name:               "empty content",
			content:            "",
			isLineNumberNeeded: false,
			expected:           []string{" "},
		},
		{
			name:               "single line",
			content:            "single line",
			isLineNumberNeeded: true,
			expected:           []string{"   1: single line"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToLines(tt.content, tt.isLineNumberNeeded)
			if len(result) != len(tt.expected) {
				t.Errorf("ConvertToLines() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("ConvertToLines()[%d] = %v, want %v", i, line, tt.expected[i])
				}
			}
		})
	}
}
