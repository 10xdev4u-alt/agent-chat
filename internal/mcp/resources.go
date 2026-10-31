package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
	roomspkg "github.com/10xdev4u-alt/agent-chat/internal/rooms"
)

// SubscriptionManager tracks which sessions are subscribed to room message resources.
type SubscriptionManager struct {
	mu   sync.Mutex
	subs map[string][]string // roomID -> session IDs
}

// NewSubscriptionManager creates a new in-memory subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{subs: make(map[string][]string)}
}

// Subscribe records a session subscription for a room's messages resource.
func (m *SubscriptionManager) Subscribe(roomID, sessionID string) {
	if roomID == "" || sessionID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.subs[roomID] {
		if id == sessionID {
			return
		}
	}
	m.subs[roomID] = append(m.subs[roomID], sessionID)
}

// Unsubscribe removes a session subscription for a room's messages resource.
func (m *SubscriptionManager) Unsubscribe(roomID, sessionID string) {
	if roomID == "" || sessionID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.subs[roomID]
	out := list[:0]
	for _, id := range list {
		if id != sessionID {
			out = append(out, id)
		}
	}
	m.subs[roomID] = out
}

// UnsubscribeAll removes a session from every room subscription.
func (m *SubscriptionManager) UnsubscribeAll(sessionID string) {
	if sessionID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for roomID, list := range m.subs {
		out := list[:0]
		for _, id := range list {
			if id != sessionID {
				out = append(out, id)
			}
		}
		m.subs[roomID] = out
	}
}

// Subscribers returns a copy of the session IDs subscribed to a room's messages.
func (m *SubscriptionManager) Subscribers(roomID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.subs[roomID]...)
}

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

	hooks := s.GetHooks()
	if hooks != nil {
		hooks.AddAfterSubscribe(func(ctx context.Context, id any, req *mcp.SubscribeRequest, res *mcp.EmptyResult) {
			roomID, ok := roomIDFromMessagesURI(req.Params.URI)
			if !ok {
				return
			}
			if sess := server.ClientSessionFromContext(ctx); sess != nil {
				srv.Subscriptions.Subscribe(roomID, sess.SessionID())
			}
		})
		hooks.AddAfterUnsubscribe(func(ctx context.Context, id any, req *mcp.UnsubscribeRequest, res *mcp.EmptyResult) {
			roomID, ok := roomIDFromMessagesURI(req.Params.URI)
			if !ok {
				return
			}
			if sess := server.ClientSessionFromContext(ctx); sess != nil {
				srv.Subscriptions.Unsubscribe(roomID, sess.SessionID())
			}
		})
	}
}

func (s *Server) handleRoomsList(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	rooms, err := roomspkg.ListRooms(s.DB)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	return jsonResource("rooms://", rooms), nil
}

func (s *Server) handleRoomMetadata(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	roomID := roomIDFromURI(request.Params.URI)
	room, err := roomspkg.GetRoom(s.DB, roomID)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return jsonResource(request.Params.URI, room), nil
}

func (s *Server) handleRoomMessages(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	roomID := roomIDFromURI(request.Params.URI)
	msgs, err := roomspkg.ReadMessages(s.DB, roomID, 0, 50)
	if err != nil {
		return nil, fmt.Errorf("read messages: %w", err)
	}
	return jsonResource(request.Params.URI, msgs), nil
}

func (s *Server) handleRoomMembers(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	roomID := roomIDFromURI(request.Params.URI)
	members, err := roomspkg.ListMembers(s.DB, roomID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	return jsonResource(request.Params.URI, members), nil
}

func (s *Server) handleAgentsList(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	agents, err := dbpkg.ListAgents(s.DB)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	out := make([]map[string]any, 0, len(agents))
	for _, a := range agents {
		out = append(out, map[string]any{
			"id":         a.ID,
			"name":       a.Name,
			"harness":    a.Harness,
			"repo":       a.Repo,
			"branch":     a.Branch,
			"status":     a.Status,
			"doing":      a.Doing,
			"role":       a.Role,
			"created_at": a.CreatedAt,
			"updated_at": a.UpdatedAt,
		})
	}
	return jsonResource("agents://", out), nil
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

func roomIDFromURI(uri string) string {
	u := strings.TrimPrefix(uri, "rooms://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

func roomIDFromMessagesURI(uri string) (string, bool) {
	const prefix = "rooms://"
	if !strings.HasPrefix(uri, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(uri, prefix)
	if !strings.HasSuffix(rest, "/messages") {
		return "", false
	}
	id := strings.TrimSuffix(rest, "/messages")
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}
