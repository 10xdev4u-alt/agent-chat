package mcp

import (
	"database/sql"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the underlying MCP server and HTTP transport for agent-chat.
type Server struct {
	*server.MCPServer

	DB *sql.DB

	// Aliases for compatibility with different callers.
	MCP                  *server.MCPServer
	HTTPServer           *server.StreamableHTTPServer
	StreamableHTTPServer *server.StreamableHTTPServer
	StreamableHTTP       *server.StreamableHTTPServer
	HTTP                 *server.StreamableHTTPServer
}

// NewServer creates an MCP server with streamable HTTP transport.
func NewServer(db *sql.DB) (*Server, error) {
	mcpSrv := server.NewMCPServer(
		"agent-chat",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
	)

	s := &Server{
		MCPServer: mcpSrv,
		DB:        db,
	}
	s.MCP = mcpSrv

	// Register tools and resources.
	registerTools(mcpSrv, s)
	registerResources(mcpSrv, s)

	httpSrv := server.NewStreamableHTTPServer(
		mcpSrv,
		server.WithSessionIdleTTL(30*time.Minute),
	)
	s.HTTPServer = httpSrv
	s.StreamableHTTPServer = httpSrv
	s.StreamableHTTP = httpSrv
	s.HTTP = httpSrv

	return s, nil
}
