package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
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

func (s *Server) notImplemented(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("not implemented"), nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// Helper to get agent ID from context (set by auth middleware).
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
