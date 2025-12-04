package optimization

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MCPRUNNER/gossisMCP/pkg/types"
	"github.com/mark3labs/mcp-go/mcp"
)

// ResolveFilePath resolves a file path against the package directory
func ResolveFilePath(filePath, packageDirectory string) string {
	if strings.HasPrefix(filePath, "/") || (len(filePath) >= 3 && filePath[1:3] == ":\\") {
		return filePath
	}
	return packageDirectory + "/" + filePath
}

// getTaskType determines the type of a task based on its properties
func getTaskType(task types.Task) string {
	for _, prop := range task.Properties {
		if prop.Name == "TaskType" || prop.Name == "CreationName" {
			return prop.Value
		}
	}
	return "Unknown"
}

// isDataFlowTask checks if a task is a Data Flow Task
func isDataFlowTask(task types.Task) bool {
	taskType := getTaskType(task)
	return taskType == "SSIS.Pipeline.3" || taskType == "SSIS.Pipeline.2" || strings.Contains(taskType, "Pipeline")
}

// getComponentType extracts the component type from the ComponentClassID
func getComponentType(classID string) string {
	// Extract component type from class ID
	parts := strings.Split(classID, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return classID
}

// BufferSetting represents a buffer configuration setting
type BufferSetting struct {
	Name  string
	Value string
}

// ComponentMemory represents memory usage analysis for a component
type ComponentMemory struct {
	Component      string
	Estimate       string
	Recommendation string
}

// HandleOptimizeBufferSize analyzes and provides recommendations for buffer size optimization
func HandleOptimizeBufferSize(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
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
	result.WriteString("🔄 Buffer Size Optimization Analysis:\n\n")

	dataFlowCount := 0
	totalComponents := 0

	for _, task := range pkg.Executables.Tasks {
		if isDataFlowTask(task) {
			dataFlowCount++
			result.WriteString(fmt.Sprintf("📊 Data Flow Task: %s\n", task.Name))

			// Analyze current buffer settings
			bufferSettings := analyzeBufferSettings(task)
			if len(bufferSettings) > 0 {
				result.WriteString("  Current Buffer Settings:\n")
				for _, setting := range bufferSettings {
					result.WriteString(fmt.Sprintf("  • %s: %s\n", setting.Name, setting.Value))
				}
			} else {
				result.WriteString("  No explicit buffer settings found (using defaults).\n")
			}

			// Count components and estimate data volume
			if task.ObjectData.DataFlow.Components.Components != nil {
				componentCount := len(task.ObjectData.DataFlow.Components.Components)
				totalComponents += componentCount

				result.WriteString(fmt.Sprintf("  Components: %d\n", componentCount))

				// Estimate buffer requirements based on component types
				bufferRecommendations := generateBufferRecommendations(task.ObjectData.DataFlow.Components.Components, bufferSettings)
				if len(bufferRecommendations) > 0 {
					result.WriteString("  📈 Buffer Optimization Recommendations:\n")
					for _, rec := range bufferRecommendations {
						result.WriteString(fmt.Sprintf("    • %s\n", rec))
					}
				}
			}
			result.WriteString("\n")
		}
	}

	if dataFlowCount == 0 {
		result.WriteString("❌ No Data Flow Tasks found in this package.\n\n")
		result.WriteString("💡 Buffer optimization is only applicable to Data Flow Tasks.\n")
	} else {
		result.WriteString("🎯 Overall Buffer Optimization Summary:\n")
		result.WriteString(fmt.Sprintf("• Analyzed %d Data Flow Task(s) with %d total components\n", dataFlowCount, totalComponents))

		// General buffer optimization guidelines
		result.WriteString("\n📋 General Buffer Optimization Guidelines:\n")
		result.WriteString("• DefaultBufferSize: Start with 10MB, increase to 50MB+ for large datasets\n")
		result.WriteString("• DefaultBufferMaxRows: 10,000-100,000 rows based on row size\n")
		result.WriteString("• MaxBufferSize: Set to 100MB+ for very large datasets\n")
		result.WriteString("• MinBufferSize: Keep at 64KB unless processing very small datasets\n")
		result.WriteString("• AutoAdjustBufferSize: Enable for automatic optimization\n")
		result.WriteString("• BufferTempStoragePath: Use fast SSD storage for spill operations\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// HandleAnalyzeParallelProcessing analyzes parallel processing capabilities and provides recommendations
func HandleAnalyzeParallelProcessing(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
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
	result.WriteString("⚡ Parallel Processing Analysis:\n\n")

	// Analyze package-level parallel settings
	result.WriteString("📦 Package-Level Parallel Settings:\n")
	maxConcurrent := "Not set"
	for _, prop := range pkg.Properties {
		if prop.Name == "MaxConcurrentExecutables" {
			maxConcurrent = prop.Value
			break
		}
	}

	if maxConcurrent == "Not set" {
		result.WriteString("• MaxConcurrentExecutables: Not configured (using SSIS default)\n")
		result.WriteString("  ⚠️  Consider setting this to optimize parallel execution\n")
	} else {
		result.WriteString(fmt.Sprintf("• MaxConcurrentExecutables: %s\n", maxConcurrent))
		if val, err := strconv.Atoi(maxConcurrent); err == nil && val < 4 {
			result.WriteString("  💡 Consider increasing for better parallel utilization\n")
		}
	}
	result.WriteString("\n")

	// Analyze task dependencies and parallel execution potential
	result.WriteString("🔗 Task Execution Analysis:\n")
	taskAnalysis := analyzeTaskParallelization(pkg.Executables.Tasks)
	for _, analysis := range taskAnalysis {
		result.WriteString(fmt.Sprintf("• %s\n", analysis))
	}
	result.WriteString("\n")

	// Analyze data flow parallel processing
	result.WriteString("🔄 Data Flow Parallel Processing:\n")
	dataFlowAnalysis := analyzeDataFlowParallelization(pkg.Executables.Tasks)
	for _, analysis := range dataFlowAnalysis {
		result.WriteString(fmt.Sprintf("• %s\n", analysis))
	}
	result.WriteString("\n")

	// Container analysis for parallel execution
	result.WriteString("📁 Container Parallel Execution:\n")
	containerAnalysis := analyzeContainerParallelization(pkg.Executables.Tasks)
	for _, analysis := range containerAnalysis {
		result.WriteString(fmt.Sprintf("• %s\n", analysis))
	}
	result.WriteString("\n")

	// Performance recommendations
	result.WriteString("🚀 Parallel Processing Optimization Recommendations:\n")
	result.WriteString("• Set MaxConcurrentExecutables to 2-4 times the number of CPU cores\n")
	result.WriteString("• Use Sequence Containers to group independent tasks\n")
	result.WriteString("• Configure EngineThreads (2-10) based on data flow complexity\n")
	result.WriteString("• Avoid unnecessary precedence constraints that block parallel execution\n")
	result.WriteString("• Use For Loop containers for parallel processing of multiple files\n")
	result.WriteString("• Consider partitioning large datasets for parallel processing\n")
	result.WriteString("• Monitor CPU utilization to avoid over-subscription\n")

	return mcp.NewToolResultText(result.String()), nil
}

// HandleProfileMemoryUsage profiles memory usage patterns in SSIS packages
func HandleProfileMemoryUsage(_ context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
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
	result.WriteString("🧠 Memory Usage Profiling:\n\n")

	totalEstimatedMemory := int64(0)
	dataFlowCount := 0

	for _, task := range pkg.Executables.Tasks {
		if isDataFlowTask(task) {
			dataFlowCount++
			result.WriteString(fmt.Sprintf("📊 Data Flow Task: %s\n", task.Name))

			// Analyze buffer memory usage
			bufferMemory := estimateBufferMemoryUsage(task)
			if bufferMemory > 0 {
				result.WriteString(fmt.Sprintf("  Buffer Memory: ~%s\n", formatBytes(bufferMemory)))
				totalEstimatedMemory += bufferMemory
			}

			// Analyze component memory usage
			if task.ObjectData.DataFlow.Components.Components != nil {
				result.WriteString("  Component Memory Analysis:\n")
				componentMemory := analyzeComponentMemoryUsage(task.ObjectData.DataFlow.Components.Components)
				for _, compMem := range componentMemory {
					result.WriteString(fmt.Sprintf("    • %s: %s\n", compMem.Component, compMem.Estimate))
					if compMem.Recommendation != "" {
						result.WriteString(fmt.Sprintf("      💡 %s\n", compMem.Recommendation))
					}
				}
			}

			// Check for memory-intensive operations
			memoryIssues := detectMemoryIntensiveOperations(task)
			if len(memoryIssues) > 0 {
				result.WriteString("  ⚠️  Memory-Intensive Operations:\n")
				for _, issue := range memoryIssues {
					result.WriteString(fmt.Sprintf("    • %s\n", issue))
				}
			}
			result.WriteString("\n")
		}
	}

	if dataFlowCount == 0 {
		result.WriteString("❌ No Data Flow Tasks found in this package.\n\n")
		result.WriteString("💡 Memory profiling is only applicable to Data Flow Tasks.\n")
	} else {
		result.WriteString("📈 Overall Memory Profile:\n")
		result.WriteString(fmt.Sprintf("• Analyzed %d Data Flow Task(s)\n", dataFlowCount))
		result.WriteString(fmt.Sprintf("• Estimated Total Buffer Memory: ~%s\n", formatBytes(totalEstimatedMemory)))
		result.WriteString(fmt.Sprintf("• Recommended System Memory: ~%s+\n", formatBytes(totalEstimatedMemory*2)))

		// Memory optimization recommendations
		result.WriteString("\n🧠 Memory Optimization Recommendations:\n")
		result.WriteString("• Monitor actual memory usage during execution\n")
		result.WriteString("• Adjust DefaultBufferSize based on available RAM\n")
		result.WriteString("• Use 64-bit SSIS for large memory requirements\n")
		result.WriteString("• Consider data partitioning for very large datasets\n")
		result.WriteString("• Use BLOBTempStoragePath for large object processing\n")
		result.WriteString("• Optimize data types to reduce memory footprint\n")
		result.WriteString("• Consider caching strategies for reference data\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// analyzeBufferSettings extracts buffer-related settings from a task
func analyzeBufferSettings(task types.Task) []BufferSetting {
	var settings []BufferSetting

	bufferProps := []string{
		"DefaultBufferSize", "DefaultBufferMaxRows", "MaxBufferSize",
		"MinBufferSize", "AutoAdjustBufferSize", "BufferTempStoragePath",
		"BLOBTempStoragePath", "EngineThreads",
	}

	for _, prop := range task.Properties {
		for _, bufferProp := range bufferProps {
			if prop.Name == bufferProp {
				settings = append(settings, BufferSetting{
					Name:  prop.Name,
					Value: prop.Value,
				})
				break
			}
		}
	}

	return settings
}

// generateBufferRecommendations generates buffer optimization recommendations based on components and settings
func generateBufferRecommendations(components []types.DataFlowComponent, bufferSettings []BufferSetting) []string {
	var recommendations []string

	// Count different component types
	sourceCount := 0
	transformCount := 0
	destCount := 0
	hasLookup := false
	hasSort := false
	hasAggregate := false

	for _, comp := range components {
		compType := getComponentType(comp.ComponentClassID)
		switch {
		case strings.Contains(strings.ToLower(compType), "source"):
			sourceCount++
		case strings.Contains(strings.ToLower(compType), "destination"):
			destCount++
		case strings.Contains(strings.ToLower(compType), "lookup"):
			hasLookup = true
			transformCount++
		case strings.Contains(strings.ToLower(compType), "sort"):
			hasSort = true
			transformCount++
		case strings.Contains(strings.ToLower(compType), "aggregate"):
			hasAggregate = true
			transformCount++
		default:
			transformCount++
		}
	}

	// Generate recommendations based on component analysis
	if sourceCount > 1 {
		recommendations = append(recommendations, "Multiple sources detected - consider increasing EngineThreads for parallel reading")
	}

	if destCount > 1 {
		recommendations = append(recommendations, "Multiple destinations detected - consider increasing EngineThreads for parallel writing")
	}

	if hasLookup {
		recommendations = append(recommendations, "Lookup transformations present - ensure sufficient memory for reference data caching")
	}

	if hasSort || hasAggregate {
		recommendations = append(recommendations, "Blocking transformations (Sort/Aggregate) detected - consider increasing DefaultBufferSize")
	}

	// Check current buffer settings and provide specific recommendations
	bufferSize := int64(0)
	bufferMaxRows := 0

	for _, setting := range bufferSettings {
		switch setting.Name {
		case "DefaultBufferSize":
			if val, err := strconv.ParseInt(setting.Value, 10, 64); err == nil {
				bufferSize = val
			}
		case "DefaultBufferMaxRows":
			if val, err := strconv.Atoi(setting.Value); err == nil {
				bufferMaxRows = val
			}
		}
	}

	if bufferSize < 10485760 { // Less than 10MB
		recommendations = append(recommendations, "DefaultBufferSize is low - consider increasing to 10MB+ for better performance")
	}

	if bufferMaxRows > 0 && bufferMaxRows < 10000 {
		recommendations = append(recommendations, "DefaultBufferMaxRows is low - consider increasing to 10,000+ for better throughput")
	}

	return recommendations
}

// analyzeTaskParallelization analyzes task dependencies for parallel execution potential
func analyzeTaskParallelization(tasks []types.Task) []string {
	var analysis []string

	// Simple analysis of task dependencies
	// In a real implementation, this would parse precedence constraints
	taskCount := len(tasks)
	if taskCount == 0 {
		return []string{"No tasks found in package"}
	}

	analysis = append(analysis, fmt.Sprintf("Total tasks: %d", taskCount))

	// Count different task types
	dataFlowCount := 0
	executeSQLCount := 0
	fileSystemCount := 0
	scriptCount := 0

	for _, task := range tasks {
		switch getTaskType(task) {
		case "ExecuteSQLTask":
			executeSQLCount++
		case "FileSystemTask":
			fileSystemCount++
		case "ScriptTask":
			scriptCount++
		default:
			if isDataFlowTask(task) {
				dataFlowCount++
			}
		}
	}

	if dataFlowCount > 0 {
		analysis = append(analysis, fmt.Sprintf("Data Flow Tasks: %d (can run in parallel if no dependencies)", dataFlowCount))
	}
	if executeSQLCount > 0 {
		analysis = append(analysis, fmt.Sprintf("Execute SQL Tasks: %d (check for dependencies on same database)", executeSQLCount))
	}
	if fileSystemCount > 0 {
		analysis = append(analysis, fmt.Sprintf("File System Tasks: %d (can often run in parallel)", fileSystemCount))
	}

	return analysis
}

// analyzeDataFlowParallelization analyzes Data Flow Tasks for parallel processing configuration
func analyzeDataFlowParallelization(tasks []types.Task) []string {
	var analysis []string

	for _, task := range tasks {
		if isDataFlowTask(task) {
			engineThreads := "Not set"
			for _, prop := range task.Properties {
				if prop.Name == "EngineThreads" {
					engineThreads = prop.Value
					break
				}
			}

			if engineThreads == "Not set" {
				analysis = append(analysis, fmt.Sprintf("Data Flow '%s': EngineThreads not configured (using default)", task.Name))
			} else {
				if val, err := strconv.Atoi(engineThreads); err == nil && val < 2 {
					analysis = append(analysis, fmt.Sprintf("Data Flow '%s': EngineThreads=%s (consider increasing for parallel processing)", task.Name, engineThreads))
				} else {
					analysis = append(analysis, fmt.Sprintf("Data Flow '%s': EngineThreads=%s (good for parallel processing)", task.Name, engineThreads))
				}
			}
		}
	}

	if len(analysis) == 0 {
		analysis = append(analysis, "No Data Flow Tasks found")
	}

	return analysis
}

// analyzeContainerParallelization analyzes containers for parallel execution capabilities
func analyzeContainerParallelization(tasks []types.Task) []string {
	var analysis []string

	containerCount := 0
	for _, task := range tasks {
		if getTaskType(task) == "Sequence" || getTaskType(task) == "ForLoop" || getTaskType(task) == "ForeachLoop" {
			containerCount++
			containerType := getTaskType(task)
			analysis = append(analysis, fmt.Sprintf("%s '%s': Enables parallel execution of child tasks", containerType, task.Name))
		}
	}

	if containerCount == 0 {
		analysis = append(analysis, "No containers found - consider using Sequence Containers to group independent tasks")
	} else {
		analysis = append(analysis, fmt.Sprintf("Found %d container(s) for organizing parallel execution", containerCount))
	}

	return analysis
}

// estimateBufferMemoryUsage estimates memory usage based on buffer settings
func estimateBufferMemoryUsage(task types.Task) int64 {
	bufferSize := int64(1048576) // Default 1MB
	bufferMaxRows := 10000       // Default 10k rows

	for _, prop := range task.Properties {
		switch prop.Name {
		case "DefaultBufferSize":
			if val, err := strconv.ParseInt(prop.Value, 10, 64); err == nil {
				bufferSize = val
			}
		case "DefaultBufferMaxRows":
			if val, err := strconv.Atoi(prop.Value); err == nil {
				bufferMaxRows = val
			}
		}
	}

	// Estimate memory based on buffer settings
	// This is a simplified estimation: buffer size * estimated row overhead
	estimatedRowSize := int64(100) // Rough estimate of bytes per row
	rowOverhead := bufferMaxRows * int(estimatedRowSize)
	return bufferSize + int64(rowOverhead)
}

// formatBytes formats byte counts into human-readable strings
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// analyzeComponentMemoryUsage analyzes memory usage patterns of data flow components
func analyzeComponentMemoryUsage(components []types.DataFlowComponent) []ComponentMemory {
	var memoryAnalysis []ComponentMemory

	for _, comp := range components {
		compType := getComponentType(comp.ComponentClassID)
		var estimate string
		var recommendation string

		switch {
		case strings.Contains(strings.ToLower(compType), "lookup"):
			estimate = "High (caches reference data)"
			recommendation = "Ensure sufficient memory for lookup cache"
		case strings.Contains(strings.ToLower(compType), "sort"):
			estimate = "High (requires memory for sorting)"
			recommendation = "Consider increasing buffer size for large sorts"
		case strings.Contains(strings.ToLower(compType), "aggregate"):
			estimate = "Medium-High (accumulates data)"
			recommendation = "Monitor memory usage during aggregation"
		case strings.Contains(strings.ToLower(compType), "merge"):
			estimate = "Medium (combines multiple inputs)"
			recommendation = "Ensure adequate buffer size for merge operations"
		case strings.Contains(strings.ToLower(compType), "source"):
			estimate = "Low-Medium (reading data)"
			recommendation = "Optimize source query if possible"
		case strings.Contains(strings.ToLower(compType), "destination"):
			estimate = "Low-Medium (writing data)"
			recommendation = "Configure appropriate batch sizes"
		default:
			estimate = "Medium (standard transformation)"
			recommendation = ""
		}

		memoryAnalysis = append(memoryAnalysis, ComponentMemory{
			Component:      compType,
			Estimate:       estimate,
			Recommendation: recommendation,
		})
	}

	return memoryAnalysis
}

// detectMemoryIntensiveOperations identifies components that may be memory-intensive
func detectMemoryIntensiveOperations(task types.Task) []string {
	var issues []string

	if task.ObjectData.DataFlow.Components.Components != nil {
		for _, comp := range task.ObjectData.DataFlow.Components.Components {
			compType := getComponentType(comp.ComponentClassID)

			// Check for memory-intensive operations
			if strings.Contains(strings.ToLower(compType), "sort") {
				issues = append(issues, fmt.Sprintf("%s: Sorting operations can be memory-intensive", compType))
			}
			if strings.Contains(strings.ToLower(compType), "aggregate") {
				issues = append(issues, fmt.Sprintf("%s: Aggregation operations accumulate data in memory", compType))
			}
			if strings.Contains(strings.ToLower(compType), "lookup") {
				issues = append(issues, fmt.Sprintf("%s: Lookup caching can consume significant memory", compType))
			}
		}
	}

	return issues
}
