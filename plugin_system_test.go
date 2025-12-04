package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/MCPRUNNER/gossisMCP/pkg/config"
	"github.com/mark3labs/mcp-go/mcp"
)

type stubToolExecutor struct {
	executed bool
}

func (s *stubToolExecutor) Execute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.executed = true
	return nil, nil
}

type stubResourceHandler struct {
	data []byte
}

func (s *stubResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return s.data, nil
}

func (s *stubResourceHandler) List(ctx context.Context, uri string) ([]mcp.Resource, error) {
	return nil, nil
}

func TestPluginRegistryRegisterGetList(t *testing.T) {
	registry := NewPluginRegistry()
	first := &PluginMetadata{ID: "b", Name: "Beta"}
	second := &PluginMetadata{ID: "a", Name: "Alpha"}

	registry.Register(first)
	registry.Register(second)

	if got, ok := registry.Get("a"); !ok || got != second {
		t.Fatalf("expected to retrieve Alpha plugin, got %v %v", got, ok)
	}

	plugins := registry.List()
	if len(plugins) != 2 {
		t.Fatalf("expected two plugins in list, got %d", len(plugins))
	}
	if plugins[0].Name != "Alpha" || plugins[1].Name != "Beta" {
		t.Fatalf("expected list to be sorted by name, got %v", []string{plugins[0].Name, plugins[1].Name})
	}
}

func TestPluginManagerGetTool(t *testing.T) {
	manager := NewPluginManager()
	executor := &stubToolExecutor{}
	manager.loadedPlugins["plugin"] = &LoadedPlugin{
		Metadata: &PluginMetadata{ID: "plugin"},
		Tools:    map[string]ToolExecutor{"tool": executor},
		Enabled:  true,
	}

	tool, ok := manager.GetTool("plugin", "tool")
	if !ok || tool == nil {
		t.Fatal("expected to retrieve registered tool")
	}

	manager.loadedPlugins["plugin"].Enabled = false
	if tool, ok := manager.GetTool("plugin", "tool"); ok || tool != nil {
		t.Fatal("expected disabled plugin tool lookup to fail")
	}

	if tool, ok := manager.GetTool("missing", "tool"); ok || tool != nil {
		t.Fatal("expected missing plugin lookup to fail")
	}
}

func TestPluginManagerGetResource(t *testing.T) {
	manager := NewPluginManager()
	handler := &stubResourceHandler{data: []byte("content")}
	manager.loadedPlugins["plugin"] = &LoadedPlugin{
		Metadata:  &PluginMetadata{ID: "plugin"},
		Resources: map[string]ResourceHandler{"uri": handler},
		Enabled:   true,
	}

	resource, ok := manager.GetResource("plugin", "uri")
	if !ok || resource == nil {
		t.Fatal("expected to retrieve registered resource handler")
	}

	manager.loadedPlugins["plugin"].Enabled = false
	if resource, ok := manager.GetResource("plugin", "uri"); ok || resource != nil {
		t.Fatal("expected disabled plugin resource lookup to fail")
	}

	if resource, ok := manager.GetResource("missing", "uri"); ok || resource != nil {
		t.Fatal("expected missing plugin resource lookup to fail")
	}
}

func TestPluginSystemInitializeRegistersBuiltins(t *testing.T) {
	cfg := config.DefaultPluginConfig()
	cfg.PluginDir = filepath.Join(t.TempDir(), "plugins")
	cfg.AutoUpdate = false

	system := NewPluginSystem(cfg)
	if err := system.Initialize(); err != nil {
		t.Fatalf("unexpected initialize error: %v", err)
	}

	if _, ok := system.registry.Get("ssis-core-analysis"); !ok {
		t.Fatal("expected builtin plugin ssis-core-analysis to be registered")
	}
}
