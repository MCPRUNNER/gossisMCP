# SSIS Analyzer Workflows Guide

## Overview

The SSIS Analyzer includes a powerful workflow engine that allows you to chain multiple analysis tools together, process batches of packages, and automatically generate reports. Workflows are defined in JSON format and can include loops, data transformations, and template rendering.

## Key Features

### 1. **Sequential Step Execution**

Workflows execute steps in order, with each step able to reference outputs from previous steps using placeholder syntax.

### 2. **Loop Processing**

Steps can iterate over arrays (e.g., lists of DTSX files), executing the same analysis tool for each item and aggregating results.

### 3. **Automatic Data Consolidation**

When a step with `output_file_path` completes, the workflow engine automatically:

- Aggregates all loop iteration results into a single JSON file
- Normalizes tool-specific output fields into a consistent structure
- Adds computed fields like `package` name for easier template rendering

### 4. **Template Rendering**

Workflows can render HTML reports using Go templates, consuming the aggregated JSON data from analysis steps.

### 5. **Inter-Step Dependencies**

Use placeholder syntax to pass data between steps, creating complex analysis pipelines.

## Workflow Structure

### Basic Workflow JSON

```json
{
  "Steps": [
    {
      "Name": "StepName",
      "Type": "#tool_name",
      "Parameters": {
        "param1": "value1",
        "param2": "{PreviousStep.Output.field}"
      },
      "Enabled": true,
      "Output": {
        "Name": "OutputName",
        "Format": "json"
      }
    }
  ]
}
```

### Step Properties

| Property           | Type    | Required | Description                                             |
| ------------------ | ------- | -------- | ------------------------------------------------------- |
| `Name`             | string  | Yes      | Unique identifier for the step                          |
| `Type`             | string  | Yes      | MCP tool name (prefixed with `#`)                       |
| `Parameters`       | object  | Yes      | Tool-specific parameters                                |
| `Enabled`          | boolean | Yes      | Whether to execute this step                            |
| `Output`           | object  | No       | Defines the output capture name and format              |
| `loop`             | object  | No       | Configuration for iterating over arrays                 |
| `output_file_path` | string  | No       | Path to write aggregated results (relative to workflow) |

### Loop Configuration

```json
"loop": {
    "input_data": "{PreviousStep.Output.arrayField}",
    "item_name": "item"
}
```

- **input_data**: Placeholder referencing an array from a previous step
- **item_name**: Variable name used in parameters to reference the current item (e.g., `{item}`)

### Output Configuration

```json
"Output": {
    "Name": "Content",
    "Format": "json"
}
```

- **Name**: The key under which this step's output will be stored
- **Format**: Output format (`json`, `text`, `html`, `markdown`, `csv`)

## Placeholder Syntax

### Basic Placeholder

Reference outputs from previous steps:

```
{StepName.OutputName.field}
```

Example:

```json
"file_path": "{GetPackages.Content.packages_absolute}"
```

### Nested Field Access

Access nested JSON fields:

```
{StepName.OutputName.parent.child}
```

### Loop Item Reference

Within loop parameters, reference the current item:

```json
"Parameters": {
    "file_path": "{pkg}"
}
```

## Data Consolidation Process

When a step has `output_file_path` set, the workflow engine performs these operations **immediately after the step completes**:

1. **Aggregation**: Combines all loop iteration outputs into a single structure
2. **Normalization**: Converts tool-specific fields to a standard format:
   - `analysis` → `results`
   - `data` → `results`
   - `message` → `results`
3. **Package Field Injection**: Adds a `package` field containing the base filename
4. **JSON Writing**: Writes the consolidated data to the specified file with a `data` array wrapper

### Example Consolidated JSON

```json
{
  "data": [
    {
      "file_path": "C:\\path\\to\\Package1.dtsx",
      "package": "Package1",
      "tool_name": "analyze_data_flow",
      "timestamp": "2025-12-05T12:00:00-05:00",
      "status": "success",
      "results": "Data Flow Analysis:\n..."
    },
    {
      "file_path": "C:\\path\\to\\Package2.dtsx",
      "package": "Package2",
      "tool_name": "analyze_data_flow",
      "timestamp": "2025-12-05T12:00:01-05:00",
      "status": "success",
      "results": "Data Flow Analysis:\n..."
    }
  ]
}
```

## Included Workflows

### 1. workflow_dataflow_analysis.json

**Purpose**: Analyze data flows in all SSIS packages and generate an HTML report

**Steps**:

1. `GetMyPackages` - List all DTSX files in a directory
2. `AnalyzeDataFlow` - Loop through packages, analyze data flows (writes `dataflow_analysis.json`)
3. `AnalyzeLogging` - Loop through packages, analyze logging (writes `logging_analysis.json`)
4. `RenderReport` - Generate HTML report from consolidated JSON

