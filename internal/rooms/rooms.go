package rooms

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// Room represents a chat room.
type Room struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OwnerID   string `json:"owner_id"`
	CreatedAt string `json:"created_at"`
}

// Member represents a room member.
type Member struct {
	RoomID   string `json:"room_id"`
	AgentID  string `json:"agent_id"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
	// Agent details (populated on list)
	Name    string `json:"name,omitempty"`
	Harness string `json:"harness,omitempty"`
	Status  string `json:"status,omitempty"`
	Doing   string `json:"doing,omitempty"`
}

func CreateRoom(db *sql.DB, name, ownerID string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("room name is required")
	}
	id := newID()
	_, err := db.Exec(
		`INSERT INTO rooms (id, name, owner_id) VALUES (?, ?, ?)`,
		id, name, ownerID,
	)
	if err != nil {
		return "", fmt.Errorf("create room: %w", err)
	}
	// Owner joins as owner.
	if err := JoinRoom(db, id, ownerID, "owner"); err != nil {
		return "", err
	}
	return id, nil
}

func GetRoom(db *sql.DB, id string) (*Room, error) {
	row := db.QueryRow(`SELECT id, name, owner_id, created_at FROM rooms WHERE id = ?`, id)
	var r Room
	if err := row.Scan(&r.ID, &r.Name, &r.OwnerID, &r.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("room not found: %s", id)
		}
		return nil, err
	}
	return &r, nil
}

func ListRooms(db *sql.DB) ([]Room, error) {
	rows, err := db.Query(`SELECT id, name, owner_id, created_at FROM rooms ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Name, &r.OwnerID, &r.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func JoinRoom(db *sql.DB, roomID, agentID, role string) error {
	if role == "" {
		role = "member"
	}
	_, err := db.Exec(
		`INSERT OR IGNORE INTO room_members (room_id, agent_id, role) VALUES (?, ?, ?)`,
		roomID, agentID, role,
	)
	if err != nil {
		return fmt.Errorf("join room: %w", err)
	}
	return nil
}

func LeaveRoom(db *sql.DB, roomID, agentID string) error {
	_, err := db.Exec(
		`DELETE FROM room_members WHERE room_id = ? AND agent_id = ?`,
		roomID, agentID,
	)
	if err != nil {
		return fmt.Errorf("leave room: %w", err)
	}
	return nil
}

func IsMember(db *sql.DB, roomID, agentID string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM room_members WHERE room_id = ? AND agent_id = ?`,
		roomID, agentID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func ListMembers(db *sql.DB, roomID string) ([]Member, error) {
	rows, err := db.Query(`
		SELECT rm.room_id, rm.agent_id, rm.role, rm.joined_at,
		       a.name, a.harness, a.status, a.doing
		FROM room_members rm
		JOIN agents a ON a.id = rm.agent_id
		WHERE rm.room_id = ?
		ORDER BY rm.joined_at`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		var name, harness, status, doing sql.NullString
		if err := rows.Scan(&m.RoomID, &m.AgentID, &m.Role, &m.JoinedAt, &name, &harness, &status, &doing); err != nil {
			return nil, err
		}
		m.Name = name.String
		m.Harness = harness.String
		m.Status = status.String
		m.Doing = doing.String
		members = append(members, m)
	}
	return members, rows.Err()
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
