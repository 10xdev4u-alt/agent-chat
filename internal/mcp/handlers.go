package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	approvalspkg "github.com/10xdev4u-alt/agent-chat/internal/approvals"
	auditpkg "github.com/10xdev4u-alt/agent-chat/internal/audit"
	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
	roomspkg "github.com/10xdev4u-alt/agent-chat/internal/rooms"
)

func (s *Server) handleRegister(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	roomID := request.GetString("room_id", "")
	if roomID == "" {
		return mcp.NewToolResultError("room_id is required"), nil
	}
	if err := roomspkg.JoinRoom(s.DB, roomID, agentID, "member"); err != nil {
		return nil, fmt.Errorf("join room: %w", err)
	}
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
	roomID := request.GetString("room_id", "")
	body := request.GetString("body", "")
	if roomID == "" || body == "" {
		return mcp.NewToolResultError("room_id and body are required"), nil
	}

	// Parse mentions from args if provided.
	mentions := request.GetStringSlice("mentions", nil)
	priority := request.GetString("priority", "normal")
	replyTo := request.GetString("reply_to", "")

	msgID, seq, err := roomspkg.SendMessage(s.DB, roomID, agentID, body, mentions, replyTo, priority)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	_ = auditpkg.Log(s.DB, agentID, "send_message", "message", msgID, map[string]any{
		"room_id": roomID,
		"seq":     seq,
	})

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
