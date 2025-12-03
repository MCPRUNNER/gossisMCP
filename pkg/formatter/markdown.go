package formatter

import (
	"fmt"
	"strings"
)

// MarkdownFormatter formats analysis results as Markdown
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(result *AnalysisResult) string {
	var output strings.Builder

	if result.Error != "" {
		output.WriteString(fmt.Sprintf("# Error\n\n%s\n", result.Error))
		return output.String()
	}

	output.WriteString(fmt.Sprintf("# %s Analysis Report\n\n", result.ToolName))
	output.WriteString(fmt.Sprintf("**File:** %s\n\n", result.FilePath))
	output.WriteString(fmt.Sprintf("**Generated:** %s\n\n", result.Timestamp))

	f.formatDataMarkdown(&output, result.Data, 0)
	return output.String()
}

func (f *MarkdownFormatter) formatDataMarkdown(output *strings.Builder, data interface{}, level int) {
	switch v := data.(type) {
	case string:
		output.WriteString(fmt.Sprintf("%s\n\n", v))
	case []string:
		for _, s := range v {
			output.WriteString(fmt.Sprintf("- %s\n", s))
		}
		output.WriteString("\n")
	case map[string]interface{}:
		for key, value := range v {
			output.WriteString(fmt.Sprintf("**%s:** ", key))
			f.formatDataMarkdown(output, value, level)
		}
	case []interface{}:
		for _, item := range v {
			f.formatDataMarkdown(output, item, level)
		}
	case *TableData:
		f.formatTableMarkdown(output, v)
	case []SectionData:
		f.formatSectionsMarkdown(output, v, level)
	default:
		output.WriteString(fmt.Sprintf("%v\n\n", v))
	}
}

func (f *MarkdownFormatter) formatTableMarkdown(output *strings.Builder, table *TableData) {
	if len(table.Rows) == 0 {
		output.WriteString("No data available.\n\n")
		return
	}

	// Headers
	output.WriteString("| ")
	for i, header := range table.Headers {
		output.WriteString(f.escapeMarkdown(header))
		if i < len(table.Headers)-1 {
			output.WriteString(" | ")
		}
	}
	output.WriteString(" |\n")

	// Separator
	output.WriteString("| ")
	for i := range table.Headers {
		output.WriteString("---")
		if i < len(table.Headers)-1 {
			output.WriteString(" | ")
		}
	}
	output.WriteString(" |\n")

	// Rows
	for _, row := range table.Rows {
		output.WriteString("| ")
		for i, cell := range row {
			output.WriteString(f.escapeMarkdown(cell))
			if i < len(row)-1 {
				output.WriteString(" | ")
			}
		}
		output.WriteString(" |\n")
	}
	output.WriteString("\n")
}

func (f *MarkdownFormatter) formatSectionsMarkdown(output *strings.Builder, sections []SectionData, level int) {
	for _, section := range sections {
		f.formatSectionMarkdown(output, &section, level)
	}
}

func (f *MarkdownFormatter) formatSectionMarkdown(output *strings.Builder, section *SectionData, level int) {
	headerPrefix := strings.Repeat("#", level+2)
	output.WriteString(fmt.Sprintf("%s %s\n\n", headerPrefix, section.Title))

	f.formatDataMarkdown(output, section.Content, level)

	for _, subsection := range section.Subsections {
		f.formatSectionMarkdown(output, &subsection, level+1)
	}
}

func (f *MarkdownFormatter) escapeMarkdown(s string) string {
	// Escape pipe characters in table cells
	return strings.ReplaceAll(s, "|", "\\|")
}

func (f *MarkdownFormatter) GetContentType() string {
	return "text/markdown"
}