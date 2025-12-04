package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func newRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func TestExamplePluginMetadata(t *testing.T) {
	if Plugin == nil {
		t.Fatal("expected global Plugin instance to be initialised")
	}

	plugin := &ExamplePlugin{}
	metadata := plugin.Metadata()

	if metadata["id"] != "example-ssis-plugin" {
		t.Fatalf("unexpected id: %v", metadata["id"])
	}
	if metadata["name"] != "Example SSIS Plugin" {
		t.Fatalf("unexpected name: %v", metadata["name"])
	}
	if metadata["category"] != "Analysis" {
		t.Fatalf("unexpected category: %v", metadata["category"])
	}

	tools, ok := metadata["tools"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected tools slice, got %T", metadata["tools"])
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestExamplePluginToolsMirrorMetadata(t *testing.T) {
	plugin := &ExamplePlugin{}
	metadataTools := PluginInfo["tools"].([]map[string]interface{})
	tools := plugin.Tools()

	if len(tools) != len(metadataTools) {
		t.Fatalf("expected %d tools, got %d", len(metadataTools), len(tools))
	}

	for i, tool := range tools {
		if tool["name"] != metadataTools[i]["name"] {
			t.Fatalf("tool %d name mismatch: %v != %v", i, tool["name"], metadataTools[i]["name"])
		}
	}
}

func TestExamplePluginExecuteDetectHardcodedConnections(t *testing.T) {
	plugin := &ExamplePlugin{}
	request := newRequest(map[string]interface{}{
		"file_path": "test.dtsx",
	})

	result, err := plugin.ExecuteTool(context.Background(), "detect_hardcoded_connections", request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result from detect_hardcoded_connections")
	}

	output := fmt.Sprintf("%v", result)
	if !strings.Contains(output, "Hardcoded connection analysis result: 1 findings found") {
		t.Fatalf("unexpected analysis output: %s", output)
	}
	if !strings.Contains(output, "warning") {
		t.Fatalf("expected default severity warning, got: %s", output)
	}
}

func TestExamplePluginExecuteDetectHardcodedConnectionsCustomSeverity(t *testing.T) {
	plugin := &ExamplePlugin{}
	request := newRequest(map[string]interface{}{
		"file_path": "test.dtsx",
		"severity":  "error",
	})

	result, err := plugin.ExecuteTool(context.Background(), "detect_hardcoded_connections", request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := fmt.Sprintf("%v", result)
	if !strings.Contains(output, "error") {
		t.Fatalf("expected custom severity to appear, got: %s", output)
	}
}

func TestExamplePluginExecuteAnalyzeVariableUsage(t *testing.T) {
	plugin := &ExamplePlugin{}
	request := newRequest(map[string]interface{}{
		"file_path": "variables.dtsx",
	})

	result, err := plugin.ExecuteTool(context.Background(), "analyze_variable_usage", request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result from analyze_variable_usage")
	}

	output := fmt.Sprintf("%v", result)
	if !strings.Contains(output, "Variable usage analysis: 2 total, 1 used, 1 unused") {
		t.Fatalf("unexpected variable usage output: %s", output)
	}
	if !strings.Contains(output, "50.0%") {
		t.Fatalf("expected usage ratio in output, got: %s", output)
	}
}

func TestExamplePluginExecuteUnknownTool(t *testing.T) {
	plugin := &ExamplePlugin{}
	request := newRequest(map[string]interface{}{})

	result, err := plugin.ExecuteTool(context.Background(), "unknown", request)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if result != nil {
		t.Fatalf("expected nil result on error, got: %v", result)
	}
}
