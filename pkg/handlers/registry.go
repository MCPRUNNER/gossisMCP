package handlers

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// HandlerFunc represents a handler function signature
type HandlerFunc func(ctx context.Context, request mcp.CallToolRequest, packageDirectory string) (*mcp.CallToolResult, error)

// HandlerRegistry manages MCP tool handlers
type HandlerRegistry struct {
	handlers map[string]HandlerFunc
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register registers a handler for a tool name
func (r *HandlerRegistry) Register(toolName string, handler HandlerFunc) {
	r.handlers[toolName] = handler
}

// GetHandler returns the handler for a tool name
func (r *HandlerRegistry) GetHandler(toolName string) (HandlerFunc, bool) {
	handler, exists := r.handlers[toolName]
	return handler, exists
}

// GetRegisteredTools returns all registered tool names
func (r *HandlerRegistry) GetRegisteredTools() []string {
	tools := make([]string, 0, len(r.handlers))
	for tool := range r.handlers {
		tools = append(tools, tool)
	}
	return tools
}