**Output Files**:

- `.gossismcp/Output/dataflow_analysis.json` - Aggregated analysis data
- `.gossismcp/Output/logging_analysis.json` - Aggregated logging data
- `.gossismcp/Output/dataflow_analysis.html` - HTML report

### 2. workflow_logging_analysis.json

**Purpose**: Analyze logging configurations and generate a report

**Steps**:

1. `GetMyPackages` - List all DTSX files
2. `AnalyzeLogging` - Loop through packages, analyze logging (writes `logging_analysis.json`)
3. `RenderReport` - Generate HTML report

**Output Files**:

- `.gossismcp/Output/logging_analysis.json` - Aggregated logging analysis
- `.gossismcp/Output/logging_analysis.html` - HTML report

### 3. workflow_combined_reports.json

**Purpose**: Run multiple analyses and generate separate HTML reports

**Steps**:

1. `GetMyPackages` - List all DTSX files
2. `AnalyzeDataFlow` - Analyze data flows (writes `dataflow_analysis.json`)
3. `AnalyzeLogging` - Analyze logging (writes `logging_analysis.json`)
4. `RenderDataflowReport` - Generate data flow HTML report
5. `RenderLoggingReport` - Generate logging HTML report

**Output Files**:

- `.gossismcp/Output/dataflow_analysis.json`
- `.gossismcp/Output/dataflow_analysis.html`
- `.gossismcp/Output/logging_analysis.json`
- `.gossismcp/Output/logging_analysis.html`

### 4. workflow_example.json

**Purpose**: Demonstrate basic workflow structure with batch analysis

**Steps**:

1. `GetMyPackages` - List DTSX files
2. `Batch_Analyze` - Run parallel analysis on all packages
3. `RenderReport` - Generate HTML report

## Running Workflows

### Using the MCP Tool

```json
{
  "tool": "workflow_runner",
  "arguments": {
    "file_path": "C:\\path\\to\\workflow.json",
    "format": "markdown"
  }
}
```

### Using the Standalone Tool

```powershell
# Run a workflow
ssis-analyzer.exe workflow_runner --file_path .gossismcp/workflows/workflow_combined_reports.json

# With output format
ssis-analyzer.exe workflow_runner --file_path .gossismcp/workflows/workflow_combined_reports.json --format markdown
```

### From VS Code MCP

```
#mcp_ssis-analyzer_workflow_runner C:\path\to\workflow.json
```

## Creating Custom Workflows

### Example: Custom Analysis Workflow

```json
{
  "Steps": [
    {
      "Name": "ListPackages",
      "Type": "#list_packages",
      "Parameters": {
        "directory": "C:\\MySSISPackages",
        "format": "json"
      },
      "Enabled": true,
      "Output": {
        "Name": "Content",
        "Format": "json"
      }
    },
    {
      "Name": "AnalyzeConnections",
      "Type": "#analyze_connections",
      "loop": {
        "input_data": "{ListPackages.Content.packages_absolute}",
        "item_name": "pkg"
      },
      "Parameters": {
        "file_path": "{pkg}",
        "format": "json"
      },
      "Enabled": true,
      "Output": {
        "Name": "Message",
        "Format": "json"
      },
      "output_file_path": "../Output/connections.json"
    },
    {
      "Name": "GenerateReport",
      "Type": "#render_template",
      "Parameters": {
        "template_file_path": "../templates/connections_report.gotmpl",
        "json_file_path": "../Output/connections.json",
        "output_file_path": "../Output/connections.html"
      },
      "Enabled": true,
      "Output": {
        "Name": "Message",
        "Format": "text"
      }
    }
  ]
}
```

## Template Development

### Accessing Consolidated Data

Templates receive the consolidated JSON with a `data` array:

```html
<!DOCTYPE html>
<html>
  <head>
    <title>Analysis Report</title>
  </head>
  <body>
    <h1>SSIS Package Analysis</h1>
    {{ $rows := index . "data" }} {{ if gt (len $rows) 0 }}
    <table>
      <thead>
        <tr>
          <th>Package</th>
          <th>Status</th>
          <th>Results</th>
        </tr>
      </thead>
      <tbody>
        {{ range $rows }}
        <tr>
          <td>{{ index . "package" }}</td>
          <td>{{ index . "status" }}</td>
          <td><pre>{{ index . "results" }}</pre></td>
        </tr>
        {{ end }}
      </tbody>
    </table>
    {{ else }}
    <p>No data available.</p>
    {{ end }}
  </body>
</html>
```

### Standard Fields in Consolidated Data

After normalization, each item in the `data` array contains:

