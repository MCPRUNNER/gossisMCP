package server

import (
	"log"

	"github.com/mark3labs/mcp-go/server"
)

// RunHTTPServer starts an HTTP server with streaming capabilities
func RunHTTPServer(s *server.MCPServer, port string) {
	// Use the official MCP StreamableHTTPServer for proper MCP HTTP transport
	streamableServer := server.NewStreamableHTTPServer(s)

	log.Printf("Starting MCP HTTP server on port %s", port)
	log.Printf("MCP endpoints available at: http://localhost:%s/mcp", port)
	log.Printf("Health check available at: http://localhost:%s/health", port)

	// Start the server
	if err := streamableServer.Start(":" + port); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
