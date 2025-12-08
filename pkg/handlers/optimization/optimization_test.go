package optimization

import (
	"path/filepath"
	"testing"

	"github.com/MCPRUNNER/gossisMCP/pkg/types"
)

func TestResolveFilePath(t *testing.T) {
	base := "C:/packages"
	relative := "sample.dtsx"
	expected := filepath.Join(base, relative)
	if got := ResolveFilePath(relative, base); got != expected {
		t.Fatalf("expected %s, got %s", expected, got)
	}
	absolute := "C:\\data\\file.dtsx"
	if got := ResolveFilePath(absolute, base); got != absolute {
		t.Fatalf("expected absolute path %s, got %s", absolute, got)
	}
}

func TestTaskHelpers(t *testing.T) {
	task := types.Task{
		Properties: []types.Property{{Name: "TaskType", Value: "SSIS.Pipeline.3"}},
	}
	if taskType := getTaskType(task); taskType != "SSIS.Pipeline.3" {
		t.Fatalf("unexpected task type %s", taskType)
	}
	if !isDataFlowTask(task) {
		t.Fatal("expected data flow task to be detected")
	}
}

func TestBufferRecommendations(t *testing.T) {
	components := []types.DataFlowComponent{
		{ComponentClassID: "Microsoft.OLEDBSource"},
		{ComponentClassID: "Microsoft.Sort"},
		{ComponentClassID: "Microsoft.Lookup"},
	}
	settings := []BufferSetting{{Name: "DefaultBufferSize", Value: "1024"}, {Name: "DefaultBufferMaxRows", Value: "100"}}
	recs := generateBufferRecommendations(components, settings)
	if len(recs) == 0 {
		t.Fatal("expected buffer recommendations to be generated")
	}
}

func TestAnalyzeTaskParallelization(t *testing.T) {
	tasks := []types.Task{
		{Properties: []types.Property{{Name: "TaskType", Value: "SSIS.Pipeline.3"}}},
		{Properties: []types.Property{{Name: "TaskType", Value: "ExecuteSQLTask"}}},
	}
	analysis := analyzeTaskParallelization(tasks)
	if len(analysis) == 0 {
		t.Fatal("expected analysis entries")
	}
}

func TestAnalyzeDataFlowParallelization(t *testing.T) {
	tasks := []types.Task{
		{Name: "DataFlow", Properties: []types.Property{{Name: "TaskType", Value: "SSIS.Pipeline.3"}, {Name: "EngineThreads", Value: "1"}}},
	}
	analysis := analyzeDataFlowParallelization(tasks)
	if len(analysis) == 0 {
		t.Fatal("expected engine thread analysis")
	}
}

func TestAnalyzeComponentMemoryUsage(t *testing.T) {
	components := []types.DataFlowComponent{
		{ComponentClassID: "Microsoft.Sort"},
		{ComponentClassID: "Microsoft.Lookup"},
	}
	analysis := analyzeComponentMemoryUsage(components)
	if len(analysis) != 2 {
		t.Fatalf("expected two component analyses, got %d", len(analysis))
	}
	if analysis[0].Estimate == "" {
		t.Fatal("expected estimate to be populated")
	}
}

func TestDetectMemoryIntensiveOperations(t *testing.T) {
	task := types.Task{
		ObjectData: types.TaskObjectData{
			DataFlow: types.DataFlowDetails{
				Components: types.DataFlowComponents{
					Components: []types.DataFlowComponent{{ComponentClassID: "Microsoft.Sort"}},
				},
			},
		},
	}
	issues := detectMemoryIntensiveOperations(task)
	if len(issues) == 0 {
		t.Fatal("expected sort component to be flagged")
	}
}

func TestEstimateBufferMemoryUsage(t *testing.T) {
	task := types.Task{
		Properties: []types.Property{{Name: "DefaultBufferSize", Value: "2048"}, {Name: "DefaultBufferMaxRows", Value: "20"}},
	}
	if estimate := estimateBufferMemoryUsage(task); estimate <= 0 {
		t.Fatalf("expected positive memory estimate, got %d", estimate)
	}
}

func TestFormatBytes(t *testing.T) {
	if out := formatBytes(500); out != "500 B" {
		t.Fatalf("expected plain bytes, got %s", out)
	}
	if !hasSuffix(formatBytes(2048), "KB") {
		t.Fatalf("expected kilobyte suffix, got %s", formatBytes(2048))
	}
}

func hasSuffix(value, suffix string) bool {
	if len(value) < len(suffix) {
		return false
	}
	return value[len(value)-len(suffix):] == suffix
}
