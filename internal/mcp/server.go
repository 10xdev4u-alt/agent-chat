package mcp

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"

	authpkg "github.com/10xdev4u-alt/agent-chat/internal/auth"
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
		server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			// Extract bearer token and store in context for tool handlers.
			token, ok := authpkg.BearerTokenFromHeader(r.Header.Get("Authorization"))
			if ok {
				ctx = context.WithValue(ctx, authpkgTokenKey{}, string(token))
			}
			return ctx
		}),
	)
	s.HTTPServer = httpSrv
	s.StreamableHTTPServer = httpSrv
	s.StreamableHTTP = httpSrv
	s.HTTP = httpSrv

	return s, nil
}

// authpkgTokenKey is a context key for the bearer token.
type authpkgTokenKey struct{}

// TokenFromContext retrieves the bearer token from context.
func TokenFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(authpkgTokenKey{}).(string)
	return v, ok
}
