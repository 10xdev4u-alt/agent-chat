package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	approvalspkg "github.com/10xdev4u-alt/agent-chat/internal/approvals"
	attachmentspkg "github.com/10xdev4u-alt/agent-chat/internal/attachments"
	auditpkg "github.com/10xdev4u-alt/agent-chat/internal/audit"
	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
	reactionspkg "github.com/10xdev4u-alt/agent-chat/internal/reactions"
	relaypkg "github.com/10xdev4u-alt/agent-chat/internal/relay"
	roomspkg "github.com/10xdev4u-alt/agent-chat/internal/rooms"
)

func (s *Server) handleRegister(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.RateLimiter != nil && !s.RateLimiter.Allow("register") {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	name := request.GetString("name", "")
	harness := request.GetString("harness", "")
	repo := request.GetString("repo", "")
	branch := request.GetString("branch", "")

	if name == "" || harness == "" {
		return mcp.NewToolResultError("name and harness are required"), nil
	}

	agentID, apiKey, err := dbpkg.CreateAgent(s.DB, name, harness, repo, branch, "agent")
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	sessionID := uuid.New().String()

	result := map[string]any{
		"agent_id":   agentID,
		"api_key":    apiKey,
		"session_id": sessionID,
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleListRooms(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rooms, err := roomspkg.ListRooms(s.DB)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	b, _ := json.Marshal(rooms)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleJoinRoom(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	if s.RateLimiter != nil && !s.RateLimiter.Allow(agentID) {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	roomID := request.GetString("room_id", "")
	if roomID == "" {
		return mcp.NewToolResultError("room_id is required"), nil
	}
	if err := roomspkg.JoinRoom(s.DB, roomID, agentID, "member"); err != nil {
		return nil, fmt.Errorf("join room: %w", err)
	}
	_ = auditpkg.Log(s.DB, agentID, "room.joined", "room", roomID, nil)
	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleLeaveRoom(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	roomID := request.GetString("room_id", "")
	if roomID == "" {
		return mcp.NewToolResultError("room_id is required"), nil
	}
	if err := roomspkg.LeaveRoom(s.DB, roomID, agentID); err != nil {
		return nil, fmt.Errorf("leave room: %w", err)
	}
	_ = auditpkg.Log(s.DB, agentID, "room.left", "room", roomID, nil)
	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleListMembers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := request.GetString("room_id", "")
	if roomID == "" {
		return mcp.NewToolResultError("room_id is required"), nil
	}
	members, err := roomspkg.ListMembers(s.DB, roomID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	b, _ := json.Marshal(members)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleSendMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	if s.RateLimiter != nil && !s.RateLimiter.Allow(agentID) {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	roomID := request.GetString("room_id", "")
	body := request.GetString("body", "")
	if roomID == "" || body == "" {
		return mcp.NewToolResultError("room_id and body are required"), nil
	}

	// Parse mentions from body + args.
	mentions := request.GetStringSlice("mentions", nil)
	members, _ := roomspkg.ListMembers(s.DB, roomID)
	var knownAgents []string
	for _, m := range members {
		knownAgents = append(knownAgents, m.AgentID)
	}
	parsedMentions := roomspkg.ParseMentions(body, knownAgents)
	for _, m := range parsedMentions {
		if !contains(mentions, m) {
			mentions = append(mentions, m)
		}
	}
	priority := request.GetString("priority", "normal")
	replyTo := request.GetString("reply_to", "")
	attachmentIDs := request.GetStringSlice("attachment_ids", nil)

	// Loop guard check.
	if s.LoopGuard != nil {
		for _, m := range mentions {
			if err := s.LoopGuard.Check(agentID, m); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
	}

	msgID, seq, err := roomspkg.SendMessage(s.DB, roomID, agentID, body, mentions, replyTo, priority, attachmentIDs)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Record in loop guard.
	if s.LoopGuard != nil {
		for _, m := range mentions {
			s.LoopGuard.Record(agentID, m)
		}
	}

	_ = auditpkg.Log(s.DB, agentID, "message.sent", "message", msgID, map[string]any{
		"room_id": roomID,
		"seq":     seq,
	})

	// Notify resource subscribers.
	if s.Subscriptions != nil {
		for _, sid := range s.Subscriptions.Subscribers(roomID) {
			_ = s.SendNotificationToSpecificClient(sid, mcp.MethodNotificationResourceUpdated, map[string]any{
				"uri": "rooms://" + roomID + "/messages",
			})
		}
	}

	result := map[string]any{
		"message_id": msgID,
		"seq":        seq,
		"status":     "delivered",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleReadMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	roomID := request.GetString("room_id", "")
	if roomID == "" {
		return mcp.NewToolResultError("room_id is required"), nil
	}
	afterSeq := request.GetInt("after_seq", 0)
	limit := request.GetInt("limit", 50)

	msgs, err := roomspkg.ReadMessages(s.DB, roomID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("read messages: %w", err)
	}

	// Update read cursor.
	if len(msgs) > 0 {
		lastSeq := msgs[len(msgs)-1].Seq
		roomspkg.UpdateReadCursor(s.DB, agentID, roomID, lastSeq)
	}

	b, _ := json.Marshal(msgs)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleUpdateStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	status := request.GetString("status", "")
	doing := request.GetString("doing", "")
	if status == "" {
		return mcp.NewToolResultError("status is required"), nil
	}
	if err := dbpkg.UpdateAgentStatus(s.DB, agentID, status, doing); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleSearchMessages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := request.GetString("room_id", "")
	query := request.GetString("query", "")
	if roomID == "" || query == "" {
		return mcp.NewToolResultError("room_id and query are required"), nil
	}
	limit := request.GetInt("limit", 20)
	msgs, err := roomspkg.SearchMessages(s.DB, roomID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	b, _ := json.Marshal(msgs)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleSearchMessagesFTS(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roomID := request.GetString("room_id", "")
	query := request.GetString("query", "")
	if roomID == "" || query == "" {
		return mcp.NewToolResultError("room_id and query are required"), nil
	}
	limit := request.GetInt("limit", 20)
	results, err := roomspkg.SearchFTSResults(s.DB, roomID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages fts: %w", err)
	}
	b, _ := json.Marshal(results)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleEditMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	if s.RateLimiter != nil && !s.RateLimiter.Allow(agentID) {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	messageID := request.GetString("message_id", "")
	body := request.GetString("body", "")
	if messageID == "" || body == "" {
		return mcp.NewToolResultError("message_id and body are required"), nil
	}

	msg, _ := roomspkg.GetMessage(s.DB, messageID)
	if err := roomspkg.EditMessage(s.DB, messageID, agentID, body); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	meta := map[string]any{}
	if msg != nil {
		meta["room_id"] = msg.RoomID
	}
	_ = auditpkg.Log(s.DB, agentID, "message.edited", "message", messageID, meta)

	if s.Subscriptions != nil && msg != nil {
		for _, sid := range s.Subscriptions.Subscribers(msg.RoomID) {
			_ = s.SendNotificationToSpecificClient(sid, mcp.MethodNotificationResourceUpdated, map[string]any{
				"uri": "rooms://" + msg.RoomID + "/messages",
			})
		}
	}

	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleDeleteMessage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	if s.RateLimiter != nil && !s.RateLimiter.Allow(agentID) {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	messageID := request.GetString("message_id", "")
	if messageID == "" {
		return mcp.NewToolResultError("message_id is required"), nil
	}

	msg, _ := roomspkg.GetMessage(s.DB, messageID)
	if err := roomspkg.DeleteMessage(s.DB, messageID, agentID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	meta := map[string]any{}
	if msg != nil {
		meta["room_id"] = msg.RoomID
	}
	_ = auditpkg.Log(s.DB, agentID, "message.deleted", "message", messageID, meta)

	if s.Subscriptions != nil && msg != nil {
		for _, sid := range s.Subscriptions.Subscribers(msg.RoomID) {
			_ = s.SendNotificationToSpecificClient(sid, mcp.MethodNotificationResourceUpdated, map[string]any{
				"uri": "rooms://" + msg.RoomID + "/messages",
			})
		}
	}

	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleRequestApproval(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	roomID := request.GetString("room_id", "")
	action := request.GetString("action", "")
	if roomID == "" || action == "" {
		return mcp.NewToolResultError("room_id and action are required"), nil
	}
	description := request.GetString("description", "")
	riskLevel := request.GetString("risk_level", "medium")

	id, err := approvalspkg.CreateApproval(s.DB, roomID, agentID, action, description, riskLevel)
	if err != nil {
		return nil, fmt.Errorf("create approval: %w", err)
	}

	result := map[string]any{
		"approval_id": id,
		"status":      "pending",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleCheckApproval(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	approvalID := request.GetString("approval_id", "")
	if approvalID == "" {
		return mcp.NewToolResultError("approval_id is required"), nil
	}
	ap, err := approvalspkg.GetApproval(s.DB, approvalID)
	if err != nil {
		return mcp.NewToolResultError("approval not found"), nil
	}
	result := map[string]any{
		"status": ap.Status,
	}
	if ap.Description != "" {
		result["note"] = ap.Description
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) agentIDFromContext(ctx context.Context) (string, bool) {
	token, ok := TokenFromContext(ctx)
	if !ok {
		return "", false
	}
	agent, err := dbpkg.GetAgentByKey(s.DB, token)
	if err != nil || agent == nil {
		return "", false
	}
	return agent.ID, true
}

func (s *Server) notImplemented(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (s *Server) handleAttachFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	messageID := request.GetString("message_id", "")
	filename := request.GetString("filename", "")
	contentType := request.GetString("content_type", "")
	storagePath := request.GetString("storage_path", "")
	sizeBytes := request.GetInt("size_bytes", 0)
	if messageID == "" || filename == "" || storagePath == "" {
		return mcp.NewToolResultError("message_id, filename and storage_path are required"), nil
	}
	attID, err := attachmentspkg.CreateAttachment(s.DB, messageID, filename, contentType, int64(sizeBytes), storagePath)
	if err != nil {
		return nil, fmt.Errorf("attach file: %w", err)
	}
	_ = auditpkg.Log(s.DB, agentID, "attachment.created", "attachment", attID, map[string]any{
		"message_id": messageID,
	})
	result := map[string]any{"attachment_id": attID}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleListAttachments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	messageID := request.GetString("message_id", "")
	if messageID == "" {
		return mcp.NewToolResultError("message_id is required"), nil
	}
	atts, err := attachmentspkg.ListAttachments(s.DB, messageID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	out := make([]map[string]any, 0, len(atts))
	for _, a := range atts {
		out = append(out, map[string]any{
			"id":           a.ID,
			"filename":     a.Filename,
			"content_type": a.ContentType,
			"size_bytes":   a.SizeBytes,
		})
	}
	b, _ := json.Marshal(out)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleAddReaction(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	if s.RateLimiter != nil && !s.RateLimiter.Allow(agentID) {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	messageID := request.GetString("message_id", "")
	emoji := request.GetString("emoji", "")
	if messageID == "" || emoji == "" {
		return mcp.NewToolResultError("message_id and emoji are required"), nil
	}
	msg, _ := roomspkg.GetMessage(s.DB, messageID)
	if err := reactionspkg.AddReaction(s.DB, messageID, agentID, emoji); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	meta := map[string]any{"emoji": emoji}
	if msg != nil {
		meta["room_id"] = msg.RoomID
	}
	_ = auditpkg.Log(s.DB, agentID, "reaction.added", "message", messageID, meta)
	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleRemoveReaction(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	if s.RateLimiter != nil && !s.RateLimiter.Allow(agentID) {
		return mcp.NewToolResultError("rate limit exceeded"), nil
	}
	messageID := request.GetString("message_id", "")
	emoji := request.GetString("emoji", "")
	if messageID == "" || emoji == "" {
		return mcp.NewToolResultError("message_id and emoji are required"), nil
	}
	msg, _ := roomspkg.GetMessage(s.DB, messageID)
	if err := reactionspkg.RemoveReaction(s.DB, messageID, agentID, emoji); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	meta := map[string]any{"emoji": emoji}
	if msg != nil {
		meta["room_id"] = msg.RoomID
	}
	_ = auditpkg.Log(s.DB, agentID, "reaction.removed", "message", messageID, meta)
	return mcp.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleListReactions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, ok := s.agentIDFromContext(ctx)
	if !ok {
		return mcp.NewToolResultError("unauthorized"), nil
	}
	messageID := request.GetString("message_id", "")
	if messageID == "" {
		return mcp.NewToolResultError("message_id is required"), nil
	}
	sums, err := reactionspkg.GetReactionSummary(s.DB, messageID)
	if err != nil {
		return nil, fmt.Errorf("list reactions: %w", err)
	}
	out := make([]map[string]any, 0, len(sums))
	for _, s := range sums {
		out = append(out, map[string]any{
			"emoji":  s.Emoji,
			"count":  s.Count,
			"agents": s.Agents,
		})
	}
	b, _ := json.Marshal(out)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleConnectPeer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	peerURL := request.GetString("peer_url", "")
	if peerURL == "" {
		return mcp.NewToolResultError("peer_url is required"), nil
	}
	if s.Relay == nil {
		s.Relay = relaypkg.New(s.DB)
	}
	if err := s.Relay.Connect(peerURL); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, _ := json.Marshal(map[string]any{"ok": true, "peers": s.Relay.ListPeers()})
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleListPeers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.Relay == nil {
		s.Relay = relaypkg.New(s.DB)
	}
	b, _ := json.Marshal(s.Relay.ListPeers())
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleDisconnectPeer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	peerURL := request.GetString("peer_url", "")
	if peerURL == "" {
		return mcp.NewToolResultError("peer_url is required"), nil
	}
	if s.Relay == nil {
		s.Relay = relaypkg.New(s.DB)
	}
	if err := s.Relay.Disconnect(peerURL); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, _ := json.Marshal(map[string]any{"ok": true, "peers": s.Relay.ListPeers()})
	return mcp.NewToolResultText(string(b)), nil
}
