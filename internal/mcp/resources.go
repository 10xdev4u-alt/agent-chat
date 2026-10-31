package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerResources(s *server.MCPServer, srv *Server) {
	s.AddResource(mcp.NewResource("rooms://", "rooms",
		mcp.WithResourceDescription("List all rooms"),
	), srv.handleRoomsList)

	s.AddResourceTemplate(mcp.NewResourceTemplate("rooms://{id}", "room",
		mcp.WithTemplateDescription("Room metadata"),
	), srv.handleRoomMetadata)

	s.AddResourceTemplate(mcp.NewResourceTemplate("rooms://{id}/messages", "room-messages",
		mcp.WithTemplateDescription("Room message history"),
	), srv.handleRoomMessages)

	s.AddResourceTemplate(mcp.NewResourceTemplate("rooms://{id}/members", "room-members",
		mcp.WithTemplateDescription("Room membership"),
	), srv.handleRoomMembers)

	s.AddResource(mcp.NewResource("agents://", "agents",
		mcp.WithResourceDescription("List all agents"),
	), srv.handleAgentsList)
}

func (s *Server) handleRoomsList(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource("rooms://", []map[string]any{}), nil
}

func (s *Server) handleRoomMetadata(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource(request.Params.URI, map[string]any{}), nil
}

func (s *Server) handleRoomMessages(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource(request.Params.URI, []map[string]any{}), nil
}

func (s *Server) handleRoomMembers(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource(request.Params.URI, []map[string]any{}), nil
}

func (s *Server) handleAgentsList(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource("agents://", []map[string]any{}), nil
}

func jsonResource(uri string, v any) []mcp.ResourceContents {
	b, _ := json.Marshal(v)
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(b),
		},
	}
}
