package analysis

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/MCPRUNNER/gossisMCP/pkg/formatter"
	"github.com/MCPRUNNER/gossisMCP/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

// ResolveFilePath resolves a file path against the package directory if it's relative
func ResolveFilePath(filePath, packageDirectory string) string {
	if packageDirectory == "" || filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(packageDirectory, filePath)
}

// HandleAnalyzeDataFlow handles data flow analysis from DTSX files
func HandleAnalyzeDataFlow(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	// Read the DTSX file as string for analysis
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Data Flow Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	xmlContent := string(data)

	var result strings.Builder
	result.WriteString("Data Flow Analysis:\n\n")

	// Check if this package contains data flow tasks
	if !strings.Contains(xmlContent, "Microsoft.Pipeline") {
		result.WriteString("No Data Flow Tasks found in this package.\n")
		analysisResult := formatter.CreateAnalysisResult("Data Flow Analysis", filePath, result.String(), nil)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	result.WriteString("Data Flow Task found in package.\n\n")

	// Extract component information using regex
	// Find all component definitions
	componentRegex := regexp.MustCompile(`(?s)<component[^>]*>`)
	matches := componentRegex.FindAllString(xmlContent, -1)

	if len(matches) > 0 {
		result.WriteString("Components:\n")
		for _, componentTag := range matches {
			// Extract individual attributes
			nameRegex := regexp.MustCompile(`name="([^"]*)"`)
			classIDRegex := regexp.MustCompile(`componentClassID="([^"]*)"`)
			descRegex := regexp.MustCompile(`description="([^"]*)"`)

			nameMatch := nameRegex.FindStringSubmatch(componentTag)
			classIDMatch := classIDRegex.FindStringSubmatch(componentTag)
			descMatch := descRegex.FindStringSubmatch(componentTag)

			name := ""
			if len(nameMatch) > 1 {
				name = nameMatch[1]
			}

			classID := ""
			if len(classIDMatch) > 1 {
				classID = classIDMatch[1]
			}

			description := ""
			if len(descMatch) > 1 {
				description = descMatch[1]
			}

			componentType := getComponentType(classID)

			result.WriteString(fmt.Sprintf("  - %s (%s)\n", name, componentType))
			if description != "" {
				result.WriteString(fmt.Sprintf("    Description: %s\n", description))
			}

			// Try to find key properties for this component
			// Look for properties within this component's section
			componentStart := strings.Index(xmlContent, fmt.Sprintf(`name="%s"`, name))
			if componentStart > 0 {
				componentEnd := strings.Index(xmlContent[componentStart:], "</component>")
				if componentEnd > 0 {
					componentSection := xmlContent[componentStart : componentStart+componentEnd]

					// Look for key properties
					keyProps := []string{"SqlCommand", "TableOrViewName", "FileName", "ConnectionString"}
					for _, prop := range keyProps {
						propRegex := regexp.MustCompile(fmt.Sprintf(`<property[^>]*name="%s"[^>]*>([^<]*)</property>`, prop))
						propMatch := propRegex.FindStringSubmatch(componentSection)
						if len(propMatch) > 1 && strings.TrimSpace(propMatch[1]) != "" {
							result.WriteString(fmt.Sprintf("    %s: %s\n", prop, strings.TrimSpace(propMatch[1])))
						}
					}
				}
			}
			result.WriteString("\n")
		}
	} else {
		result.WriteString("No components found in data flow.\n")
	}

	// Extract path information
	pathRegex := regexp.MustCompile(`(?s)<path[^>]*>`)
	pathMatches := pathRegex.FindAllString(xmlContent, -1)

	if len(pathMatches) > 0 {
		result.WriteString("Data Paths:\n")
		for _, pathTag := range pathMatches {
			// Extract individual attributes
			refIDRegex := regexp.MustCompile(`refId="[^"]*\.Paths\[([^]]+)\]"`)
			startIDRegex := regexp.MustCompile(`startId="([^"]*)"`)
			endIDRegex := regexp.MustCompile(`endId="([^"]*)"`)

			refIDMatch := refIDRegex.FindStringSubmatch(pathTag)
			startIDMatch := startIDRegex.FindStringSubmatch(pathTag)
			endIDMatch := endIDRegex.FindStringSubmatch(pathTag)

			pathName := ""
			if len(refIDMatch) > 1 {
				pathName = refIDMatch[1]
			}

			startID := ""
			if len(startIDMatch) > 1 {
				// Extract just the component name from the full ID
				startFull := startIDMatch[1]
				if idx := strings.LastIndex(startFull, "\\"); idx > 0 {
					startID = startFull[idx+1:]
				} else {
					startID = startFull
				}
			}

			endID := ""
			if len(endIDMatch) > 1 {
				// Extract just the component name from the full ID
				endFull := endIDMatch[1]
				if idx := strings.LastIndex(endFull, "\\"); idx > 0 {
					endID = endFull[idx+1:]
				} else {
					endID = endFull
				}
			}

			result.WriteString(fmt.Sprintf("  - %s: %s → %s\n", pathName, startID, endID))
		}
		result.WriteString("\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Data Flow Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// getComponentType determines the component type from the class ID
func getComponentType(classID string) string {
	switch {
	case strings.Contains(classID, "Source"):
		return "Source"
	case strings.Contains(classID, "Destination"):
		return "Destination"
	case strings.Contains(classID, "Transformation"):
		return "Transformation"
	default:
		return "Unknown"
	}
}

// HandleAnalyzeDataFlowDetailed handles detailed data flow analysis from DTSX files
func HandleAnalyzeDataFlowDetailed(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Detailed Data Flow Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	xmlContent := string(data)

	var result strings.Builder
	result.WriteString("Detailed Data Flow Analysis:\n\n")

	// Check if this package contains data flow tasks
	if !strings.Contains(xmlContent, "Microsoft.Pipeline") {
		result.WriteString("No Data Flow Tasks found in this package.\n")
		analysisResult := formatter.CreateAnalysisResult("Detailed Data Flow Analysis", filePath, result.String(), nil)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	result.WriteString("Data Flow Task found in package.\n\n")

	// Extract detailed component information using regex
	// Find all component definitions
	componentRegex := regexp.MustCompile(`(?s)<component[^>]*>.*?</component>`)
	matches := componentRegex.FindAllString(xmlContent, -1)

	if len(matches) > 0 {
		result.WriteString("Components:\n")
		for _, componentSection := range matches {
			// Extract component attributes
			nameRegex := regexp.MustCompile(`name="([^"]*)"`)
			classIDRegex := regexp.MustCompile(`componentClassID="([^"]*)"`)
			descRegex := regexp.MustCompile(`description="([^"]*)"`)

			nameMatch := nameRegex.FindStringSubmatch(componentSection)
			classIDMatch := classIDRegex.FindStringSubmatch(componentSection)
			descMatch := descRegex.FindStringSubmatch(componentSection)

			name := ""
			if len(nameMatch) > 1 {
				name = nameMatch[1]
			}

			classID := ""
			if len(classIDMatch) > 1 {
				classID = classIDMatch[1]
			}

			description := ""
			if len(descMatch) > 1 {
				description = descMatch[1]
			}

			result.WriteString(fmt.Sprintf("\nComponent: %s\n", name))
			result.WriteString(fmt.Sprintf("  Type: %s\n", getComponentType(classID)))
			if description != "" {
				result.WriteString(fmt.Sprintf("  Description: %s\n", description))
			}

			// Extract properties
			propRegex := regexp.MustCompile(`<property[^>]*name="([^"]*)"[^>]*>([^<]*)</property>`)
			propMatches := propRegex.FindAllStringSubmatch(componentSection, -1)
			if len(propMatches) > 0 {
				result.WriteString("  Properties:\n")
				for _, propMatch := range propMatches {
					if len(propMatch) > 2 {
						result.WriteString(fmt.Sprintf("    %s: %s\n", propMatch[1], propMatch[2]))
					}
				}
			}

			// Extract input columns
			inputColRegex := regexp.MustCompile(`(?s)<inputColumn[^>]*name="([^"]*)"[^>]*dataType="([^"]*)"[^>]*(?:length="([^"]*)")?`)
			inputMatches := inputColRegex.FindAllStringSubmatch(componentSection, -1)
			if len(inputMatches) > 0 {
				result.WriteString("  Input Columns:\n")
				for _, inputMatch := range inputMatches {
					if len(inputMatch) > 2 {
						colName := inputMatch[1]
						dataType := inputMatch[2]
						result.WriteString(fmt.Sprintf("    %s (%s", colName, dataType))
						if len(inputMatch) > 3 && inputMatch[3] != "" {
							result.WriteString(fmt.Sprintf(", length=%s", inputMatch[3]))
						}
						result.WriteString(")\n")
					}
				}
			}

			// Extract output columns
			outputColRegex := regexp.MustCompile(`(?s)<outputColumn[^>]*name="([^"]*)"[^>]*dataType="([^"]*)"[^>]*(?:length="([^"]*)")?`)
			outputMatches := outputColRegex.FindAllStringSubmatch(componentSection, -1)
			if len(outputMatches) > 0 {
				result.WriteString("  Output Columns:\n")
				for _, outputMatch := range outputMatches {
					if len(outputMatch) > 2 {
						colName := outputMatch[1]
						dataType := outputMatch[2]
						result.WriteString(fmt.Sprintf("    %s (%s", colName, dataType))
						if len(outputMatch) > 3 && outputMatch[3] != "" {
							result.WriteString(fmt.Sprintf(", length=%s", outputMatch[3]))
						}
						result.WriteString(")\n")
					}
				}
			}
		}
	} else {
		result.WriteString("No components found in data flow.\n")
	}

	// Extract path information
	pathRegex := regexp.MustCompile(`(?s)<path[^>]*>`)
	pathMatches := pathRegex.FindAllString(xmlContent, -1)

	if len(pathMatches) > 0 {
		result.WriteString("\nData Paths:\n")
		for _, pathTag := range pathMatches {
			// Extract individual attributes
			refIDRegex := regexp.MustCompile(`refId="[^"]*\.Paths\[([^]]+)\]"`)
			startIDRegex := regexp.MustCompile(`startId="([^"]*)"`)
			endIDRegex := regexp.MustCompile(`endId="([^"]*)"`)

			refIDMatch := refIDRegex.FindStringSubmatch(pathTag)
			startIDMatch := startIDRegex.FindStringSubmatch(pathTag)
			endIDMatch := endIDRegex.FindStringSubmatch(pathTag)

			pathName := ""
			if len(refIDMatch) > 1 {
				pathName = refIDMatch[1]
			}

			startID := ""
			if len(startIDMatch) > 1 {
				// Extract just the component name from the full ID
				startFull := startIDMatch[1]
				if idx := strings.LastIndex(startFull, "\\"); idx > 0 {
					startID = startFull[idx+1:]
				} else {
					startID = startFull
				}
			}

			endID := ""
			if len(endIDMatch) > 1 {
				// Extract just the component name from the full ID
				endFull := endIDMatch[1]
				if idx := strings.LastIndex(endFull, "\\"); idx > 0 {
					endID = endFull[idx+1:]
				} else {
					endID = endFull
				}
			}

			result.WriteString(fmt.Sprintf("  %s: %s → %s\n", pathName, startID, endID))
		}
	}

	analysisResult := formatter.CreateAnalysisResult("Detailed Data Flow Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeOLEDBSource handles OLE DB source component analysis from DTSX files
func HandleAnalyzeOLEDBSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("OLE DB Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	xmlContent := string(data)

	var result strings.Builder
	result.WriteString("OLE DB Source Analysis:\n\n")

	// Find OLE DB Source components using regex
	componentRegex := regexp.MustCompile(`(?s)<component[^>]*componentClassID="Microsoft\.OLEDBSource"[^>]*>.*?</component>`)
	matches := componentRegex.FindAllString(xmlContent, -1)

	if len(matches) > 0 {
		for i, componentSection := range matches {
			if i > 0 {
				result.WriteString("\n")
			}

			// Extract component name
			nameRegex := regexp.MustCompile(`name="([^"]*)"`)
			nameMatch := nameRegex.FindStringSubmatch(componentSection)
			componentName := ""
			if len(nameMatch) > 1 {
				componentName = nameMatch[1]
			}

			result.WriteString(fmt.Sprintf("Component: %s\n", componentName))

			// Extract description
			descRegex := regexp.MustCompile(`description="([^"]*)"`)
			descMatch := descRegex.FindStringSubmatch(componentSection)
			if len(descMatch) > 1 {
				result.WriteString(fmt.Sprintf("Description: %s\n", descMatch[1]))
			}

			// Extract properties
			propRegex := regexp.MustCompile(`<property[^>]*name="([^"]*)"[^>]*>([^<]*)</property>`)
			propMatches := propRegex.FindAllStringSubmatch(componentSection, -1)
			if len(propMatches) > 0 {
				result.WriteString("Properties:\n")
				for _, propMatch := range propMatches {
					if len(propMatch) > 2 {
						result.WriteString(fmt.Sprintf("  %s: %s\n", propMatch[1], propMatch[2]))
					}
				}
			}

			// Extract output columns
			outputColRegex := regexp.MustCompile(`(?s)<outputColumn[^>]*name="([^"]*)"[^>]*dataType="([^"]*)"[^>]*(?:length="([^"]*)")?`)
			outputMatches := outputColRegex.FindAllStringSubmatch(componentSection, -1)
			if len(outputMatches) > 0 {
				result.WriteString("Output Columns:\n")
				for _, outputMatch := range outputMatches {
					if len(outputMatch) > 2 {
						colName := outputMatch[1]
						dataType := outputMatch[2]
						result.WriteString(fmt.Sprintf("  %s (%s", colName, dataType))
						if len(outputMatch) > 3 && outputMatch[3] != "" {
							result.WriteString(fmt.Sprintf(", length=%s", outputMatch[3]))
						}
						result.WriteString(")\n")
					}
				}
			}
		}
	} else {
		result.WriteString("No OLE DB Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("OLE DB Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeADONETSource handles ADO.NET Source component analysis from DTSX files
func HandleAnalyzeADONETSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("ADO.NET Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("ADO.NET Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("ADO.NET Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.DataReaderSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No ADO.NET Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("ADO.NET Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeODBCSource handles ODBC Source component analysis from DTSX files
func HandleAnalyzeODBCSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("ODBC Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("ODBC Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("ODBC Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.OdbcSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No ODBC Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("ODBC Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeExportColumn handles Export Column component analysis from DTSX files
func HandleAnalyzeExportColumn(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Export Column Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	xmlContent := string(data)

	var result strings.Builder
	result.WriteString("Export Column Analysis:\n\n")

	// Find Export Column components using regex
	componentRegex := regexp.MustCompile(`(?s)<component[^>]*componentClassID="Microsoft\.Extractor"[^>]*>.*?</component>`)
	matches := componentRegex.FindAllString(xmlContent, -1)

	if len(matches) > 0 {
		for i, componentSection := range matches {
			if i > 0 {
				result.WriteString("\n")
			}

			// Extract component name
			nameRegex := regexp.MustCompile(`name="([^"]*)"`)
			nameMatch := nameRegex.FindStringSubmatch(componentSection)
			componentName := ""
			if len(nameMatch) > 1 {
				componentName = nameMatch[1]
			}

			result.WriteString(fmt.Sprintf("Component: %s\n", componentName))

			// Extract description
			descRegex := regexp.MustCompile(`description="([^"]*)"`)
			descMatch := descRegex.FindStringSubmatch(componentSection)
			if len(descMatch) > 1 {
				result.WriteString(fmt.Sprintf("Description: %s\n", descMatch[1]))
			}

			// Extract properties
			propRegex := regexp.MustCompile(`<property[^>]*name="([^"]*)"[^>]*>([^<]*)</property>`)
			propMatches := propRegex.FindAllStringSubmatch(componentSection, -1)
			if len(propMatches) > 0 {
				result.WriteString("Properties:\n")
				for _, propMatch := range propMatches {
					if len(propMatch) > 2 {
						result.WriteString(fmt.Sprintf("  %s: %s\n", propMatch[1], propMatch[2]))
					}
				}
			}

			// Extract input columns
			inputColRegex := regexp.MustCompile(`(?s)<inputColumn[^>]*name="([^"]*)"[^>]*dataType="([^"]*)"[^>]*(?:length="([^"]*)")?`)
			inputMatches := inputColRegex.FindAllStringSubmatch(componentSection, -1)
			if len(inputMatches) > 0 {
				result.WriteString("Input Columns:\n")
				for _, inputMatch := range inputMatches {
					if len(inputMatch) > 2 {
						colName := inputMatch[1]
						dataType := inputMatch[2]
						result.WriteString(fmt.Sprintf("  %s (%s", colName, dataType))
						if len(inputMatch) > 3 && inputMatch[3] != "" {
							result.WriteString(fmt.Sprintf(", length=%s", inputMatch[3]))
						}
						result.WriteString(")\n")
					}
				}
			}
		}
	} else {
		result.WriteString("No Export Column components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Export Column Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeDataConversion handles Data Conversion component analysis from DTSX files
func HandleAnalyzeDataConversion(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Data Conversion Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	xmlContent := string(data)

	var result strings.Builder
	result.WriteString("Data Conversion Analysis:\n\n")

	// Find Data Conversion components using regex
	componentRegex := regexp.MustCompile(`(?s)<component[^>]*componentClassID="Microsoft\.DataConvert"[^>]*>.*?</component>`)
	matches := componentRegex.FindAllString(xmlContent, -1)

	if len(matches) > 0 {
		for i, componentSection := range matches {
			if i > 0 {
				result.WriteString("\n")
			}

			// Extract component name
			nameRegex := regexp.MustCompile(`name="([^"]*)"`)
			nameMatch := nameRegex.FindStringSubmatch(componentSection)
			componentName := ""
			if len(nameMatch) > 1 {
				componentName = nameMatch[1]
			}

			result.WriteString(fmt.Sprintf("Component: %s\n", componentName))

			// Extract description
			descRegex := regexp.MustCompile(`description="([^"]*)"`)
			descMatch := descRegex.FindStringSubmatch(componentSection)
			if len(descMatch) > 1 {
				result.WriteString(fmt.Sprintf("Description: %s\n", descMatch[1]))
			}

			// Extract properties
			propRegex := regexp.MustCompile(`<property[^>]*name="([^"]*)"[^>]*>([^<]*)</property>`)
			propMatches := propRegex.FindAllStringSubmatch(componentSection, -1)
			if len(propMatches) > 0 {
				result.WriteString("Properties:\n")
				for _, propMatch := range propMatches {
					if len(propMatch) > 2 {
						result.WriteString(fmt.Sprintf("  %s: %s\n", propMatch[1], propMatch[2]))
					}
				}
			}

			// Extract input columns
			inputColRegex := regexp.MustCompile(`(?s)<inputColumn[^>]*name="([^"]*)"[^>]*dataType="([^"]*)"[^>]*(?:length="([^"]*)")?`)
			inputMatches := inputColRegex.FindAllStringSubmatch(componentSection, -1)
			if len(inputMatches) > 0 {
				result.WriteString("Input Columns:\n")
				for _, inputMatch := range inputMatches {
					if len(inputMatch) > 2 {
						colName := inputMatch[1]
						dataType := inputMatch[2]
						result.WriteString(fmt.Sprintf("  %s (%s", colName, dataType))
						if len(inputMatch) > 3 && inputMatch[3] != "" {
							result.WriteString(fmt.Sprintf(", length=%s", inputMatch[3]))
						}
						result.WriteString(")\n")
					}
				}
			}

			// Extract output columns
			outputColRegex := regexp.MustCompile(`(?s)<outputColumn[^>]*name="([^"]*)"[^>]*dataType="([^"]*)"[^>]*(?:length="([^"]*)")?`)
			outputMatches := outputColRegex.FindAllStringSubmatch(componentSection, -1)
			if len(outputMatches) > 0 {
				result.WriteString("Output Columns:\n")
				for _, outputMatch := range outputMatches {
					if len(outputMatch) > 2 {
						colName := outputMatch[1]
						dataType := outputMatch[2]
						result.WriteString(fmt.Sprintf("  %s (%s", colName, dataType))
						if len(outputMatch) > 3 && outputMatch[3] != "" {
							result.WriteString(fmt.Sprintf(", length=%s", outputMatch[3]))
						}
						result.WriteString(")\n")
					}
				}
			}
		}
	} else {
		result.WriteString("No Data Conversion components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Data Conversion Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeFlatFileSource handles Flat File Source component analysis from DTSX files
func HandleAnalyzeFlatFileSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Flat File Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Flat File Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Flat File Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FlatFileSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Flat File Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Flat File Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeExcelSource handles Excel Source component analysis from DTSX files
func HandleAnalyzeExcelSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Excel Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Excel Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Excel Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ExcelSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Excel Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Excel Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeAccessSource handles Access Source component analysis from DTSX files
func HandleAnalyzeAccessSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Access Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Access Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Access Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.AccessSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Access Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Access Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeXMLSource handles XML Source component analysis from DTSX files
func HandleAnalyzeXMLSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("XML Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("XML Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("XML Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.XmlSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No XML Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("XML Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeRawFileSource handles raw file source analysis from DTSX files
func HandleAnalyzeRawFileSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Raw File Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Raw File Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Raw File Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.RawFileSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Raw File Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Raw File Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeCDCSource handles CDC source analysis from DTSX files
func HandleAnalyzeCDCSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("CDC Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("CDC Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("CDC Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.CdcSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No CDC Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("CDC Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeSAPBWSource handles SAP BW source analysis from DTSX files
func HandleAnalyzeSAPBWSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("SAP BW Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("SAP BW Source Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("SAP BW Source Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.SapBwSourceAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Outputs
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No SAP BW Source components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("SAP BW Source Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeOLEDBDestination handles OLE DB destination analysis from DTSX files
// HandleAnalyzeDestination provides unified analysis for various SSIS destination components
func HandleAnalyzeDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	destinationType, err := request.RequireString("destination_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Map destination types to ComponentClassIDs
	destinationTypeMap := map[string]string{
		"ole_db":     "Microsoft.SqlServer.Dts.Pipeline.OLEDBDestinationAdapter",
		"flat_file":  "Microsoft.SqlServer.Dts.Pipeline.FlatFileDestinationAdapter",
		"sql_server": "Microsoft.SqlServer.Dts.Pipeline.SqlServerDestinationAdapter",
		"excel":      "Microsoft.SqlServer.Dts.Pipeline.ExcelDestinationAdapter",
		"raw_file":   "Microsoft.SqlServer.Dts.Pipeline.RawFileDestinationAdapter",
	}

	componentClassID, exists := destinationTypeMap[destinationType]
	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Unknown destination type: %s. Supported types: ole_db, flat_file, sql_server, excel, raw_file", destinationType)), nil
	}

	// Map destination types to display names
	destinationNameMap := map[string]string{
		"ole_db":     "OLE DB Destination",
		"flat_file":  "Flat File Destination",
		"sql_server": "SQL Server Destination",
		"excel":      "Excel Destination",
		"raw_file":   "Raw File Destination",
	}

	displayName := destinationNameMap[destinationType]
	analysisTitle := fmt.Sprintf("%s Analysis", displayName)

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult(analysisTitle, filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult(analysisTitle, filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s:\n\n", analysisTitle))

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == componentClassID {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Input Columns
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString(fmt.Sprintf("No %s components found in this package.\n", displayName))
	}

	analysisResult := formatter.CreateAnalysisResult(analysisTitle, filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeOLEDBDestination handles OLE DB destination analysis from DTSX files
func HandleAnalyzeOLEDBDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("OLE DB Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("OLE DB Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("OLE DB Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.OLEDBDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No OLE DB Destination components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("OLE DB Destination Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeFlatFileDestination handles flat file destination analysis from DTSX files
func HandleAnalyzeFlatFileDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Flat File Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Flat File Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Flat File Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FlatFileDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Flat File Destination components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Flat File Destination Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeSQLServerDestination handles SQL Server destination analysis from DTSX files
func HandleAnalyzeSQLServerDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("SQL Server Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("SQL Server Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("SQL Server Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.SqlServerDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs
					result.WriteString("Input Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No SQL Server Destination components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("SQL Server Destination Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeDerivedColumn handles derived column analysis from DTSX files
func HandleAnalyzeDerivedColumn(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Derived Column Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Derived Column Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Derived Column Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.DerivedColumn" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  Output: %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Derived Column components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Derived Column Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeLookup handles lookup component analysis from DTSX files
func HandleAnalyzeLookup(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		analysisResult := formatter.CreateAnalysisResult("Lookup Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		analysisResult := formatter.CreateAnalysisResult("Lookup Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	var result strings.Builder
	result.WriteString("Lookup Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Lookup" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  Output: %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Lookup components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Lookup Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeConditionalSplit handles conditional split component analysis from DTSX files
func HandleAnalyzeConditionalSplit(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		analysisResult := formatter.CreateAnalysisResult("Conditional Split Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		analysisResult := formatter.CreateAnalysisResult("Conditional Split Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	var result strings.Builder
	result.WriteString("Conditional Split Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ConditionalSplit" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Conditional Split components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Conditional Split Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeSort handles sort component analysis from DTSX files
func HandleAnalyzeSort(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		analysisResult := formatter.CreateAnalysisResult("Sort Transform Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		analysisResult := formatter.CreateAnalysisResult("Sort Transform Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	var result strings.Builder
	result.WriteString("Sort Transform Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Sort" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Sort components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Sort Transform Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeAggregate handles aggregate component analysis from DTSX files
func HandleAnalyzeAggregate(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		analysisResult := formatter.CreateAnalysisResult("Aggregate Transform Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		analysisResult := formatter.CreateAnalysisResult("Aggregate Transform Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	var result strings.Builder
	result.WriteString("Aggregate Transform Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Aggregate" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Aggregate components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Aggregate Transform Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeMergeJoin handles merge join component analysis from DTSX files
func HandleAnalyzeMergeJoin(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		analysisResult := formatter.CreateAnalysisResult("Merge Join Transform Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		analysisResult := formatter.CreateAnalysisResult("Merge Join Transform Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	var result strings.Builder
	result.WriteString("Merge Join Transform Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.MergeJoin" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Merge Join components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Merge Join Transform Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeUnionAll handles Union All transformation analysis from DTSX files
func HandleAnalyzeUnionAll(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Union All Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Union All Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Union All Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.UnionAll" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					inputCount := 0
					for _, input := range comp.Inputs.Inputs {
						inputCount++
						result.WriteString(fmt.Sprintf("  Input %d:\n", inputCount))
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString("  Output:\n")
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Union All components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Union All Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeMulticast handles Multicast transformation analysis from DTSX files
func HandleAnalyzeMulticast(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Multicast Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Multicast Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Multicast Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Multicast" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Configuration:\n")
					for _, input := range comp.Inputs.Inputs {
						result.WriteString("  Input:\n")
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					outputCount := 0
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							outputCount++
							result.WriteString(fmt.Sprintf("  Output %d:\n", outputCount))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString(fmt.Sprintf("Total Outputs: %d\n", outputCount))
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Multicast components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Multicast Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeScriptComponent handles Script Component transformation analysis from DTSX files
func HandleAnalyzeScriptComponent(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Script Component Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Script Component Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Script Component Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ScriptComponent" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) || prop.Name == "ScriptLanguage" || prop.Name == "ReadOnlyVariables" || prop.Name == "ReadWriteVariables" {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}

					// Script code (if available in properties)
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if prop.Name == "SourceCode" || prop.Name == "ScriptCode" {
							result.WriteString("Script Code:\n")
							result.WriteString(fmt.Sprintf("  %s\n", prop.Value))
						}
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						result.WriteString("  Input:\n")
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString("  Output:\n")
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Script Component components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Script Component Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzePivot handles pivot transformation analysis from DTSX files
func HandleAnalyzePivot(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Pivot Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Pivot Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Pivot Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Pivot" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "PivotKey":
							result.WriteString(fmt.Sprintf("  Pivot Key: %s\n", prop.Value))
						case "SetKey":
							result.WriteString(fmt.Sprintf("  Set Key: %s\n", prop.Value))
						case "PivotValue":
							result.WriteString(fmt.Sprintf("  Pivot Value: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Pivot components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Pivot Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeUnpivot handles unpivot transformation analysis from DTSX files
func HandleAnalyzeUnpivot(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Unpivot Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Unpivot Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Unpivot Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.Unpivot" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "PivotKeyValue":
							result.WriteString(fmt.Sprintf("  Pivot Key Value: %s\n", prop.Value))
						case "DestinationColumn":
							result.WriteString(fmt.Sprintf("  Destination Column: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Unpivot components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Unpivot Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeTermExtraction handles term extraction transformation analysis from DTSX files
func HandleAnalyzeTermExtraction(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Term Extraction Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Term Extraction Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Term Extraction Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.TermExtraction" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "ExclusionTermTable":
							result.WriteString(fmt.Sprintf("  Exclusion Term Table: %s\n", prop.Value))
						case "TermTable":
							result.WriteString(fmt.Sprintf("  Term Table: %s\n", prop.Value))
						case "ScoreType":
							result.WriteString(fmt.Sprintf("  Score Type: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Term Extraction components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Term Extraction Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeFuzzyLookup handles fuzzy lookup transformation analysis from DTSX files
func HandleAnalyzeFuzzyLookup(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Fuzzy Lookup Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Fuzzy Lookup Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Fuzzy Lookup Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FuzzyLookup" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "ReferenceTableName":
							result.WriteString(fmt.Sprintf("  Reference Table: %s\n", prop.Value))
						case "MaxOutputMatches":
							result.WriteString(fmt.Sprintf("  Max Output Matches: %s\n", prop.Value))
						case "SimilarityThreshold":
							result.WriteString(fmt.Sprintf("  Similarity Threshold: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Fuzzy Lookup components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Fuzzy Lookup Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeFuzzyGrouping handles fuzzy grouping transformation analysis from DTSX files
func HandleAnalyzeFuzzyGrouping(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Fuzzy Grouping Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Fuzzy Grouping Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Fuzzy Grouping Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.FuzzyGrouping" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						switch prop.Name {
						case "GroupingKey":
							result.WriteString(fmt.Sprintf("  Grouping Key: %s\n", prop.Value))
						case "SimilarityThreshold":
							result.WriteString(fmt.Sprintf("  Similarity Threshold: %s\n", prop.Value))
						case "MinimumSimilarity":
							result.WriteString(fmt.Sprintf("  Minimum Similarity: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Fuzzy Grouping components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Fuzzy Grouping Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeRowCount handles row count transformation analysis from DTSX files
func HandleAnalyzeRowCount(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Row Count Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Row Count Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Row Count Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.RowCount" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if prop.Name == "VariableName" {
							result.WriteString(fmt.Sprintf("  Variable Name: %s\n", prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Row Count components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Row Count Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeCharacterMap handles character map transformation analysis from DTSX files
func HandleAnalyzeCharacterMap(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Character Map Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Character Map Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Character Map Analysis:\n\n")
	xmlContent := string(data)

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.CharacterMap" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					componentPattern := fmt.Sprintf(`<component name="%s">(.*?)</component>`, comp.Name)
					re := regexp.MustCompile(componentPattern)
					matches := re.FindStringSubmatch(xmlContent)
					if len(matches) > 1 {
						componentXML := matches[1]
						opRe := regexp.MustCompile(`<property name="Operation">(.*?)</property>`)
						opMatches := opRe.FindAllStringSubmatch(componentXML, -1)
						for idx, match := range opMatches {
							if len(match) > 1 {
								result.WriteString(fmt.Sprintf("  Operation %d: %s\n", idx+1, strings.TrimSpace(match[1])))
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Character Map components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Character Map Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeCopyColumn handles copy column transformation analysis from DTSX files
func HandleAnalyzeCopyColumn(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Copy Column Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Copy Column Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Copy Column Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.CopyColumn" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					inputCount := 0
					if len(comp.Inputs.Inputs) > 0 {
						inputCount = len(comp.Inputs.Inputs[0].InputColumns.Columns)
					}
					outputCount := 0
					if len(comp.Outputs.Outputs) > 0 {
						outputCount = len(comp.Outputs.Outputs[0].OutputColumns.Columns)
					}
					result.WriteString(fmt.Sprintf("  Input Columns: %d\n", inputCount))
					result.WriteString(fmt.Sprintf("  Output Columns: %d\n", outputCount))
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Copy Column components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Copy Column Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeContainers handles container analysis from DTSX files
func HandleAnalyzeContainers(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Container Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Container Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Container Analysis:\n\n")
	containerCount := 0
	xmlContent := string(data)

	for _, task := range pkg.Executables.Tasks {
		containerType := ""
		switch task.CreationName {
		case "Microsoft.Sequence":
			containerType = "Sequence Container"
		case "Microsoft.ForLoop":
			containerType = "For Loop Container"
		case "Microsoft.ForEachLoop":
			containerType = "Foreach Loop Container"
		}

		if containerType != "" {
			containerCount++
			result.WriteString(fmt.Sprintf("Container %d: %s (%s)\n", containerCount, task.Name, containerType))
			result.WriteString(fmt.Sprintf("  Description: %s\n", task.Description))

			for _, prop := range task.Properties {
				switch prop.Name {
				case "Disabled":
					result.WriteString(fmt.Sprintf("  Disabled: %s\n", prop.Value))
				case "FailPackageOnFailure":
					result.WriteString(fmt.Sprintf("  Fail Package On Failure: %s\n", prop.Value))
				case "FailParentOnFailure":
					result.WriteString(fmt.Sprintf("  Fail Parent On Failure: %s\n", prop.Value))
				}
			}

			if task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode != "" {
				result.WriteString("  Contains Script Task Content\n\n")
				continue
			}

			containerPattern := fmt.Sprintf(`<Executable.*ObjectName="%s".*CreationName="%s">(.*?)</Executable>`, regexp.QuoteMeta(task.Name), regexp.QuoteMeta(task.CreationName))
			re := regexp.MustCompile(containerPattern)
			matches := re.FindStringSubmatch(xmlContent)
			if len(matches) > 1 {
				containerXML := matches[1]
				nestedRe := regexp.MustCompile(`<Executable.*CreationName="`)
				nestedMatches := nestedRe.FindAllString(containerXML, -1)
				result.WriteString(fmt.Sprintf("  Nested Executables: %d\n", len(nestedMatches)))
			}

			result.WriteString("\n")
		}
	}

	if containerCount == 0 {
		result.WriteString("No containers found in this package.\n")
	} else {
		result.WriteString(fmt.Sprintf("Total containers found: %d\n", containerCount))
	}

	analysisResult := formatter.CreateAnalysisResult("Container Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeCustomComponents handles custom component analysis from DTSX files
func HandleAnalyzeCustomComponents(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Custom Component Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Custom Component Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Custom and Third-Party Components Analysis:\n\n")
	customCount := 0

	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				classID := strings.ToLower(comp.ComponentClassID)
				vendor := ""
				isCustom := false

				if strings.Contains(classID, "kingswaysoft") {
					isCustom = true
					vendor = "KingswaySoft"
				} else if strings.Contains(classID, "cozyroc") {
					isCustom = true
					vendor = "CozyRoc"
				} else if strings.Contains(classID, "pragmaticworks") || strings.Contains(classID, "pragmatic") {
					isCustom = true
					vendor = "Pragmatic Works"
				} else if strings.Contains(classID, "auntiedot") {
					isCustom = true
					vendor = "AuntieDot"
				} else if strings.Contains(classID, "bluessis") {
					isCustom = true
					vendor = "BlueSSIS"
				} else if !strings.HasPrefix(classID, "microsoft.sqlserver.dts.pipeline") &&
					!strings.HasPrefix(classID, "microsoft.sqlserver.dts") &&
					!strings.HasPrefix(classID, "microsoft.") {
					isCustom = true
					vendor = "Unknown/Custom"
				}

				if isCustom {
					customCount++
					result.WriteString(fmt.Sprintf("Component %d: %s\n", customCount, comp.Name))
					result.WriteString(fmt.Sprintf("  Vendor: %s\n", vendor))
					result.WriteString(fmt.Sprintf("  Class ID: %s\n", comp.ComponentClassID))
					result.WriteString(fmt.Sprintf("  Description: %s\n", comp.Description))

					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						if isKeyProperty(prop.Name) {
							result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if customCount == 0 {
		result.WriteString("No custom or third-party components found in this package.\n")
		result.WriteString("All components appear to be standard Microsoft SSIS components.\n")
	} else {
		result.WriteString(fmt.Sprintf("Total custom/third-party components found: %d\n", customCount))
	}

	analysisResult := formatter.CreateAnalysisResult("Custom Component Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeExcelDestination handles Excel Destination component analysis from DTSX files
func HandleAnalyzeExcelDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Excel Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Excel Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Excel Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.ExcelDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Excel Destination components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Excel Destination Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeRawFileDestination handles Raw File Destination component analysis from DTSX files
func HandleAnalyzeRawFileDestination(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Raw File Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Raw File Destination Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Raw File Destination Analysis:\n\n")

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == "Microsoft.SqlServer.Dts.Pipeline.RawFileDestinationAdapter" {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Inputs and Outputs
					result.WriteString("Input/Output Columns:\n")
					for _, input := range comp.Inputs.Inputs {
						for _, col := range input.InputColumns.Columns {
							result.WriteString(fmt.Sprintf("  Input: %s (%s", col.Name, col.DataType))
							if col.Length > 0 {
								result.WriteString(fmt.Sprintf(", length=%d", col.Length))
							}
							result.WriteString(")\n")
						}
					}
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							result.WriteString(fmt.Sprintf("  Output: %s\n", output.Name))
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("    %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString("No Raw File Destination components found in this package.\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Raw File Destination Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeEventHandlers handles event handler analysis from DTSX files
func HandleAnalyzeEventHandlers(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Event Handler Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Event Handler Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Event Handler Analysis:\n\n")

	if len(pkg.EventHandlers.EventHandlers) == 0 {
		result.WriteString("No event handlers found in this package.\n")
		analysisResult := formatter.CreateAnalysisResult("Event Handler Analysis", filePath, result.String(), nil)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	result.WriteString(fmt.Sprintf("Found %d event handler(s):\n\n", len(pkg.EventHandlers.EventHandlers)))

	for i, eh := range pkg.EventHandlers.EventHandlers {
		result.WriteString(fmt.Sprintf("%d. Event Handler: %s\n", i+1, eh.ObjectName))
		result.WriteString(fmt.Sprintf("   Type: %s\n", eh.EventHandlerType))
		result.WriteString(fmt.Sprintf("   Container: %s\n", eh.ContainerID))

		// Analyze tasks in the event handler
		if len(eh.Executables.Tasks) > 0 {
			result.WriteString(fmt.Sprintf("   Tasks (%d):\n", len(eh.Executables.Tasks)))
			for _, task := range eh.Executables.Tasks {
				taskType := getTaskType(task)
				result.WriteString(fmt.Sprintf("     - %s (%s)\n", task.Name, taskType))

				// Show key properties
				for _, prop := range task.Properties {
					if isKeyProperty(prop.Name) {
						result.WriteString(fmt.Sprintf("       %s: %s\n", prop.Name, prop.Value))
					}
				}
			}
		} else {
			result.WriteString("   Tasks: None\n")
		}

		// Analyze variables in the event handler
		if len(eh.Variables.Vars) > 0 {
			result.WriteString(fmt.Sprintf("   Variables (%d):\n", len(eh.Variables.Vars)))
			for _, variable := range eh.Variables.Vars {
				resolvedValue := resolveVariableExpressions(variable.Value, pkg.Variables.Vars, 10)
				result.WriteString(fmt.Sprintf("     - %s: %s\n", variable.Name, resolvedValue))
				if variable.Expression != "" {
					result.WriteString(fmt.Sprintf("       Expression: %s\n", variable.Expression))
				}
			}
		}

		// Analyze precedence constraints in the event handler
		if len(eh.PrecedenceConstraints.Constraints) > 0 {
			result.WriteString(fmt.Sprintf("   Precedence Constraints (%d):\n", len(eh.PrecedenceConstraints.Constraints)))
			for _, constraint := range eh.PrecedenceConstraints.Constraints {
				result.WriteString(fmt.Sprintf("     - %s → %s", constraint.From, constraint.To))
				if constraint.Expression != "" {
					resolvedExpr := resolveVariableExpressions(constraint.Expression, append(pkg.Variables.Vars, eh.Variables.Vars...), 10)
					result.WriteString(fmt.Sprintf(" (Expression: %s)", resolvedExpr))
				}
				result.WriteString("\n")
			}
		}

		result.WriteString("\n")
	}

	analysisResult := formatter.CreateAnalysisResult("Event Handler Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzePackageDependencies handles package dependency analysis across multiple DTSX files
func HandleAnalyzePackageDependencies(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	// Find all DTSX files in the package directory
	var dtsxFiles []string
	err := filepath.Walk(packageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".dtsx") {
			dtsxFiles = append(dtsxFiles, path)
		}
		return nil
	})
	if err != nil {
		result := formatter.CreateAnalysisResult("Package Dependency Analysis", "", nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	if len(dtsxFiles) == 0 {
		result := "No DTSX files found in the package directory."
		analysisResult := formatter.CreateAnalysisResult("Package Dependency Analysis", "", result, nil)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	// Data structures to track dependencies
	type ConnectionInfo struct {
		Name             string
		ConnectionString string
		Packages         []string
	}

	type VariableInfo struct {
		Name     string
		Value    string
		Packages []string
	}

	connections := make(map[string]*ConnectionInfo)
	variables := make(map[string]*VariableInfo)

	// Process each DTSX file
	for _, filePath := range dtsxFiles {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Remove namespace prefixes for easier parsing
		data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

		var pkg types.SSISPackage
		if err := xml.Unmarshal(data, &pkg); err != nil {
			continue // Skip files that can't be parsed
		}

		packageName := filepath.Base(filePath)

		// Extract connections
		for _, conn := range pkg.ConnectionMgr.Connections {
			connKey := conn.Name // Use connection name as key
			if connections[connKey] == nil {
				connections[connKey] = &ConnectionInfo{
					Name:     conn.Name,
					Packages: []string{},
				}
			}
			// Add package to connection's package list if not already present
			found := false
			for _, pkg := range connections[connKey].Packages {
				if pkg == packageName {
					found = true
					break
				}
			}
			if !found {
				connections[connKey].Packages = append(connections[connKey].Packages, packageName)
			}
		}

		// Extract variables
		for _, variable := range pkg.Variables.Vars {
			varKey := variable.Name // Use variable name as key
			if variables[varKey] == nil {
				variables[varKey] = &VariableInfo{
					Name:     variable.Name,
					Packages: []string{},
				}
			}
			// Add package to variable's package list if not already present
			found := false
			for _, pkg := range variables[varKey].Packages {
				if pkg == packageName {
					found = true
					break
				}
			}
			if !found {
				variables[varKey].Packages = append(variables[varKey].Packages, packageName)
			}
		}
	}

	// Build the result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Package Dependency Analysis (%d packages scanned)\n\n", len(dtsxFiles)))

	// Shared Connections
	result.WriteString("🔗 Shared Connections:\n")
	sharedConnections := 0
	for _, conn := range connections {
		if len(conn.Packages) > 1 {
			sharedConnections++
			result.WriteString(fmt.Sprintf("• %s (used by %d packages):\n", conn.Name, len(conn.Packages)))
			for _, pkg := range conn.Packages {
				result.WriteString(fmt.Sprintf("  - %s\n", pkg))
			}
			result.WriteString("\n")
		}
	}
	if sharedConnections == 0 {
		result.WriteString("No shared connections found.\n\n")
	}

	// Shared Variables
	result.WriteString("📊 Shared Variables:\n")
	sharedVariables := 0
	for _, variable := range variables {
		if len(variable.Packages) > 1 {
			sharedVariables++
			result.WriteString(fmt.Sprintf("• %s (used by %d packages):\n", variable.Name, len(variable.Packages)))
			for _, pkg := range variable.Packages {
				result.WriteString(fmt.Sprintf("  - %s\n", pkg))
			}
			result.WriteString("\n")
		}
	}
	if sharedVariables == 0 {
		result.WriteString("No shared variables found.\n\n")
	}

	// Summary
	result.WriteString("📈 Summary:\n")
	result.WriteString(fmt.Sprintf("• Total packages analyzed: %d\n", len(dtsxFiles)))
	result.WriteString(fmt.Sprintf("• Shared connections: %d\n", sharedConnections))
	result.WriteString(fmt.Sprintf("• Shared variables: %d\n", sharedVariables))

	if sharedConnections > 0 || sharedVariables > 0 {
		result.WriteString("\n💡 These shared resources indicate potential dependencies between packages.")
		result.WriteString("\n   Consider documenting these relationships for maintenance and deployment purposes.")
	}

	analysisResult := formatter.CreateAnalysisResult("Package Dependency Analysis", "", result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeConfigurations handles configuration analysis from DTSX files
func HandleAnalyzeConfigurations(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Configuration Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Configuration Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Configuration Analysis:\n\n")

	if len(pkg.Configurations.Configs) == 0 {
		result.WriteString("No configurations found in this package.\n")
		result.WriteString("\n💡 Note: Configurations were used in SSIS 2005-2008 for parameterization.")
		result.WriteString(" Modern SSIS (2012+) uses Parameters instead.")
		analysisResult := formatter.CreateAnalysisResult("Configuration Analysis", filePath, result.String(), nil)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
	}

	result.WriteString(fmt.Sprintf("Found %d configuration(s):\n\n", len(pkg.Configurations.Configs)))

	// Configuration type mapping
	configTypes := map[int]string{
		0: "Parent Package Variable",
		1: "XML Configuration File",
		2: "Environment Variable",
		3: "Registry Entry",
		4: "Parent Package Variable (indirect)",
		5: "XML Configuration File (indirect)",
		6: "Environment Variable (indirect)",
		7: "Registry Entry (indirect)",
		8: "SQL Server",
		9: "SQL Server (indirect)",
	}

	for i, config := range pkg.Configurations.Configs {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, config.Name))

		// Configuration type
		typeName, exists := configTypes[config.Type]
		if exists {
			result.WriteString(fmt.Sprintf("   Type: %s (%d)\n", typeName, config.Type))
		} else {
			result.WriteString(fmt.Sprintf("   Type: Unknown (%d)\n", config.Type))
		}

		// Description
		if config.Description != "" {
			result.WriteString(fmt.Sprintf("   Description: %s\n", config.Description))
		}

		// Configuration string (connection info for SQL Server, file path for XML, etc.)
		if config.ConfigurationString != "" {
			result.WriteString(fmt.Sprintf("   Configuration String: %s\n", config.ConfigurationString))
		}

		// Configured type and value
		if config.ConfiguredType != "" {
			result.WriteString(fmt.Sprintf("   Configured Type: %s\n", config.ConfiguredType))
		}
		if config.ConfiguredValue != "" {
			result.WriteString(fmt.Sprintf("   Configured Value: %s\n", config.ConfiguredValue))
		}

		result.WriteString("\n")
	}

	// Summary and recommendations
	result.WriteString("📋 Configuration Summary:\n")
	xmlConfigs := 0
	sqlConfigs := 0
	envConfigs := 0

	for _, config := range pkg.Configurations.Configs {
		switch config.Type {
		case 1, 5: // XML Configuration File
			xmlConfigs++
		case 8, 9: // SQL Server
			sqlConfigs++
		case 2, 6: // Environment Variable
			envConfigs++
		}
	}

	if xmlConfigs > 0 {
		result.WriteString(fmt.Sprintf("• XML Configuration Files: %d\n", xmlConfigs))
	}
	if sqlConfigs > 0 {
		result.WriteString(fmt.Sprintf("• SQL Server Configurations: %d\n", sqlConfigs))
	}
	if envConfigs > 0 {
		result.WriteString(fmt.Sprintf("• Environment Variable Configurations: %d\n", envConfigs))
	}

	result.WriteString("\n💡 Recommendations:\n")
	result.WriteString("• Consider migrating to SSIS 2012+ Parameters for better security and maintainability\n")
	result.WriteString("• XML configurations should be stored in secure locations\n")
	result.WriteString("• SQL Server configurations require appropriate database permissions\n")
	result.WriteString("• Environment variables are machine-specific and may not work across environments\n")

	analysisResult := formatter.CreateAnalysisResult("Configuration Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzePerformanceMetrics handles performance metrics analysis from DTSX files
func HandleAnalyzePerformanceMetrics(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Performance Metrics Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Performance Metrics Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("Performance Metrics Analysis:\n\n")

	// Analyze package-level performance properties
	result.WriteString("📦 Package-Level Performance Settings:\n")
	packagePerfProps := extractPerformanceProperties(pkg.Properties, "package")
	if len(packagePerfProps) > 0 {
		for _, prop := range packagePerfProps {
			result.WriteString(fmt.Sprintf("• %s: %s\n", prop.Name, prop.Value))
			if prop.Recommendation != "" {
				result.WriteString(fmt.Sprintf("  💡 %s\n", prop.Recommendation))
			}
		}
	} else {
		result.WriteString("No performance-related package properties found.\n")
	}
	result.WriteString("\n")

	// Analyze data flow performance settings
	result.WriteString("🔄 Data Flow Performance Analysis:\n")
	dataFlowCount := 0

	for _, task := range pkg.Executables.Tasks {
		if isDataFlowTask(task) {
			dataFlowCount++
			result.WriteString(fmt.Sprintf("Data Flow Task: %s\n", task.Name))

			// Extract data flow task properties
			taskPerfProps := extractPerformanceProperties(task.Properties, "dataflow")
			if len(taskPerfProps) > 0 {
				result.WriteString("  Task Properties:\n")
				for _, prop := range taskPerfProps {
					result.WriteString(fmt.Sprintf("  • %s: %s\n", prop.Name, prop.Value))
					if prop.Recommendation != "" {
						result.WriteString(fmt.Sprintf("    💡 %s\n", prop.Recommendation))
					}
				}
			}

			// Analyze data flow components
			if task.ObjectData.DataFlow.Components.Components != nil {
				result.WriteString("  Components:\n")
				for _, comp := range task.ObjectData.DataFlow.Components.Components {
					compPerfProps := extractComponentPerformanceProperties(comp)
					if len(compPerfProps) > 0 {
						result.WriteString(fmt.Sprintf("    • %s (%s):\n", comp.Name, getComponentType(comp.ComponentClassID)))
						for _, prop := range compPerfProps {
							result.WriteString(fmt.Sprintf("      - %s: %s\n", prop.Name, prop.Value))
							if prop.Recommendation != "" {
								result.WriteString(fmt.Sprintf("        💡 %s\n", prop.Recommendation))
							}
						}
					}
				}
			}
			result.WriteString("\n")
		}
	}

	if dataFlowCount == 0 {
		result.WriteString("No Data Flow Tasks found in this package.\n\n")
	}

	// Performance recommendations
	result.WriteString("🚀 Performance Optimization Recommendations:\n")
	result.WriteString("• Increase DefaultBufferSize if processing large datasets (recommended: 10MB+)\n")
	result.WriteString("• Adjust DefaultBufferMaxRows based on row size (recommended: 10,000-100,000)\n")
	result.WriteString("• Increase EngineThreads for parallel processing (recommended: 2-4 per CPU core)\n")
	result.WriteString("• Use BLOBTempStoragePath and BufferTempStoragePath for large datasets\n")
	result.WriteString("• Consider MaxConcurrentExecutables for parallel task execution\n")
	result.WriteString("• Monitor AutoAdjustBufferSize for optimal memory usage\n")

	analysisResult := formatter.CreateAnalysisResult("Performance Metrics Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// HandleAnalyzeCodeQuality handles code quality metrics analysis from DTSX files
func HandleAnalyzeCodeQuality(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get format parameter (default to "text")
	formatStr := request.GetString("format", "text")
	format := formatter.OutputFormat(formatStr)

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		result := formatter.CreateAnalysisResult("Code Quality Metrics Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		result := formatter.CreateAnalysisResult("Code Quality Metrics Analysis", filePath, nil, err)
		return mcp.NewToolResultText(formatter.FormatAnalysisResult(result, format)), nil
	}

	var result strings.Builder
	result.WriteString("📊 Code Quality Metrics Analysis\n\n")
	result.WriteString(fmt.Sprintf("Package: %s\n\n", filepath.Base(resolvedPath)))

	// Structural Complexity Metrics
	result.WriteString("🏗️ Structural Complexity:\n")
	structuralScore := calculateStructuralComplexity(pkg)
	result.WriteString(fmt.Sprintf("• Package Size Score: %d/10 (Tasks: %d, Connections: %d, Variables: %d)\n",
		structuralScore, len(pkg.Executables.Tasks), len(pkg.ConnectionMgr.Connections), len(pkg.Variables.Vars)))
	result.WriteString(fmt.Sprintf("• Control Flow Complexity: %d/10 (Precedence Constraints: %d)\n",
		calculateControlFlowComplexity(pkg), len(pkg.PrecedenceConstraints.Constraints)))

	// Script Complexity Metrics
	result.WriteString("\n📜 Script Complexity:\n")
	scriptMetrics := analyzeScriptComplexity(pkg.Executables.Tasks)
	result.WriteString(fmt.Sprintf("• Script Tasks: %d\n", scriptMetrics.ScriptTaskCount))
	result.WriteString(fmt.Sprintf("• Total Script Lines: %d\n", scriptMetrics.TotalLines))
	result.WriteString(fmt.Sprintf("• Average Script Complexity: %.1f/10\n", scriptMetrics.AverageComplexity))
	if scriptMetrics.ScriptTaskCount > 0 {
		result.WriteString(fmt.Sprintf("• Script Quality Score: %d/10\n", scriptMetrics.QualityScore))
	}

	// Expression Complexity Metrics
	result.WriteString("\n🔍 Expression Complexity:\n")
	expressionMetrics := analyzeExpressionComplexity(pkg)
	result.WriteString(fmt.Sprintf("• Total Expressions: %d\n", expressionMetrics.TotalExpressions))
	result.WriteString(fmt.Sprintf("• Average Expression Length: %.1f characters\n", expressionMetrics.AverageLength))
	result.WriteString(fmt.Sprintf("• Expression Complexity Score: %d/10\n", expressionMetrics.ComplexityScore))

	// Variable Usage Metrics
	result.WriteString("\n📊 Variable Usage:\n")
	variableMetrics := analyzeVariableUsage(pkg)
	result.WriteString(fmt.Sprintf("• Total Variables: %d\n", variableMetrics.TotalVariables))
	result.WriteString(fmt.Sprintf("• Variables with Expressions: %d\n", variableMetrics.ExpressionsCount))
	result.WriteString(fmt.Sprintf("• Variable Usage Score: %d/10\n", variableMetrics.UsageScore))

	// Overall Maintainability Score
	result.WriteString("\n🎯 Overall Maintainability Score:\n")
	overallScore := calculateOverallScore(structuralScore, scriptMetrics.QualityScore, expressionMetrics.ComplexityScore, variableMetrics.UsageScore)
	result.WriteString(fmt.Sprintf("• Composite Score: %d/10\n", overallScore))
	result.WriteString(fmt.Sprintf("• Rating: %s\n", getMaintainabilityRating(overallScore)))

	// Recommendations
	result.WriteString("\n💡 Recommendations:\n")
	addQualityRecommendations(&result, overallScore, structuralScore, scriptMetrics, expressionMetrics, variableMetrics)

	analysisResult := formatter.CreateAnalysisResult("Code Quality Metrics Analysis", filePath, result.String(), nil)
	return mcp.NewToolResultText(formatter.FormatAnalysisResult(analysisResult, format)), nil
}

// isKeyProperty checks if a property name is considered a key property for analysis
func isKeyProperty(propName string) bool {
	keyProps := []string{
		"SqlCommand", "TableOrViewName", "FileName", "ConnectionString",
		"Expression", "SortKeyPosition", "AggregationType", "Operation",
	}
	for _, key := range keyProps {
		if propName == key {
			return true
		}
	}
	return false
}

// resolveVariableExpressions resolves variable expressions in a value string
func resolveVariableExpressions(value string, variables []types.Variable, maxDepth int) string {
	if maxDepth <= 0 {
		return value
	}

	re := regexp.MustCompile(`@[a-zA-Z_][a-zA-Z0-9_]*`)
	result := re.ReplaceAllStringFunc(value, func(match string) string {
		varValue := findVariableValue(match[1:], variables) // Remove @ prefix
		if varValue != "" && varValue != match {
			// Recursively resolve nested variables
			return resolveVariableExpressions(varValue, variables, maxDepth-1)
		}
		return match
	})

	return result
}

// findVariableValue finds the value of a variable by name
func findVariableValue(name string, variables []types.Variable) string {
	for _, variable := range variables {
		if variable.Name == name {
			return variable.Value
		}
	}
	return ""
}

// getTaskType determines the type of a task based on its properties
func getTaskType(task types.Task) string {
	if task.CreationName != "" {
		switch task.CreationName {
		case "Microsoft.SqlServer.Dts.Tasks.ExecuteSQLTask.ExecuteSQLTask, Microsoft.SqlServer.SQLTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Execute SQL Task"
		case "Microsoft.SqlServer.Dts.Tasks.BulkInsertTask.BulkInsertTask, Microsoft.SqlServer.BulkInsertTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Bulk Insert Task"
		case "Microsoft.SqlServer.Dts.Tasks.DataFlowTask.DataFlowTask, Microsoft.SqlServer.DataFlowTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Data Flow Task"
		case "Microsoft.SqlServer.Dts.Tasks.FileSystemTask.FileSystemTask, Microsoft.SqlServer.FileSystemTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "File System Task"
		case "Microsoft.SqlServer.Dts.Tasks.ScriptTask.ScriptTask, Microsoft.SqlServer.ScriptTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Script Task"
		case "Microsoft.SqlServer.Dts.Tasks.SendMailTask.SendMailTask, Microsoft.SqlServer.SendMailTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Send Mail Task"
		case "Microsoft.SqlServer.Dts.Tasks.ExecuteProcessTask.ExecuteProcessTask, Microsoft.SqlServer.ExecuteProcessTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Execute Process Task"
		case "Microsoft.SqlServer.Dts.Tasks.WebServiceTask.WebServiceTask, Microsoft.SqlServer.WebServiceTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Web Service Task"
		case "Microsoft.SqlServer.Dts.Tasks.WmiTask.WmiTask, Microsoft.SqlServer.WmiTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "WMI Task"
		case "Microsoft.SqlServer.Dts.Tasks.XmlTask.XmlTask, Microsoft.SqlServer.XmlTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "XML Task"
		case "Microsoft.SqlServer.Dts.Tasks.TransferObjectsTask.TransferObjectsTask, Microsoft.SqlServer.TransferObjectsTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Transfer Objects Task"
		case "Microsoft.SqlServer.Dts.Tasks.MessageQueueTask.MessageQueueTask, Microsoft.SqlServer.MessageQueueTask, Version=14.0.0.0, Culture=neutral, PublicKeyToken=89845dcd8080cc91":
			return "Message Queue Task"
		default:
			// Try to extract a readable name from the CreationName
			if strings.Contains(task.CreationName, ".") {
				parts := strings.Split(task.CreationName, ".")
				if len(parts) > 1 {
					className := parts[len(parts)-2]
					return strings.TrimSuffix(className, "Task")
				}
			}
			return "Unknown Task"
		}
	}
	return "Unknown Task"
}

// PerformanceProperty represents a performance-related property
type PerformanceProperty struct {
	Name           string
	Value          string
	Recommendation string
}

// extractPerformanceProperties extracts performance-related properties from a property list
func extractPerformanceProperties(properties []types.Property, category string) []PerformanceProperty {
	var perfProps []PerformanceProperty

	perfPropNames := map[string]string{
		"DefaultBufferSize":        "Buffer size in bytes (default: 10MB)",
		"DefaultBufferMaxRows":     "Maximum rows per buffer (default: 10,000)",
		"EngineThreads":            "Number of engine threads for parallel execution",
		"MaxConcurrentExecutables": "Maximum concurrent executables",
		"BLOBTempStoragePath":      "Temporary storage path for BLOB data",
		"BufferTempStoragePath":    "Temporary storage path for buffer data",
		"AutoAdjustBufferSize":     "Automatically adjust buffer size",
	}

	for _, prop := range properties {
		if _, exists := perfPropNames[prop.Name]; exists {
			rec := ""
			switch prop.Name {
			case "DefaultBufferSize":
				if val, err := strconv.Atoi(prop.Value); err == nil && val < 10485760 { // 10MB
					rec = "Consider increasing to 10MB+ for large datasets"
				}
			case "DefaultBufferMaxRows":
				if val, err := strconv.Atoi(prop.Value); err == nil && val < 10000 {
					rec = "Consider increasing to 10,000-100,000 based on row size"
				}
			case "EngineThreads":
				if val, err := strconv.Atoi(prop.Value); err == nil && val < 2 {
					rec = "Consider increasing to 2-4 per CPU core for parallel processing"
				}
			}

			perfProps = append(perfProps, PerformanceProperty{
				Name:           prop.Name,
				Value:          prop.Value,
				Recommendation: rec,
			})
		}
	}

	return perfProps
}

// extractComponentPerformanceProperties extracts performance properties from data flow components
func extractComponentPerformanceProperties(component types.DataFlowComponent) []PerformanceProperty {
	var perfProps []PerformanceProperty

	// Check component properties for performance-related settings
	for _, prop := range component.ObjectData.PipelineComponent.Properties.Properties {
		switch prop.Name {
		case "CommandTimeout":
			perfProps = append(perfProps, PerformanceProperty{
				Name:  "Command Timeout",
				Value: prop.Value,
			})
		case "BatchSize":
			perfProps = append(perfProps, PerformanceProperty{
				Name:  "Batch Size",
				Value: prop.Value,
			})
		case "RowsPerBatch":
			perfProps = append(perfProps, PerformanceProperty{
				Name:  "Rows Per Batch",
				Value: prop.Value,
			})
		case "MaximumInsertCommitSize":
			perfProps = append(perfProps, PerformanceProperty{
				Name:  "Maximum Insert Commit Size",
				Value: prop.Value,
			})
		}
	}

	return perfProps
}

// isDataFlowTask checks if a task is a data flow task
func isDataFlowTask(task types.Task) bool {
	return strings.Contains(task.CreationName, "DataFlowTask")
}

// ScriptComplexityMetrics represents script complexity analysis results
type ScriptComplexityMetrics struct {
	ScriptTaskCount   int
	TotalLines        int
	AverageComplexity float64
	QualityScore      int
}

// ExpressionComplexityMetrics represents expression complexity analysis results
type ExpressionComplexityMetrics struct {
	TotalExpressions int
	AverageLength    float64
	ComplexityScore  int
}

// VariableUsageMetrics represents variable usage analysis results
type VariableUsageMetrics struct {
	TotalVariables   int
	ExpressionsCount int
	UsageScore       int
}

// calculateStructuralComplexity calculates structural complexity score
func calculateStructuralComplexity(pkg types.SSISPackage) int {
	taskCount := len(pkg.Executables.Tasks)

	// Simple scoring based on size
	sizeScore := 0
	if taskCount <= 5 {
		sizeScore = 10
	} else if taskCount <= 15 {
		sizeScore = 7
	} else if taskCount <= 30 {
		sizeScore = 4
	} else {
		sizeScore = 1
	}

	return sizeScore
}

// calculateControlFlowComplexity calculates control flow complexity score
func calculateControlFlowComplexity(pkg types.SSISPackage) int {
	constraintCount := len(pkg.PrecedenceConstraints.Constraints)

	// Score based on number of precedence constraints
	if constraintCount <= 5 {
		return 10
	} else if constraintCount <= 15 {
		return 7
	} else if constraintCount <= 30 {
		return 4
	} else {
		return 1
	}
}

// analyzeScriptComplexity analyzes script complexity in tasks
func analyzeScriptComplexity(tasks []types.Task) ScriptComplexityMetrics {
	metrics := ScriptComplexityMetrics{}

	for _, task := range tasks {
		if strings.Contains(task.CreationName, "ScriptTask") {
			metrics.ScriptTaskCount++

			// Look for script code in properties
			for _, prop := range task.Properties {
				if prop.Name == "ReadOnlyVariables" || prop.Name == "ReadWriteVariables" || prop.Name == "ScriptCode" {
					if prop.Value != "" {
						lines := strings.Split(prop.Value, "\n")
						metrics.TotalLines += len(lines)
					}
				}
			}
		}
	}

	if metrics.ScriptTaskCount > 0 {
		metrics.AverageComplexity = float64(metrics.TotalLines) / float64(metrics.ScriptTaskCount)
		// Quality score based on average complexity
		if metrics.AverageComplexity <= 10 {
			metrics.QualityScore = 10
		} else if metrics.AverageComplexity <= 25 {
			metrics.QualityScore = 7
		} else if metrics.AverageComplexity <= 50 {
			metrics.QualityScore = 4
		} else {
			metrics.QualityScore = 1
		}
	} else {
		metrics.QualityScore = 10 // No scripts = good
	}

	return metrics
}

// analyzeExpressionComplexity analyzes expression complexity
func analyzeExpressionComplexity(pkg types.SSISPackage) ExpressionComplexityMetrics {
	metrics := ExpressionComplexityMetrics{}

	// Check task properties for expressions
	for _, task := range pkg.Executables.Tasks {
		for _, prop := range task.Properties {
			if strings.Contains(prop.Value, "@") || strings.Contains(prop.Value, "ISNULL") || strings.Contains(prop.Value, "LEN") {
				metrics.TotalExpressions++
				metrics.AverageLength += float64(len(prop.Value))
			}
		}
	}

	// Check variable expressions
	for _, variable := range pkg.Variables.Vars {
		if variable.Expression != "" {
			metrics.TotalExpressions++
			metrics.AverageLength += float64(len(variable.Expression))
		}
	}

	if metrics.TotalExpressions > 0 {
		metrics.AverageLength /= float64(metrics.TotalExpressions)
	}

	// Complexity score based on expression count and average length
	if metrics.TotalExpressions <= 5 && metrics.AverageLength <= 50 {
		metrics.ComplexityScore = 10
	} else if metrics.TotalExpressions <= 15 && metrics.AverageLength <= 100 {
		metrics.ComplexityScore = 7
	} else if metrics.TotalExpressions <= 30 && metrics.AverageLength <= 200 {
		metrics.ComplexityScore = 4
	} else {
		metrics.ComplexityScore = 1
	}

	return metrics
}

// analyzeVariableUsage analyzes variable usage patterns
func analyzeVariableUsage(pkg types.SSISPackage) VariableUsageMetrics {
	metrics := VariableUsageMetrics{
		TotalVariables: len(pkg.Variables.Vars),
	}

	for _, variable := range pkg.Variables.Vars {
		if variable.Expression != "" {
			metrics.ExpressionsCount++
		}
	}

	// Usage score based on expression usage
	if metrics.TotalVariables == 0 {
		metrics.UsageScore = 10 // No variables = simple package
	} else {
		expressionRatio := float64(metrics.ExpressionsCount) / float64(metrics.TotalVariables)
		if expressionRatio >= 0.5 {
			metrics.UsageScore = 10 // Good use of expressions
		} else if expressionRatio >= 0.2 {
			metrics.UsageScore = 7
		} else {
			metrics.UsageScore = 4 // Limited expression usage
		}
	}

	return metrics
}

// calculateOverallScore calculates overall maintainability score
func calculateOverallScore(structural, script, expression, variable int) int {
	return (structural + script + expression + variable) / 4
}

// getMaintainabilityRating returns a rating string based on score
func getMaintainabilityRating(score int) string {
	switch {
	case score >= 9:
		return "Excellent"
	case score >= 7:
		return "Good"
	case score >= 5:
		return "Fair"
	case score >= 3:
		return "Poor"
	default:
		return "Critical"
	}
}

// addQualityRecommendations adds quality recommendations to the result
func addQualityRecommendations(result *strings.Builder, overallScore int, structuralScore int, scriptMetrics ScriptComplexityMetrics, expressionMetrics ExpressionComplexityMetrics, variableMetrics VariableUsageMetrics) {
	if overallScore >= 7 {
		result.WriteString("âœ… Package quality is good. Continue following best practices.\n")
	} else {
		result.WriteString("âš ï¸ Package quality needs improvement:\n")

		if structuralScore < 5 {
			result.WriteString("â€¢ Consider breaking down large packages into smaller, focused packages\n")
		}

		if scriptMetrics.QualityScore < 5 {
			result.WriteString("â€¢ Review and refactor complex script tasks\n")
			result.WriteString("â€¢ Consider moving complex logic to custom components\n")
		}

		if expressionMetrics.ComplexityScore < 5 {
			result.WriteString("â€¢ Simplify complex expressions using variables or derived columns\n")
			result.WriteString("â€¢ Consider using script components for very complex logic\n")
		}

		if variableMetrics.UsageScore < 5 {
			result.WriteString("â€¢ Increase use of expressions in variables for dynamic behavior\n")
			result.WriteString("â€¢ Review variable naming and organization\n")
		}
	}

	result.WriteString("\nðŸ”¨ General Recommendations:\n")
	result.WriteString("â€¢ Use consistent naming conventions\n")
	result.WriteString("â€¢ Add proper documentation and annotations\n")
	result.WriteString("â€¢ Implement proper error handling and logging\n")
	result.WriteString("â€¢ Use package parameters instead of hardcoded values\n")
	result.WriteString("â€¢ Regularly review and refactor complex packages\n")
}

// maskSensitiveValue masks sensitive values for display
func maskSensitiveValue(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

// analyzeConnectionSecurity analyzes connection managers for security issues
func analyzeConnectionSecurity(connections []types.Connection) []string {
	var issues []string

	for _, conn := range connections {
		connStr := conn.ObjectData.ConnectionMgr.ConnectionString
		if connStr == "" {
			connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
		}
		connStrLower := strings.ToLower(connStr)
		// Check for password patterns in connection string
		if strings.Contains(connStrLower, "password=") || strings.Contains(connStrLower, "pwd=") ||
			strings.Contains(connStrLower, "user id=") || strings.Contains(connStrLower, "uid=") {
			issues = append(issues, fmt.Sprintf("Connection '%s' contains potential credentials in connection string", conn.Name))
		}
		// Check for integrated security=false which might indicate SQL auth
		if strings.Contains(connStrLower, "integrated security=false") ||
			strings.Contains(connStrLower, "trusted_connection=false") {
			issues = append(issues, fmt.Sprintf("Connection '%s' uses SQL Server authentication (consider Windows auth)", conn.Name))
		}
	}

	return issues
}

// analyzeVariableSecurity analyzes variables for sensitive data
func analyzeVariableSecurity(variables []types.Variable) []string {
	var issues []string

	sensitiveVarPatterns := []string{"password", "pwd", "secret", "key", "token", "credential"}

	for _, variable := range variables {
		varNameLower := strings.ToLower(variable.Name)
		for _, pattern := range sensitiveVarPatterns {
			if strings.Contains(varNameLower, pattern) {
				// Check if the value looks like a real credential (not empty, not expression)
				if variable.Value != "" && !strings.HasPrefix(variable.Value, "@") {
					issues = append(issues, fmt.Sprintf("Variable '%s' contains sensitive data: %s", variable.Name, maskSensitiveValue(variable.Value)))
				}
			}
		}
	}

	return issues
}

// analyzeScriptSecurity analyzes script tasks for hardcoded credentials
func analyzeScriptSecurity(tasks []types.Task) []string {
	var issues []string

	for _, task := range tasks {
		if getTaskType(task) == "Script Task" {
			// Check script project for hardcoded strings
			scriptCode := task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode
			if scriptCode != "" {
				scriptLower := strings.ToLower(scriptCode)
				// Check for common credential patterns in script code
				if strings.Contains(scriptLower, "password") || strings.Contains(scriptLower, "pwd") ||
					strings.Contains(scriptLower, "secret") || strings.Contains(scriptLower, "connectionstring") {
					issues = append(issues, fmt.Sprintf("Script Task '%s' may contain hardcoded credentials in script code", task.Name))
				}
			}
		}
	}

	return issues
}

// analyzeExpressionSecurity analyzes expressions for sensitive data
func analyzeExpressionSecurity(tasks []types.Task, variables []types.Variable) []string {
	var issues []string

	for _, task := range tasks {
		for _, prop := range task.Properties {
			if prop.Name == "Expression" || strings.Contains(prop.Name, "Expression") {
				expr := strings.ToLower(prop.Value)
				// Check for hardcoded sensitive data in expressions
				if strings.Contains(expr, "password") || strings.Contains(expr, "pwd") ||
					strings.Contains(expr, "secret") || strings.Contains(expr, "key") {
					issues = append(issues, fmt.Sprintf("Task '%s' expression may contain sensitive data", task.Name))
				}
			}
		}
	}

	return issues
}

// HandleDetectSecurityIssues handles security issues detection from DTSX files
func HandleDetectSecurityIssues(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("🔒 Security Issues Analysis:\n\n")

	issuesFound := false

	// Check connection managers for hardcoded credentials
	result.WriteString("🔗 Connection Managers:\n")
	connIssues := analyzeConnectionSecurity(pkg.ConnectionMgr.Connections)
	if len(connIssues) > 0 {
		issuesFound = true
		for _, issue := range connIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in connection managers.\n")
	}
	result.WriteString("\n")

	// Check variables for sensitive data
	result.WriteString("📊 Variables:\n")
	varIssues := analyzeVariableSecurity(pkg.Variables.Vars)
	if len(varIssues) > 0 {
		issuesFound = true
		for _, issue := range varIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in variables.\n")
	}
	result.WriteString("\n")

	// Check script tasks for hardcoded credentials
	result.WriteString("📜 Script Tasks:\n")
	scriptIssues := analyzeScriptSecurity(pkg.Executables.Tasks)
	if len(scriptIssues) > 0 {
		issuesFound = true
		for _, issue := range scriptIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in script tasks.\n")
	}
	result.WriteString("\n")

	// Check expressions for sensitive data
	result.WriteString("🧮 Expressions:\n")
	exprIssues := analyzeExpressionSecurity(pkg.Executables.Tasks, pkg.Variables.Vars)
	if len(exprIssues) > 0 {
		issuesFound = true
		for _, issue := range exprIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No security issues found in expressions.\n")
	}
	result.WriteString("\n")

	if !issuesFound {
		result.WriteString("✅ No security issues detected in this package.\n\n")
		result.WriteString("💡 Security Best Practices:\n")
		result.WriteString("• Use package parameters or environment variables for credentials\n")
		result.WriteString("• Avoid hardcoded passwords in connection strings\n")
		result.WriteString("• Use SSIS package protection levels for sensitive data\n")
		result.WriteString("• Consider using Azure Key Vault or similar for credential management\n")
	} else {
		result.WriteString("🚨 Security Recommendations:\n")
		result.WriteString("• Replace hardcoded credentials with parameters or expressions\n")
		result.WriteString("• Use SSIS package configurations for sensitive connection properties\n")
		result.WriteString("• Implement proper package protection and encryption\n")
		result.WriteString("• Review and audit access to sensitive data\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// HandleScanCredentials handles advanced credential scanning from DTSX files
func HandleScanCredentials(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("🔍 Advanced Credential Scanning Report:\n\n")

	issuesFound := false

	// Comprehensive credential pattern matching
	credentialPatterns := []struct {
		name     string
		patterns []string
		category string
	}{
		{
			name:     "Database Credentials",
			patterns: []string{"password=", "pwd=", "user id=", "uid=", "userid=", "trusted_connection=false", "integrated security=false"},
			category: "Database",
		},
		{
			name:     "API Keys & Tokens",
			patterns: []string{"apikey=", "api_key=", "token=", "bearer=", "authorization=", "x-api-key="},
			category: "API",
		},
		{
			name:     "Cloud Credentials",
			patterns: []string{"accesskey=", "secretkey=", "accountkey=", "sharedaccesskey=", "sas=", "connectionstring="},
			category: "Cloud",
		},
		{
			name:     "Encryption Keys",
			patterns: []string{"encryptionkey=", "key=", "certificate=", "privatekey=", "publickey="},
			category: "Encryption",
		},
		{
			name:     "Personal Data",
			patterns: []string{"ssn=", "socialsecurity=", "creditcard=", "cardnumber=", "email=", "phone="},
			category: "PII",
		},
	}

	// scanConnectionCredentials performs advanced credential scanning on connection strings
	scanConnectionCredentials := func(connections []types.Connection, patterns []struct {
		name     string
		patterns []string
		category string
	}) []string {
		var issues []string

		for _, conn := range connections {
			connStr := conn.ObjectData.ConnectionMgr.ConnectionString
			if connStr == "" {
				connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
			}
			connStrLower := strings.ToLower(connStr)

			for _, patternGroup := range patterns {
				for _, pattern := range patternGroup.patterns {
					if strings.Contains(connStrLower, strings.ToLower(pattern)) {
						issues = append(issues, fmt.Sprintf("[%s] Connection '%s' contains %s pattern: %s",
							patternGroup.category, conn.Name, patternGroup.name, pattern))
					}
				}
			}
		}

		return issues
	}

	// scanVariableCredentials performs advanced credential scanning on variables
	scanVariableCredentials := func(variables []types.Variable, patterns []struct {
		name     string
		patterns []string
		category string
	}) []string {
		var issues []string

		for _, variable := range variables {
			varNameLower := strings.ToLower(variable.Name)
			varValueLower := strings.ToLower(variable.Value)

			for _, patternGroup := range patterns {
				for _, pattern := range patternGroup.patterns {
					if strings.Contains(varNameLower, strings.ToLower(strings.TrimSuffix(pattern, "="))) ||
						strings.Contains(varValueLower, strings.ToLower(pattern)) {
						if variable.Value != "" && !strings.HasPrefix(variable.Value, "@") {
							issues = append(issues, fmt.Sprintf("[%s] Variable '%s' contains %s pattern",
								patternGroup.category, variable.Name, patternGroup.name))
						}
					}
				}
			}
		}

		return issues
	}

	// scanScriptCredentials performs advanced credential scanning on script tasks
	scanScriptCredentials := func(tasks []types.Task, patterns []struct {
		name     string
		patterns []string
		category string
	}) []string {
		var issues []string

		for _, task := range tasks {
			if getTaskType(task) == "Script Task" {
				scriptCode := task.ObjectData.ScriptTask.ScriptTaskData.ScriptProject.ScriptCode
				if scriptCode != "" {
					scriptLower := strings.ToLower(scriptCode)

					for _, patternGroup := range patterns {
						for _, pattern := range patternGroup.patterns {
							if strings.Contains(scriptLower, strings.ToLower(pattern)) {
								issues = append(issues, fmt.Sprintf("[%s] Script Task '%s' contains %s pattern in code",
									patternGroup.category, task.Name, patternGroup.name))
							}
						}
					}
				}
			}
		}

		return issues
	}

	// scanExpressionCredentials performs advanced credential scanning on expressions
	scanExpressionCredentials := func(tasks []types.Task, patterns []struct {
		name     string
		patterns []string
		category string
	}) []string {
		var issues []string

		for _, task := range tasks {
			for _, prop := range task.Properties {
				if prop.Name == "Expression" || strings.Contains(prop.Name, "Expression") {
					exprLower := strings.ToLower(prop.Value)

					for _, patternGroup := range patterns {
						for _, pattern := range patternGroup.patterns {
							if strings.Contains(exprLower, strings.ToLower(pattern)) {
								issues = append(issues, fmt.Sprintf("[%s] Task '%s' expression contains %s pattern",
									patternGroup.category, task.Name, patternGroup.name))
							}
						}
					}
				}
			}
		}

		return issues
	}

	// scanRawContentCredentials performs advanced credential scanning on raw XML content
	scanRawContentCredentials := func(content string, patterns []struct {
		name     string
		patterns []string
		category string
	}) []string {
		var issues []string
		contentLower := strings.ToLower(content)

		for _, patternGroup := range patterns {
			for _, pattern := range patternGroup.patterns {
				if strings.Contains(contentLower, strings.ToLower(pattern)) {
					issues = append(issues, fmt.Sprintf("[%s] Raw content contains %s pattern: %s",
						patternGroup.category, patternGroup.name, pattern))
				}
			}
		}

		return issues
	}

	// Check connection strings with advanced pattern matching
	result.WriteString("🔗 Connection String Analysis:\n")
	connIssues := scanConnectionCredentials(pkg.ConnectionMgr.Connections, credentialPatterns)
	if len(connIssues) > 0 {
		issuesFound = true
		for _, issue := range connIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in connection strings.\n")
	}
	result.WriteString("\n")

	// Check variables for credential patterns
	result.WriteString("📊 Variable Analysis:\n")
	varIssues := scanVariableCredentials(pkg.Variables.Vars, credentialPatterns)
	if len(varIssues) > 0 {
		issuesFound = true
		for _, issue := range varIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in variables.\n")
	}
	result.WriteString("\n")

	// Check script tasks for embedded credentials
	result.WriteString("📜 Script Task Analysis:\n")
	scriptIssues := scanScriptCredentials(pkg.Executables.Tasks, credentialPatterns)
	if len(scriptIssues) > 0 {
		issuesFound = true
		for _, issue := range scriptIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in script tasks.\n")
	}
	result.WriteString("\n")

	// Check expressions for credential patterns
	result.WriteString("🧮 Expression Analysis:\n")
	exprIssues := scanExpressionCredentials(pkg.Executables.Tasks, credentialPatterns)
	if len(exprIssues) > 0 {
		issuesFound = true
		for _, issue := range exprIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in expressions.\n")
	}
	result.WriteString("\n")

	// Check raw XML content for additional patterns
	result.WriteString("📄 Raw Content Analysis:\n")
	rawIssues := scanRawContentCredentials(string(data), credentialPatterns)
	if len(rawIssues) > 0 {
		issuesFound = true
		for _, issue := range rawIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No credential patterns detected in raw content.\n")
	}
	result.WriteString("\n")

	if !issuesFound {
		result.WriteString("✅ No credential patterns detected in this package.\n\n")
		result.WriteString("💡 Credential Security Best Practices:\n")
		result.WriteString("• Use SSIS parameters for all credentials\n")
		result.WriteString("• Store sensitive data in environment variables or secure stores\n")
		result.WriteString("• Use Azure Key Vault or similar for cloud credentials\n")
		result.WriteString("• Implement proper package protection levels\n")
		result.WriteString("• Regularly rotate credentials and keys\n")
	} else {
		result.WriteString("🚨 Critical Security Recommendations:\n")
		result.WriteString("• Immediately replace all hardcoded credentials with secure alternatives\n")
		result.WriteString("• Implement proper encryption for sensitive package elements\n")
		result.WriteString("• Use SSIS package configurations or parameters\n")
		result.WriteString("• Consider using managed identity or service principals\n")
		result.WriteString("• Audit and monitor access to sensitive data\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// HandleDetectEncryption handles encryption detection and recommendations from DTSX files
func HandleDetectEncryption(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("🔐 Encryption Detection & Recommendations:\n\n")

	// analyzeConnectionEncryption checks for encryption issues in connection strings
	analyzeConnectionEncryption := func(connections []types.Connection) []string {
		var issues []string

		for _, conn := range connections {
			connStr := conn.ObjectData.ConnectionMgr.ConnectionString
			if connStr == "" {
				connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
			}
			connStrLower := strings.ToLower(connStr)

			// Check for unencrypted connections
			if strings.Contains(connStrLower, "server=") || strings.Contains(connStrLower, "data source=") {
				if !strings.Contains(connStrLower, "encrypt=true") && !strings.Contains(connStrLower, "ssl") {
					issues = append(issues, fmt.Sprintf("Connection '%s' may not be using encryption", conn.Name))
				}
			}

			// Check for HTTP connections without SSL
			if strings.Contains(connStrLower, "http://") && !strings.Contains(connStrLower, "https://") {
				issues = append(issues, fmt.Sprintf("Connection '%s' uses unencrypted HTTP", conn.Name))
			}
		}

		return issues
	}

	// analyzeSensitiveDataHandling checks for sensitive data handling issues
	analyzeSensitiveDataHandling := func(pkg types.SSISPackage) []string {
		var issues []string

		// Check for sensitive data in logs
		for _, task := range pkg.Executables.Tasks {
			if getTaskType(task) == "Execute SQL Task" {
				for _, prop := range task.Properties {
					if strings.Contains(strings.ToLower(prop.Name), "sqlstatement") {
						sqlLower := strings.ToLower(prop.Value)
						if strings.Contains(sqlLower, "password") || strings.Contains(sqlLower, "ssn") ||
							strings.Contains(sqlLower, "credit") || strings.Contains(sqlLower, "salary") {
							issues = append(issues, fmt.Sprintf("Execute SQL Task '%s' may be logging sensitive data", task.Name))
						}
					}
				}
			}
		}

		// Check for unencrypted flat files
		for _, conn := range pkg.ConnectionMgr.Connections {
			connStr := conn.ObjectData.ConnectionMgr.ConnectionString
			if connStr == "" {
				connStr = conn.ObjectData.MsmqConnMgr.ConnectionString
			}
			connStrLower := strings.ToLower(connStr)
			// Check for file paths in connection strings (likely flat files)
			if strings.Contains(connStrLower, ".csv") || strings.Contains(connStrLower, ".txt") ||
				strings.Contains(connStrLower, ".dat") || strings.Contains(connStrLower, "\\") {
				issues = append(issues, fmt.Sprintf("File-based connection '%s' detected - ensure file encryption if sensitive data", conn.Name))
			}
		}

		return issues
	}

	// Check package protection level
	result.WriteString("📦 Package Protection Level:\n")
	protectionLevel := "Not specified"
	for _, prop := range pkg.Properties {
		if prop.Name == "ProtectionLevel" {
			protectionLevel = prop.Value
			break
		}
	}

	switch protectionLevel {
	case "0", "DontSaveSensitive":
		result.WriteString("⚠️  Protection Level: DontSaveSensitive - Sensitive data will not be saved\n")
		result.WriteString("   This is generally secure but requires runtime parameter input\n")
	case "1", "EncryptSensitiveWithUserKey":
		result.WriteString("⚠️  Protection Level: EncryptSensitiveWithUserKey - Uses current user key\n")
		result.WriteString("   This may cause issues when deploying to different users/machines\n")
	case "2", "EncryptSensitiveWithPassword":
		result.WriteString("✅ Protection Level: EncryptSensitiveWithPassword - Uses password protection\n")
		result.WriteString("   Good for deployment but requires secure password management\n")
	case "3", "EncryptAllWithPassword":
		result.WriteString("✅ Protection Level: EncryptAllWithPassword - Encrypts entire package\n")
		result.WriteString("   Maximum security but requires password for execution\n")
	case "4", "EncryptAllWithUserKey":
		result.WriteString("⚠️  Protection Level: EncryptAllWithUserKey - Uses current user key for all data\n")
		result.WriteString("   May cause deployment issues across different users/machines\n")
	default:
		result.WriteString("❓ Protection Level: Unknown or not set\n")
		result.WriteString("   Consider setting an appropriate protection level\n")
	}
	result.WriteString("\n")

	// Check for encrypted connection strings
	result.WriteString("🔗 Connection Encryption:\n")
	encryptionIssues := analyzeConnectionEncryption(pkg.ConnectionMgr.Connections)
	if len(encryptionIssues) > 0 {
		for _, issue := range encryptionIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No encryption issues detected in connections.\n")
	}
	result.WriteString("\n")

	// Check for sensitive data handling
	result.WriteString("🔒 Sensitive Data Handling:\n")
	sensitiveDataIssues := analyzeSensitiveDataHandling(pkg)
	if len(sensitiveDataIssues) > 0 {
		for _, issue := range sensitiveDataIssues {
			result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
		}
	} else {
		result.WriteString("No sensitive data handling issues detected.\n")
	}
	result.WriteString("\n")

	// Provide encryption recommendations
	result.WriteString("💡 Encryption Recommendations:\n")
	result.WriteString("• Use EncryptSensitiveWithPassword for most scenarios\n")
	result.WriteString("• Consider EncryptAllWithPassword for maximum security\n")
	result.WriteString("• Use Azure Key Vault for cloud deployments\n")
	result.WriteString("• Implement proper certificate management\n")
	result.WriteString("• Use SSL/TLS for all external connections\n")
	result.WriteString("• Consider column-level encryption for sensitive data\n")
	result.WriteString("• Implement proper key rotation policies\n")

	return mcp.NewToolResultText(result.String()), nil
}

// HandleCheckCompliance handles compliance checking for various standards from DTSX files
func HandleCheckCompliance(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	complianceStandard := request.GetString("compliance_standard", "all")

	// Resolve the file path against the package directory
	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	// Remove namespace prefixes for easier parsing
	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("⚠️  Compliance Check Report:\n")
	result.WriteString(fmt.Sprintf("Standard: %s\n\n", strings.ToUpper(complianceStandard)))

	issuesFound := false

	// checkGDPRCompliance checks for GDPR compliance issues
	checkGDPRCompliance := func(pkg types.SSISPackage, content string) []string {
		var issues []string
		contentLower := strings.ToLower(content)

		// GDPR-specific patterns
		gdprPatterns := []string{
			"email", "emailaddress", "firstname", "lastname", "fullname", "dateofbirth", "dob",
			"address", "phonenumber", "phone", "ipaddress", "ip", "location", "geolocation",
			"cookie", "tracking", "consent", "gdpr", "personaldata", "pii",
		}

		for _, pattern := range gdprPatterns {
			if strings.Contains(contentLower, pattern) {
				issues = append(issues, fmt.Sprintf("Potential GDPR-sensitive data pattern detected: %s", pattern))
			}
		}

		// Check for data retention patterns
		if strings.Contains(contentLower, "delete") || strings.Contains(contentLower, "truncate") {
			issues = append(issues, "Data deletion operations detected - ensure GDPR compliance for data retention")
		}

		return issues
	}

	// checkHIPAACompliance checks for HIPAA compliance issues
	checkHIPAACompliance := func(pkg types.SSISPackage, content string) []string {
		var issues []string
		contentLower := strings.ToLower(content)

		// HIPAA-specific patterns
		hipaaPatterns := []string{
			"medicalrecord", "healthrecord", "diagnosis", "treatment", "medication",
			"patientid", "patient_id", "medicalid", "healthid", "phi", "protectedhealth",
			"insurance", "policy", "claim", "billing", "hipaa",
		}

		for _, pattern := range hipaaPatterns {
			if strings.Contains(contentLower, pattern) {
				issues = append(issues, fmt.Sprintf("Potential HIPAA-sensitive data pattern detected: %s", pattern))
			}
		}

		// Check for audit logging
		hasAudit := false
		for _, task := range pkg.Executables.Tasks {
			if getTaskType(task) == "Audit" || strings.Contains(strings.ToLower(task.Name), "audit") {
				hasAudit = true
				break
			}
		}
		if !hasAudit {
			issues = append(issues, "No audit logging detected - HIPAA requires audit trails for PHI access")
		}

		return issues
	}

	// checkPCICompliance checks for PCI DSS compliance issues
	checkPCICompliance := func(pkg types.SSISPackage, content string) []string {
		var issues []string
		contentLower := strings.ToLower(content)

		// PCI DSS-specific patterns
		pciPatterns := []string{
			"creditcard", "cardnumber", "cvv", "cvc", "expiration", "expiry",
			"cardholder", "pci", "payment", "transaction", "merchant",
		}

		for _, pattern := range pciPatterns {
			if strings.Contains(contentLower, pattern) {
				issues = append(issues, fmt.Sprintf("Potential PCI DSS-sensitive data pattern detected: %s", pattern))
			}
		}

		// Check for encryption of card data
		if strings.Contains(contentLower, "creditcard") || strings.Contains(contentLower, "cardnumber") {
			if !strings.Contains(contentLower, "encrypt") && !strings.Contains(contentLower, "mask") {
				issues = append(issues, "Card data detected without apparent encryption or masking")
			}
		}

		return issues
	}

	// checkGeneralDataProtection checks for general data protection issues
	checkGeneralDataProtection := func(pkg types.SSISPackage, content string) []string {
		var issues []string
		contentLower := strings.ToLower(content)

		// General data protection patterns
		generalPatterns := []string{
			"password", "pwd", "secret", "key", "token", "apikey", "api_key",
			"confidential", "sensitive", "private", "internal",
		}

		for _, pattern := range generalPatterns {
			if strings.Contains(contentLower, pattern) {
				issues = append(issues, fmt.Sprintf("Potential sensitive data pattern detected: %s", pattern))
			}
		}

		// Check for data masking or anonymization
		hasMasking := strings.Contains(contentLower, "mask") || strings.Contains(contentLower, "anonymize") ||
			strings.Contains(contentLower, "pseudonymize")

		if !hasMasking && (strings.Contains(contentLower, "personal") || strings.Contains(contentLower, "sensitive")) {
			issues = append(issues, "Sensitive data handling detected without apparent masking/anonymization")
		}

		return issues
	}

	// GDPR Compliance Patterns
	if complianceStandard == "gdpr" || complianceStandard == "all" {
		result.WriteString("🇪🇺 GDPR Compliance Analysis:\n")
		gdprIssues := checkGDPRCompliance(pkg, string(data))
		if len(gdprIssues) > 0 {
			issuesFound = true
			for _, issue := range gdprIssues {
				result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
			}
		} else {
			result.WriteString("No GDPR compliance issues detected.\n")
		}
		result.WriteString("\n")
	}

	// HIPAA Compliance Patterns
	if complianceStandard == "hipaa" || complianceStandard == "all" {
		result.WriteString("🏥 HIPAA Compliance Analysis:\n")
		hipaaIssues := checkHIPAACompliance(pkg, string(data))
		if len(hipaaIssues) > 0 {
			issuesFound = true
			for _, issue := range hipaaIssues {
				result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
			}
		} else {
			result.WriteString("No HIPAA compliance issues detected.\n")
		}
		result.WriteString("\n")
	}

	// PCI DSS Compliance Patterns
	if complianceStandard == "pci" || complianceStandard == "all" {
		result.WriteString("💳 PCI DSS Compliance Analysis:\n")
		pciIssues := checkPCICompliance(pkg, string(data))
		if len(pciIssues) > 0 {
			issuesFound = true
			for _, issue := range pciIssues {
				result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
			}
		} else {
			result.WriteString("No PCI DSS compliance issues detected.\n")
		}
		result.WriteString("\n")
	}

	// General Data Protection Analysis
	if complianceStandard == "all" {
		result.WriteString("🔒 General Data Protection Analysis:\n")
		generalIssues := checkGeneralDataProtection(pkg, string(data))
		if len(generalIssues) > 0 {
			issuesFound = true
			for _, issue := range generalIssues {
				result.WriteString(fmt.Sprintf("⚠️  %s\n", issue))
			}
		} else {
			result.WriteString("No general data protection issues detected.\n")
		}
		result.WriteString("\n")
	}

	if !issuesFound {
		result.WriteString("✅ No compliance issues detected for the specified standards.\n\n")
		result.WriteString("💡 Compliance Best Practices:\n")
		result.WriteString("• Implement proper data classification and labeling\n")
		result.WriteString("• Use encryption for sensitive data at rest and in transit\n")
		result.WriteString("• Implement access controls and audit logging\n")
		result.WriteString("• Regular security assessments and penetration testing\n")
		result.WriteString("• Data minimization and purpose limitation principles\n")
		result.WriteString("• Implement proper data retention and deletion policies\n")
	} else {
		result.WriteString("🚨 Compliance Remediation Required:\n")
		result.WriteString("• Address all flagged compliance issues immediately\n")
		result.WriteString("• Implement proper data protection measures\n")
		result.WriteString("• Consult with compliance officers and legal teams\n")
		result.WriteString("• Document compliance measures and controls\n")
		result.WriteString("• Regular compliance audits and monitoring\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// HandleAnalyzeSource provides unified analysis for various SSIS source components
func HandleAnalyzeSource(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file_path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sourceType, err := request.RequireString("source_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Map source types to ComponentClassIDs
	sourceTypeMap := map[string]string{
		"ole_db":    "Microsoft.OLEDBSource",
		"ado_net":   "Microsoft.SqlServer.Dts.Pipeline.DataReaderSourceAdapter",
		"odbc":      "Microsoft.SqlServer.Dts.Pipeline.OdbcSourceAdapter",
		"flat_file": "Microsoft.SqlServer.Dts.Pipeline.FlatFileSourceAdapter",
		"excel":     "Microsoft.SqlServer.Dts.Pipeline.ExcelSourceAdapter",
		"access":    "Microsoft.SqlServer.Dts.Pipeline.AccessSourceAdapter",
		"xml":       "Microsoft.SqlServer.Dts.Pipeline.XmlSourceAdapter",
		"raw_file":  "Microsoft.SqlServer.Dts.Pipeline.RawFileSourceAdapter",
		"cdc":       "Microsoft.SqlServer.Dts.Pipeline.CdcSourceAdapter",
		"sap_bw":    "Microsoft.SqlServer.Dts.Pipeline.SapBwSourceAdapter",
	}

	componentClassID, exists := sourceTypeMap[sourceType]
	if !exists {
		return mcp.NewToolResultError(fmt.Sprintf("Unknown source type: %s. Supported types: ole_db, ado_net, odbc, flat_file, excel, access, xml, raw_file, cdc, sap_bw", sourceType)), nil
	}

	// Map source types to display names
	sourceNameMap := map[string]string{
		"ole_db":    "OLE DB Source",
		"ado_net":   "ADO.NET Source",
		"odbc":      "ODBC Source",
		"flat_file": "Flat File Source",
		"excel":     "Excel Source",
		"access":    "Access Source",
		"xml":       "XML Source",
		"raw_file":  "Raw File Source",
		"cdc":       "CDC Source",
		"sap_bw":    "SAP BW Source",
	}

	displayName := sourceNameMap[sourceType]

	resolvedPath := ResolveFilePath(filePath, packageDirectory)

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	data = []byte(strings.ReplaceAll(string(data), "DTS:", ""))
	data = []byte(strings.ReplaceAll(string(data), `xmlns="www.microsoft.com/SqlServer/Dts"`, ""))

	var pkg types.SSISPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse XML: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s Analysis:\n\n", displayName))

	found := false
	for _, task := range pkg.Executables.Tasks {
		if strings.Contains(task.CreationName, "Pipeline") {
			for _, comp := range task.ObjectData.DataFlow.Components.Components {
				if comp.ComponentClassID == componentClassID {
					found = true
					result.WriteString(fmt.Sprintf("Component: %s\n", comp.Name))
					result.WriteString(fmt.Sprintf("Description: %s\n", comp.Description))

					// Properties
					result.WriteString("Properties:\n")
					for _, prop := range comp.ObjectData.PipelineComponent.Properties.Properties {
						result.WriteString(fmt.Sprintf("  %s: %s\n", prop.Name, prop.Value))
					}

					// Output Columns
					result.WriteString("Output Columns:\n")
					for _, output := range comp.Outputs.Outputs {
						if !output.IsErrorOut {
							for _, col := range output.OutputColumns.Columns {
								result.WriteString(fmt.Sprintf("  %s (%s", col.Name, col.DataType))
								if col.Length > 0 {
									result.WriteString(fmt.Sprintf(", length=%d", col.Length))
								}
								result.WriteString(")\n")
							}
						}
					}
					result.WriteString("\n")
				}
			}
		}
	}

	if !found {
		result.WriteString(fmt.Sprintf("No %s components found in this package.\n", displayName))
	}

	return mcp.NewToolResultText(result.String()), nil
}
