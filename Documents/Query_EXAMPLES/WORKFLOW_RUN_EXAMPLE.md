# Workflow Execution Example Run

## Copilot Query

```copilot
#workflow_runner .gossismcp\workflows\workflow_merged_example.json
```

## Results

1. [logging_analysis.html](../reports/logging_analysis.html)
1. [best_practices_validation.html](../reports/best_practices_validation.html)
1. [dataflow_analysis_detailed.html](../reports/dataflow_analysis_detailed.html)
1. [merged_analysis.html](../reports/merged_analysis.html)
1. [merged_analysis.tmpl](../reports/merged_analysis.tmpl)

## Workflow Definition

```json
{
  "Steps": [
    {
      "Name": "GetMyPackages",
      "Type": "#list_packages",
      "Parameters": {
        "directory": "Documents\\SSIS_EXAMPLES",
        "format": "json"
      },
      "Enabled": true,
      "Output": {
        "Name": "Content",
        "Format": "JSON"
      }
    },
    {
      "Name": "AnalyzeDataFlow",
      "Type": "#analyze_data_flow",
      "loop": {
        "input_data": "{GetMyPackages.Content.packages_absolute}",
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
      "output_file_path": "../reports/dataflow_analysis.json"
    },
    {
      "Name": "AnalyzeLogging",
      "Type": "#analyze_logging_configuration",
      "loop": {
        "input_data": "{GetMyPackages.Content.packages_absolute}",
        "item_name": "pkg"
      },
      "Parameters": {
        "file_path": "{pkg}",
        "format": "json"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "json" },
      "output_file_path": "../reports/logging_analysis.json"
    },
    {
      "Name": "ValidateBestPractices",
      "Type": "#validate_best_practices",
      "loop": {
        "input_data": "{GetMyPackages.Content.packages_absolute}",
        "item_name": "pkg"
      },
      "Parameters": {
        "file_path": "{pkg}",
        "format": "json"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "json" },
      "output_file_path": "../reports/best_practices_validation.json"
    },
    {
      "Name": "AnalyzeDataFlowDetailed",
      "Type": "#analyze_data_flow_detailed",
      "loop": {
        "input_data": "{GetMyPackages.Content.packages_absolute}",
        "item_name": "pkg"
      },
      "Parameters": {
        "file_path": "{pkg}",
        "format": "json"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "json" },
      "output_file_path": "../reports/dataflow_analysis_detailed.json"
    },
    {
      "Name": "RenderDataflowReport",
      "Type": "#render_template",
      "Parameters": {
        "template_file_path": "../templates/dataflow_analysis.tmpl",
        "json_file_path": "../reports/dataflow_analysis.json",
        "output_file_path": "../reports/dataflow_analysis.html"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "text" }
    },
    {
      "Name": "RenderLoggingReport",
      "Type": "#render_template",
      "Parameters": {
        "template_file_path": "../templates/logging_analysis.tmpl",
        "json_file_path": "../reports/logging_analysis.json",
        "output_file_path": "../reports/logging_analysis.html"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "text" }
    },
    {
      "Name": "RenderBestPracticesReport",
      "Type": "#render_template",
      "Parameters": {
        "template_file_path": "../templates/best_practices_validation.tmpl",
        "json_file_path": "../reports/best_practices_validation.json",
        "output_file_path": "../reports/best_practices_validation.html"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "text" }
    },
    {
      "Name": "RenderDetailedDataflowReport",
      "Type": "#render_template",
      "Parameters": {
        "template_file_path": "../templates/dataflow_analysis_detailed.tmpl",
        "json_file_path": "../reports/dataflow_analysis_detailed.json",
        "output_file_path": "../reports/dataflow_analysis_detailed.html"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "text" }
    },
    {
      "Name": "MergeReports",
      "Type": "#merge_json",
      "Enabled": true,
      "Parameters": {
        "file_paths": [
          "../reports/dataflow_analysis.json",
          "../reports/logging_analysis.json",
          "../reports/best_practices_validation.json",
          "../reports/dataflow_analysis_detailed.json"
        ],
        "output_file_path": "../reports/merged_analysis.json",
        "format": "json"
      }
    },
    {
      "Name": "MergedRenderDetailedDataflowReport",
      "Type": "#render_template",
      "Parameters": {
        "template_file_path": "../templates/merged_analysis.tmpl",
        "json_file_path": "../reports/merged_analysis.json",
        "output_file_path": "../reports/merged_analysis.html"
      },
      "Enabled": true,
      "Output": { "Name": "Message", "Format": "text" }
    }
  ]
}
```
