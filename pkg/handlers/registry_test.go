package handlers

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandlerRegistry(t *testing.T) {
	registry := NewHandlerRegistry()
	firstCalled := false
	first := func(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error) {
		firstCalled = true
		return nil, nil
	}

	registry.Register("first", first)

	handler, ok := registry.GetHandler("first")
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	if handler == nil {
		t.Fatal("expected handler to be returned")
	}

	_, _ = handler(context.Background(), mcp.CallToolRequest{}, "")
	if !firstCalled {
		t.Fatal("expected registered handler to be invoked")
	}

	tools := registry.GetRegisteredTools()
	if len(tools) != 1 {
		t.Fatalf("expected exactly one registered tool, got %v", tools)
	}
	found := false
	for _, tool := range tools {
		if tool == "first" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected registered tools to contain first, got %v", tools)
	}

	if _, exists := registry.GetHandler("missing"); exists {
		t.Fatal("expected unknown handler lookup to fail")
	}
}
