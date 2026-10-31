package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleRegister(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("name", "")
	harness := request.GetString("harness", "")
	repo := request.GetString("repo", "")
	branch := request.GetString("branch", "")

	if name == "" || harness == "" {
		return mcp.NewToolResultError("name and harness are required"), nil
	}

	agentID := uuid.New().String()
	apiKey := uuid.New().String()
	hash := sha256Hex(apiKey)

	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO agents (id, name, harness, repo, branch, api_key_hash, status) VALUES (?, ?, ?, ?, ?, ?, 'online')`,
		agentID, name, harness, nullable(repo), nullable(branch), hash,
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
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

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
