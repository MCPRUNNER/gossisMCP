package formatter

import (
	"fmt"
	"strings"
)

// CSVFormatter formats analysis results as CSV
type CSVFormatter struct{}

func (f *CSVFormatter) Format(result *AnalysisResult) string {
	if result.Error != "" {
		return fmt.Sprintf("Error,%s\n", f.escapeCSV(result.Error))
	}

	var output strings.Builder

	// Handle different data types
	switch v := result.Data.(type) {
	case *TableData:
		return f.formatTableCSV(v)
	case []SectionData:
		return f.formatSectionsCSV(v)
	default:
		// Fallback to simple format
		output.WriteString("Tool,File,Timestamp,Status,Data,Error\n")
		output.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			f.escapeCSV(result.ToolName),
			f.escapeCSV(result.FilePath),
			f.escapeCSV(result.Timestamp),
			f.escapeCSV(result.Status),
			f.escapeCSV(fmt.Sprintf("%v", result.Data)),
			f.escapeCSV(result.Error)))
		return output.String()
	}
}

func (f *CSVFormatter) formatTableCSV(table *TableData) string {
	var output strings.Builder

	// Headers
	for i, header := range table.Headers {
		output.WriteString(f.escapeCSV(header))
		if i < len(table.Headers)-1 {
			output.WriteString(",")
		}
	}
	output.WriteString("\n")

	// Rows
	for _, row := range table.Rows {
		for i, cell := range row {
			output.WriteString(f.escapeCSV(cell))
			if i < len(row)-1 {
				output.WriteString(",")
			}
		}
		output.WriteString("\n")
	}

	return output.String()
}

func (f *CSVFormatter) formatSectionsCSV(sections []SectionData) string {
	var output strings.Builder
	output.WriteString("Section,Content\n")

	for _, section := range sections {
		f.formatSectionCSV(&output, section, "")
	}

	return output.String()
}

func (f *CSVFormatter) formatSectionCSV(output *strings.Builder, section SectionData, prefix string) {
	sectionPath := section.Title
	if prefix != "" {
		sectionPath = prefix + " > " + section.Title
	}

	switch v := section.Content.(type) {
	case string:
		output.WriteString(fmt.Sprintf("%s,%s\n", f.escapeCSV(sectionPath), f.escapeCSV(v)))
	case []string:
		for _, s := range v {
			output.WriteString(fmt.Sprintf("%s,%s\n", f.escapeCSV(sectionPath), f.escapeCSV(s)))
		}
	case map[string]interface{}:
		for key, value := range v {
			output.WriteString(fmt.Sprintf("%s,%s: %v\n", f.escapeCSV(sectionPath), f.escapeCSV(key), value))
		}
	}

	for _, subsection := range section.Subsections {
		f.formatSectionCSV(output, subsection, sectionPath)
	}
}

func (f *CSVFormatter) escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		s = strings.ReplaceAll(s, "\"", "\"\"")
		s = "\"" + s + "\""
	}
	return s
}

func (f *CSVFormatter) GetContentType() string {
	return "text/csv"
}