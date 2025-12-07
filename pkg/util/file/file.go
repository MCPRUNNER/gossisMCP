package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveFilePath resolves a file path against the package directory if it's relative
func ResolveFilePath(filePath, packageDirectory string) string {
	if packageDirectory == "" || filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(packageDirectory, filePath)
}

// IsFileBinary detects if a file is binary by checking for null bytes in the first 512 bytes
func IsFileBinary(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return false, err
	}

	// Check for null bytes
	for _, b := range buffer[:n] {
		if b == 0 {
			return true, nil
		}
	}

	return false, nil
}

// ConvertToLines converts content to lines with optional line numbers
func ConvertToLines(content string, isLineNumberNeeded bool) []string {
	lines := strings.Split(content, "\n")
	var rawLines []string
	if len(lines) == 0 {
		return rawLines
	}
	if !isLineNumberNeeded {
		for _, line := range lines {
			rawLines = append(rawLines, fmt.Sprintf(" %s", line))
		}
	} else {
		for i, line := range lines {
			rawLines = append(rawLines, fmt.Sprintf("%4d: %s", i+1, line))
		}
	}
	return rawLines
}
