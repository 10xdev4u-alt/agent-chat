package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"crypto/rand"
	"encoding/hex"
)

// AuditEvent represents an audited action.
type AuditEvent struct {
	ID         string         `json:"id"`
	ActorID    string         `json:"actor_id,omitempty"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  string         `json:"created_at"`
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// Log records an audit event.
func Log(db *sql.DB, actorID, action, targetType, targetID string, metadata map[string]any) error {
	if action == "" {
		return fmt.Errorf("action is required")
	}
	if targetType == "" {
		return fmt.Errorf("target_type is required")
	}
	if targetID == "" {
		return fmt.Errorf("target_id is required")
	}
	id := newID()
	if id == "" {
		return fmt.Errorf("generate id")
	}
	var meta any
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		meta = string(b)
	}
	_, err := db.Exec(
		`INSERT INTO audit_events (id, actor_id, action, target_type, target_id, metadata) VALUES (?, ?, ?, ?, ?, ?)`,
		id, nullable(actorID), action, targetType, targetID, meta,
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// List returns audit events with optional filters.
func List(db *sql.DB, limit, offset int, actorID, action string) ([]AuditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT id, actor_id, action, target_type, target_id, metadata, created_at FROM audit_events WHERE 1=1`
	args := []any{}
	if actorID != "" {
		query += ` AND actor_id = ?`
		args = append(args, actorID)
	}
	if action != "" {
		query += ` AND action = ?`
		args = append(args, action)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var actorIDNull, metadataNull sql.NullString
		if err := rows.Scan(&e.ID, &actorIDNull, &e.Action, &e.TargetType, &e.TargetID, &metadataNull, &e.CreatedAt); err != nil {
			return nil, err
		}
		if actorIDNull.Valid {
			e.ActorID = actorIDNull.String
		}
		if metadataNull.Valid && metadataNull.String != "" {
			var m map[string]any
			if err := json.Unmarshal([]byte(metadataNull.String), &m); err == nil {
				e.Metadata = m
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
