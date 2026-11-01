package presence

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Presence struct {
	AgentID  string `json:"agent_id"`
	Status   string `json:"status"`
	LastSeen string `json:"last_seen"`
	IsOnline bool   `json:"is_online"`
	Doing    string `json:"doing,omitempty"`
}

const staleThresholdMinutes = 5

func Heartbeat(db *sql.DB, agentID string) error {
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	res, err := db.Exec(
		`UPDATE agents SET last_seen = strftime('%Y-%m-%dT%H:%M:%fZ','now'), status = 'online', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		agentID,
	)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("agent not found: %s", agentID)
	}
	return nil
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func isOnline(status, lastSeen string) bool {
	if status == "offline" {
		return false
	}
	t, ok := parseTime(lastSeen)
	if !ok {
		return false
	}
	return time.Since(t) <= staleThresholdMinutes*time.Minute
}

func GetPresence(db *sql.DB, agentID string) (*Presence, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	row := db.QueryRow(`SELECT id, status, doing, last_seen FROM agents WHERE id = ?`, agentID)
	var p Presence
	var doingNS, lastSeenNS sql.NullString
	err := row.Scan(&p.AgentID, &p.Status, &doingNS, &lastSeenNS)
	if err != nil {
		return nil, fmt.Errorf("get presence: %w", err)
	}
	p.Doing = doingNS.String
	p.LastSeen = lastSeenNS.String
	p.IsOnline = isOnline(p.Status, p.LastSeen)
	return &p, nil
}

func MarkStaleOffline(db *sql.DB, staleMinutes int) (int, error) {
	if staleMinutes <= 0 {
		staleMinutes = staleThresholdMinutes
	}
	modifier := fmt.Sprintf("-%d minutes", staleMinutes)
	res, err := db.Exec(
		`UPDATE agents SET status = 'offline', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		 WHERE status != 'offline' AND (last_seen IS NULL OR last_seen < strftime('%Y-%m-%dT%H:%M:%fZ','now', ?))`,
		modifier,
	)
	if err != nil {
		return 0, fmt.Errorf("mark stale offline: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func ListPresence(db *sql.DB) ([]Presence, error) {
	rows, err := db.Query(`SELECT id, status, doing, last_seen FROM agents ORDER BY created_at`)
	if err != nil {
		rows, err = db.Query(`SELECT id, status, doing, last_seen FROM agents`)
		if err != nil {
			return nil, fmt.Errorf("list presence: %w", err)
		}
	}
	defer rows.Close()
	var out []Presence
	for rows.Next() {
		var p Presence
		var doingNS, lastSeenNS sql.NullString
		if err := rows.Scan(&p.AgentID, &p.Status, &doingNS, &lastSeenNS); err != nil {
			return nil, fmt.Errorf("scan presence: %w", err)
		}
		p.Doing = doingNS.String
		p.LastSeen = lastSeenNS.String
		p.IsOnline = isOnline(p.Status, p.LastSeen)
		out = append(out, p)
	}
	return out, rows.Err()
}
