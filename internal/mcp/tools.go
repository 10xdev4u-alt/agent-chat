package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func boolPtr(v bool) *bool { return &v }

func registerTools(s *server.MCPServer, srv *Server) {
	s.AddTool(mcp.NewTool("register",
		mcp.WithDescription("Register a new agent and obtain credentials"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Agent name")),
		mcp.WithString("harness", mcp.Required(), mcp.Description("Agent harness")),
		mcp.WithString("repo", mcp.Description("Repository")),
		mcp.WithString("branch", mcp.Description("Branch")),
		mcp.WithTitleAnnotation("Register"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleRegister)

	stubTools := []struct {
		name string
		desc string
	}{
		{"list_rooms", "List all rooms"},
		{"join_room", "Join a room"},
		{"leave_room", "Leave a room"},
		{"list_members", "List room members"},
		{"send_message", "Send a message"},
		{"read_messages", "Read messages"},
		{"update_status", "Update agent status"},
		{"search_messages", "Search messages"},
		{"request_approval", "Request approval"},
		{"check_approval", "Check approval status"},
	}
	_ = stubTools

	// list_rooms
	s.AddTool(mcp.NewTool("list_rooms",
		mcp.WithDescription("List all rooms"),
		mcp.WithTitleAnnotation("List Rooms"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("join_room",
		mcp.WithDescription("Join a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithTitleAnnotation("Join Room"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("leave_room",
		mcp.WithDescription("Leave a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithTitleAnnotation("Leave Room"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("list_members",
		mcp.WithDescription("List room members"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithTitleAnnotation("List Members"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("send_message",
		mcp.WithDescription("Send a message to a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Message body")),
		mcp.WithArray("mentions", mcp.Description("Mentioned agent IDs"), mcp.WithStringItems()),
		mcp.WithString("reply_to", mcp.Description("Reply-to message ID")),
		mcp.WithString("priority", mcp.Description("Priority: urgent, normal, fyi")),
		mcp.WithTitleAnnotation("Send Message"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("read_messages",
		mcp.WithDescription("Read messages from a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithNumber("after_seq", mcp.Description("Read after this seq")),
		mcp.WithNumber("limit", mcp.Description("Max messages")),
		mcp.WithTitleAnnotation("Read Messages"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("update_status",
		mcp.WithDescription("Update agent status"),
		mcp.WithString("status", mcp.Required(), mcp.Description("Status")),
		mcp.WithString("doing", mcp.Description("Current activity")),
		mcp.WithTitleAnnotation("Update Status"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("search_messages",
		mcp.WithDescription("Search messages in a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithNumber("limit", mcp.Description("Max results")),
		mcp.WithTitleAnnotation("Search Messages"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("request_approval",
		mcp.WithDescription("Request approval for an action"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action")),
		mcp.WithString("description", mcp.Description("Description")),
		mcp.WithString("risk_level", mcp.Description("Risk level")),
		mcp.WithTitleAnnotation("Request Approval"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)

	s.AddTool(mcp.NewTool("check_approval",
		mcp.WithDescription("Check approval status"),
		mcp.WithString("approval_id", mcp.Required(), mcp.Description("Approval ID")),
		mcp.WithTitleAnnotation("Check Approval"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.notImplemented)
}
