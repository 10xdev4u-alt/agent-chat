package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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

	// list_rooms
	s.AddTool(mcp.NewTool("list_rooms",
		mcp.WithDescription("List all rooms"),
		mcp.WithTitleAnnotation("List Rooms"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleListRooms)

	s.AddTool(mcp.NewTool("join_room",
		mcp.WithDescription("Join a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithTitleAnnotation("Join Room"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleJoinRoom)

	s.AddTool(mcp.NewTool("leave_room",
		mcp.WithDescription("Leave a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithTitleAnnotation("Leave Room"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleLeaveRoom)

	s.AddTool(mcp.NewTool("list_members",
		mcp.WithDescription("List room members"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithTitleAnnotation("List Members"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleListMembers)

	s.AddTool(mcp.NewTool("send_message",
		mcp.WithDescription("Send a message to a room"),
		mcp.WithString("room_id", mcp.Required(), mcp.Description("Room ID")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Message body")),
		mcp.WithArray("mentions", mcp.Description("Mentioned agent IDs"), mcp.WithStringItems()),
		mcp.WithString("reply_to", mcp.Description("Reply-to message ID")),
		mcp.WithString("priority", mcp.Description("Priority: urgent, normal, fyi")),
		mcp.WithArray("attachment_ids", mcp.Description("Attachment IDs to attach"), mcp.WithStringItems()),
		mcp.WithTitleAnnotation("Send Message"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleSendMessage)

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
	), srv.handleReadMessages)

	s.AddTool(mcp.NewTool("update_status",
		mcp.WithDescription("Update agent status"),
		mcp.WithString("status", mcp.Required(), mcp.Description("Status")),
		mcp.WithString("doing", mcp.Description("Current activity")),
		mcp.WithTitleAnnotation("Update Status"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleUpdateStatus)

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
	), srv.handleSearchMessages)

	s.AddTool(mcp.NewTool("edit_message",
		mcp.WithDescription("Edit a message body"),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("Message ID")),
		mcp.WithString("body", mcp.Required(), mcp.Description("New message body")),
		mcp.WithTitleAnnotation("Edit Message"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleEditMessage)

	s.AddTool(mcp.NewTool("delete_message",
		mcp.WithDescription("Soft-delete a message"),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("Message ID")),
		mcp.WithTitleAnnotation("Delete Message"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleDeleteMessage)

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
	), srv.handleRequestApproval)

	s.AddTool(mcp.NewTool("check_approval",
		mcp.WithDescription("Check approval status"),
		mcp.WithString("approval_id", mcp.Required(), mcp.Description("Approval ID")),
		mcp.WithTitleAnnotation("Check Approval"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleCheckApproval)

	s.AddTool(mcp.NewTool("attach_file",
		mcp.WithDescription("Attach a file to a message"),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("Message ID")),
		mcp.WithString("filename", mcp.Required(), mcp.Description("Filename")),
		mcp.WithString("content_type", mcp.Description("Content type")),
		mcp.WithNumber("size_bytes", mcp.Required(), mcp.Description("Size in bytes")),
		mcp.WithString("storage_path", mcp.Required(), mcp.Description("Storage path")),
		mcp.WithTitleAnnotation("Attach File"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleAttachFile)

	s.AddTool(mcp.NewTool("list_attachments",
		mcp.WithDescription("List attachments for a message"),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("Message ID")),
		mcp.WithTitleAnnotation("List Attachments"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleListAttachments)


	s.AddTool(mcp.NewTool("connect_peer",
		mcp.WithDescription("Connect to a remote agent-chat instance"),
		mcp.WithString("peer_url", mcp.Required(), mcp.Description("Peer WebSocket URL")),
		mcp.WithTitleAnnotation("Connect Peer"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), srv.handleConnectPeer)

	s.AddTool(mcp.NewTool("list_peers",
		mcp.WithDescription("List connected peers"),
		mcp.WithTitleAnnotation("List Peers"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleListPeers)

	s.AddTool(mcp.NewTool("disconnect_peer",
		mcp.WithDescription("Disconnect from a remote agent-chat instance"),
		mcp.WithString("peer_url", mcp.Required(), mcp.Description("Peer WebSocket URL")),
		mcp.WithTitleAnnotation("Disconnect Peer"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), srv.handleDisconnectPeer)
}
