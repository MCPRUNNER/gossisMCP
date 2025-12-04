package formatter

import (
	"fmt"
	"html"
	"strings"
)

// HTMLFormatter formats analysis results as HTML
type HTMLFormatter struct{}

func (f *HTMLFormatter) Format(result *AnalysisResult) string {
	var output strings.Builder

	output.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	output.WriteString("<title>SSIS Analysis Report</title>\n")
	output.WriteString("<style>\n")
	output.WriteString("body { font-family: Arial, sans-serif; margin: 20px; }\n")
	output.WriteString("h1 { color: #333; }\n")
	output.WriteString("h2 { color: #666; margin-top: 30px; }\n")
	output.WriteString("table { border-collapse: collapse; width: 100%; margin: 10px 0; }\n")
	output.WriteString("th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }\n")
	output.WriteString("th { background-color: #f2f2f2; }\n")
	output.WriteString("tr:nth-child(even) { background-color: #f9f9f9; }\n")
	output.WriteString(".error { color: red; }\n")
	output.WriteString(".success { color: green; }\n")
	output.WriteString("</style>\n")
	output.WriteString("</head>\n<body>\n")

	if result.Error != "" {
		output.WriteString(fmt.Sprintf("<h1 class=\"error\">Error</h1>\n<p>%s</p>\n", html.EscapeString(result.Error)))
	} else {
		output.WriteString(fmt.Sprintf("<h1>%s Analysis Report</h1>\n", html.EscapeString(result.ToolName)))
		output.WriteString(fmt.Sprintf("<p><strong>File:</strong> %s</p>\n", html.EscapeString(result.FilePath)))
		output.WriteString(fmt.Sprintf("<p><strong>Generated:</strong> %s</p>\n", html.EscapeString(result.Timestamp)))

		f.formatDataHTML(&output, result.Data, 0)
	}

	output.WriteString("</body>\n</html>\n")
	return output.String()
}

func (f *HTMLFormatter) formatDataHTML(output *strings.Builder, data interface{}, level int) {
	switch v := data.(type) {
	case string:
		output.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(v)))
	case []string:
		output.WriteString("<ul>\n")
		for _, s := range v {
			output.WriteString(fmt.Sprintf("<li>%s</li>\n", html.EscapeString(s)))
		}
		output.WriteString("</ul>\n")
	case map[string]interface{}:
		output.WriteString("<dl>\n")
		for key, value := range v {
			output.WriteString(fmt.Sprintf("<dt>%s</dt>\n", html.EscapeString(key)))
			output.WriteString("<dd>")
			f.formatDataHTML(output, value, level)
			output.WriteString("</dd>\n")
		}
		output.WriteString("</dl>\n")
	case []interface{}:
		for _, item := range v {
			f.formatDataHTML(output, item, level)
		}
	case *TableData:
		f.formatTableHTML(output, v)
	case []SectionData:
		f.formatSectionsHTML(output, v, level)
	default:
		output.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(fmt.Sprintf("%v", v))))
	}
}

func (f *HTMLFormatter) formatTableHTML(output *strings.Builder, table *TableData) {
	output.WriteString("<table>\n<thead>\n<tr>\n")
	for _, header := range table.Headers {
		output.WriteString(fmt.Sprintf("<th>%s</th>\n", html.EscapeString(header)))
	}
	output.WriteString("</tr>\n</thead>\n<tbody>\n")

	for _, row := range table.Rows {
		output.WriteString("<tr>\n")
		for _, cell := range row {
			output.WriteString(fmt.Sprintf("<td>%s</td>\n", html.EscapeString(cell)))
		}
		output.WriteString("</tr>\n")
	}

	output.WriteString("</tbody>\n</table>\n")
}

func (f *HTMLFormatter) formatSectionsHTML(output *strings.Builder, sections []SectionData, level int) {
	for _, section := range sections {
		f.formatSectionHTML(output, &section, level)
	}
}

func (f *HTMLFormatter) formatSectionHTML(output *strings.Builder, section *SectionData, level int) {
	tag := fmt.Sprintf("h%d", level+2)
	output.WriteString(fmt.Sprintf("<%s>%s</%s>\n", tag, html.EscapeString(section.Title), tag))

	f.formatDataHTML(output, section.Content, level)

	for _, subsection := range section.Subsections {
		f.formatSectionHTML(output, &subsection, level+1)
	}
}

func (f *HTMLFormatter) GetContentType() string {
	return "text/html"
}
