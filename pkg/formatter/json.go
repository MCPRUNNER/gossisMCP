package formatter

import (
	"encoding/json"
	"fmt"
)

// JSONFormatter formats analysis results as JSON
type JSONFormatter struct{}

func (f *JSONFormatter) Format(result *AnalysisResult) string {
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to format result: %s"}`, err.Error())
	}
	return string(jsonBytes)
}

func (f *JSONFormatter) GetContentType() string {
	return "application/json"
}
