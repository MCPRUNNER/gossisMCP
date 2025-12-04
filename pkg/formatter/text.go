package formatter

import (
	"fmt"
	"strings"
)

// TextFormatter formats analysis results as plain text
type TextFormatter struct{}

func (f *TextFormatter) Format(result *AnalysisResult) string {
	if result.Error != "" {
		return fmt.Sprintf("Error: %s", result.Error)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("%s Analysis Report\n", result.ToolName))
	output.WriteString(fmt.Sprintf("File: %s\n", result.FilePath))
	output.WriteString(fmt.Sprintf("Generated: %s\n\n", result.Timestamp))

	f.formatData(&output, result.Data, 0)
	return output.String()
}

func (f *TextFormatter) formatData(output *strings.Builder, data interface{}, level int) {
	indent := strings.Repeat("  ", level)

	switch v := data.(type) {
	case string:
		output.WriteString(fmt.Sprintf("%s%s\n", indent, v))
	case []string:
		for _, s := range v {
			output.WriteString(fmt.Sprintf("%s%s\n", indent, s))
		}
	case map[string]interface{}:
		for key, value := range v {
			output.WriteString(fmt.Sprintf("%s%s: ", indent, key))
			f.formatData(output, value, 0) // Don't indent values
		}
	case []interface{}:
		for _, item := range v {
			f.formatData(output, item, level)
		}
	case *TableData:
		f.formatTable(output, v, level)
	case []SectionData:
		f.formatSections(output, v, level)
	default:
		output.WriteString(fmt.Sprintf("%s%v\n", indent, v))
	}
}

func (f *TextFormatter) formatTable(output *strings.Builder, table *TableData, level int) {
	if len(table.Rows) == 0 {
		output.WriteString("No data available.\n")
		return
	}

	// Calculate column widths
	colWidths := make([]int, len(table.Headers))
	for i, header := range table.Headers {
		colWidths[i] = len(header)
	}
	for _, row := range table.Rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	indent := strings.Repeat("  ", level)

	// Header
	output.WriteString(indent)
	for i, header := range table.Headers {
		output.WriteString(fmt.Sprintf("%-*s", colWidths[i], header))
		if i < len(table.Headers)-1 {
			output.WriteString(" | ")
		}
	}
	output.WriteString("\n")

	// Separator
	output.WriteString(indent)
	for i, width := range colWidths {
		output.WriteString(strings.Repeat("-", width))
		if i < len(colWidths)-1 {
			output.WriteString("-+-")
		}
	}
	output.WriteString("\n")

	// Rows
	for _, row := range table.Rows {
		output.WriteString(indent)
		for i, cell := range row {
			if i < len(colWidths) {
				output.WriteString(fmt.Sprintf("%-*s", colWidths[i], cell))
				if i < len(row)-1 {
					output.WriteString(" | ")
				}
			}
		}
		output.WriteString("\n")
	}
}

func (f *TextFormatter) formatSections(output *strings.Builder, sections []SectionData, level int) {
	for _, section := range sections {
		f.formatSection(output, &section, level)
	}
}

func (f *TextFormatter) formatSection(output *strings.Builder, section *SectionData, level int) {
	indent := strings.Repeat("  ", level)
	output.WriteString(fmt.Sprintf("%s%s\n", indent, section.Title))
	output.WriteString(fmt.Sprintf("%s%s\n", indent, strings.Repeat("=", len(section.Title))))

	f.formatData(output, section.Content, level)
	output.WriteString("\n")

	for _, subsection := range section.Subsections {
		f.formatSection(output, &subsection, level+1)
	}
}

func (f *TextFormatter) GetContentType() string {
	return "text/plain"
}
