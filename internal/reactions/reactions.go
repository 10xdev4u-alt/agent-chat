package reactions

import (
	"database/sql"
	"fmt"

	"crypto/rand"
	"encoding/hex"
)

// Reaction represents an emoji reaction on a message.
type Reaction struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id"`
	AgentID   string `json:"agent_id"`
	Emoji     string `json:"emoji"`
	CreatedAt string `json:"created_at"`
	AgentName string `json:"agent_name,omitempty"`
}

// ReactionSummary aggregates reactions for a message by emoji.
type ReactionSummary struct {
	Emoji  string   `json:"emoji"`
	Count  int      `json:"count"`
	Agents []string `json:"agents"`
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// AddReaction adds an emoji reaction to a message. Idempotent: duplicate
// (message, agent, emoji) tuples are ignored.
func AddReaction(db *sql.DB, messageID, agentID, emoji string) error {
	if messageID == "" {
		return fmt.Errorf("message_id is required")
	}
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}
	id := newID()
	if id == "" {
		return fmt.Errorf("generate id")
	}
	_, err := db.Exec(
		`INSERT OR IGNORE INTO message_reactions (id, message_id, agent_id, emoji) VALUES (?, ?, ?, ?)`,
		id, messageID, agentID, emoji,
	)
	if err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

// RemoveReaction removes an emoji reaction from a message.
func RemoveReaction(db *sql.DB, messageID, agentID, emoji string) error {
	if messageID == "" {
		return fmt.Errorf("message_id is required")
	}
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if emoji == "" {
		return fmt.Errorf("emoji is required")
	}
	_, err := db.Exec(
		`DELETE FROM message_reactions WHERE message_id = ? AND agent_id = ? AND emoji = ?`,
		messageID, agentID, emoji,
	)
	if err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	return nil
}

// ListReactions returns all reactions for a message, including agent names.
func ListReactions(db *sql.DB, messageID string) ([]Reaction, error) {
	rows, err := db.Query(
		`SELECT r.id, r.message_id, r.agent_id, r.emoji, r.created_at, COALESCE(a.name, '')
		 FROM message_reactions r LEFT JOIN agents a ON a.id = r.agent_id
		 WHERE r.message_id = ? ORDER BY r.created_at ASC`,
		messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reaction
	for rows.Next() {
		var r Reaction
		if err := rows.Scan(&r.ID, &r.MessageID, &r.AgentID, &r.Emoji, &r.CreatedAt, &r.AgentName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Reaction{}
	}
	return out, nil
}

// GetReactionSummary returns reactions grouped by emoji with counts and agent names.
func GetReactionSummary(db *sql.DB, messageID string) ([]ReactionSummary, error) {
	reactions, err := ListReactions(db, messageID)
	if err != nil {
		return nil, err
	}
	order := []string{}
	groups := map[string][]string{}
	counts := map[string]int{}
	for _, r := range reactions {
		if counts[r.Emoji] == 0 {
			order = append(order, r.Emoji)
		}
		counts[r.Emoji]++
		name := r.AgentName
		if name == "" {
			name = r.AgentID
		}
		groups[r.Emoji] = append(groups[r.Emoji], name)
	}
	out := make([]ReactionSummary, 0, len(order))
	for _, e := range order {
		agents := groups[e]
		if agents == nil {
			agents = []string{}
		}
		out = append(out, ReactionSummary{
			Emoji:  e,
			Count:  counts[e],
			Agents: agents,
		})
	}
	if out == nil {
		out = []ReactionSummary{}
	}
	return out, nil
}
