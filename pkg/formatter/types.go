package formatter

// OutputFormat represents the supported output formats
type OutputFormat string

const (
	FormatText     OutputFormat = "text"
	FormatJSON     OutputFormat = "json"
	FormatCSV      OutputFormat = "csv"
	FormatHTML     OutputFormat = "html"
	FormatMarkdown OutputFormat = "markdown"
)

// AnalysisResult represents the result of an analysis operation
type AnalysisResult struct {
	ToolName  string                 `json:"tool_name"`
	FilePath  string                 `json:"file_path"`
	Package  string                 `json:"package"`
	Timestamp string                 `json:"timestamp"`
	Status    string                 `json:"status"`
	Data      interface{}            `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// TableData represents tabular data for formatting
type TableData struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// SectionData represents structured content with sections and subsections
type SectionData struct {
	Title       string        `json:"title"`
	Content     interface{}   `json:"content"`
	Level       int           `json:"level,omitempty"`
	Subsections []SectionData `json:"subsections,omitempty"`
}

// OutputFormatter defines the interface for formatting analysis results
type OutputFormatter interface {
	Format(result *AnalysisResult) string
	GetContentType() string
}
