package formatter

import (
	"path/filepath"
	"time"
)

// Global formatter registry
var formatters = map[OutputFormat]OutputFormatter{
	FormatText:     &TextFormatter{},
	FormatJSON:     &JSONFormatter{},
	FormatCSV:      &CSVFormatter{},
	FormatHTML:     &HTMLFormatter{},
	FormatMarkdown: &MarkdownFormatter{},
}

// GetFormatter returns the formatter for the specified format
func GetFormatter(format OutputFormat) OutputFormatter {
	if formatter, exists := formatters[format]; exists {
		return formatter
	}
	// Default to text
	return formatters[FormatText]
}

// CreateAnalysisResult creates a new analysis result
func CreateAnalysisResult(toolName, filePath string, data interface{}, err error) *AnalysisResult {

	result := &AnalysisResult{
		ToolName:  toolName,
		FilePath:  filePath,
		Package:   filepath.Base(filePath),
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      data,
		Metadata:  make(map[string]interface{}),
	}

	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
	} else {
		result.Status = "success"
	}

	return result
}

// FormatAnalysisResult formats an analysis result using the specified format
func FormatAnalysisResult(result *AnalysisResult, format OutputFormat) string {
	formatter := GetFormatter(format)
	return formatter.Format(result)
}