- **file_path**: Full path to the DTSX file
- **package**: Package name (basename without extension)
- **tool_name**: Name of the analysis tool used
- **timestamp**: ISO 8601 timestamp of analysis
- **status**: "success" or "error"
- **results**: Normalized analysis output (replaces tool-specific fields)

## Workflow Execution Flow

1. **Validation**: Workflow structure is validated before execution
2. **Sequential Processing**: Each enabled step executes in order
3. **Parameter Resolution**: Placeholders are resolved from previous step outputs
4. **Loop Execution** (if configured):
   - Resolve the array from `input_data`
   - For each item, resolve parameters and invoke the tool
   - Aggregate all outputs
5. **Immediate Consolidation**: If `output_file_path` is set, write consolidated JSON
6. **Next Step**: Subsequent steps can reference the consolidated data

## Best Practices

### 1. **Use Descriptive Step Names**

Choose clear, meaningful names that indicate what each step does:

```json
"Name": "AnalyzeDataFlow"  // Good
"Name": "Step2"            // Bad
```

### 2. **Set output_file_path for Loop Steps**

When looping over multiple packages, always set `output_file_path` to ensure data is consolidated before subsequent steps:

```json
{
    "Name": "AnalyzeAllPackages",
    "loop": { ... },
    "output_file_path": "../Output/analysis.json"
}
```

### 3. **Use Relative Paths**

Paths in workflows should be relative to the workflow file location:

```json
"template_file_path": "../templates/report.gotmpl",
"output_file_path": "../Output/report.html"
```

### 4. **Enable/Disable Steps for Testing**

Use the `Enabled` flag to temporarily skip steps during development:

```json
{
    "Name": "ExpensiveAnalysis",
    "Enabled": false,
    ...
}
```

### 5. **Consistent Output Formats**

Use consistent output formats across your workflow:

```json
"Output": {
    "Name": "Content",
    "Format": "json"  // json, text, html, markdown, or csv
}
```

### 6. **Validate JSON Syntax**

Always validate your workflow JSON before running:

```powershell
Get-Content workflow.json | ConvertFrom-Json
```

## Troubleshooting

### Common Issues

**Issue**: Template shows only one row instead of all packages
**Solution**: Ensure the loop step has `output_file_path` set to trigger data consolidation

**Issue**: Placeholder not resolved
**Solution**: Check step names and output names match exactly (case-sensitive)

**Issue**: Loop not executing
**Solution**: Verify `input_data` references an array and `item_name` is used in parameters

**Issue**: File not found errors
**Solution**: Use relative paths from the workflow file location, not absolute paths

### Debugging Tips

1. **Check Workflow Output**: The workflow runner provides detailed execution logs
2. **Inspect Intermediate JSON**: Examine the consolidated JSON files in the Output directory
3. **Test Steps Individually**: Disable later steps to test earlier ones in isolation
4. **Validate Template Syntax**: Test templates with sample JSON data first

## Advanced Features

### Conditional Step Execution

While not directly supported, you can disable steps manually:

```json
"Enabled": false
```

### Multiple Loops in Sequence

Chain multiple loop steps, each consolidating its data:

```json
{
    "Name": "AnalyzeDataFlow",
    "loop": { ... },
    "output_file_path": "../Output/dataflow.json"
},
{
    "Name": "AnalyzeLogging",
    "loop": { ... },
    "output_file_path": "../Output/logging.json"
}
```

### Nested Field Access

Access deeply nested JSON structures:

```json
"input_data": "{GetPackages.Content.metadata.packages.production}"
```

## Integration with MCP

Workflows are fully integrated with the Model Context Protocol, allowing:

- Execution via MCP tools
- Integration with VS Code and other MCP clients
- Automated workflow triggering from AI assistants

## Performance Considerations

- **Parallel Execution**: Loop iterations execute sequentially; for parallel processing, use `#batch_analyze`
- **Memory Usage**: Large package sets may require substantial memory for aggregation
- **File I/O**: Consolidated JSON is written to disk after each step with `output_file_path`

## Future Enhancements

Potential future workflow features:

- Conditional step execution based on previous results
- Parallel loop processing
- Workflow composition (calling workflows from workflows)
- Dynamic parameter generation
- Error handling and retry logic

## Support and Resources

- **Templates Directory**: `.gossismcp/templates/`
- **Workflows Directory**: `.gossismcp/workflows/`
- **Output Directory**: `.gossismcp/Output/`
- **Main Documentation**: `README.md`
- **MCP Documentation**: `Documents/mcp.json`

## Examples

See the `.gossismcp/workflows/` directory for complete working examples of:

- Data flow analysis workflows
- Logging analysis workflows
- Combined multi-report workflows
- Batch processing workflows
