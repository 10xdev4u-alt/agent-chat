package rooms

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	attachmentspkg "github.com/10xdev4u-alt/agent-chat/internal/attachments"
)

// Message represents a chat message.
type Message struct {
	ID          string                      `json:"id"`
	RoomID      string                      `json:"room_id"`
	SenderID    string                      `json:"sender_id"`
	Body        string                      `json:"body"`
	Mentions    []string                    `json:"mentions,omitempty"`
	ReplyTo     string                      `json:"reply_to,omitempty"`
	Priority    string                      `json:"priority"`
	Seq         int                         `json:"seq"`
	CreatedAt   string                      `json:"created_at"`
	EditedAt    string                      `json:"edited_at,omitempty"`
	DeletedAt   string                      `json:"deleted_at,omitempty"`
	Attachments []attachmentspkg.Attachment `json:"attachments,omitempty"`
}

func SendMessage(db *sql.DB, roomID, senderID, body string, mentions []string, replyTo, priority string, attachmentIDs ...any) (string, int, error) {
	if body == "" {
		return "", 0, fmt.Errorf("message body is required")
	}
	// Check membership.
	ok, err := IsMember(db, roomID, senderID)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return "", 0, fmt.Errorf("sender is not a member of room %s", roomID)
	}
	if priority == "" {
		priority = "normal"
	}

	// Normalize optional attachment IDs.
	var attachIDs []string
	for _, a := range attachmentIDs {
		switch v := a.(type) {
		case nil:
		case string:
			if v != "" {
				attachIDs = append(attachIDs, v)
			}
		case []string:
			attachIDs = append(attachIDs, v...)
		case []any:
			for _, x := range v {
				if xs, ok := x.(string); ok && xs != "" {
					attachIDs = append(attachIDs, xs)
				}
			}
		}
	}

	// Get next seq.
	var maxSeq sql.NullInt64
	err = db.QueryRow(`SELECT MAX(seq) FROM messages WHERE room_id = ?`, roomID).Scan(&maxSeq)
	if err != nil {
		return "", 0, fmt.Errorf("get max seq: %w", err)
	}
	seq := 1
	if maxSeq.Valid {
		seq = int(maxSeq.Int64) + 1
	}

	id := newID()
	mentionsJSON := "[]"
	if len(mentions) > 0 {
		b, _ := json.Marshal(mentions)
		mentionsJSON = string(b)
	}

	var replyToVal any
	if replyTo != "" {
		replyToVal = replyTo
	}

	_, err = db.Exec(
		`INSERT INTO messages (id, room_id, sender_id, body, mentions, reply_to, priority, seq) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, roomID, senderID, body, mentionsJSON, replyToVal, priority, seq,
	)
	if err != nil {
		return "", 0, fmt.Errorf("insert message: %w", err)
	}
	for _, aid := range attachIDs {
		_, _ = db.Exec(`UPDATE attachments SET message_id = ? WHERE id = ?`, id, aid)
	}
	return id, seq, nil
}

func ReadMessages(db *sql.DB, roomID string, afterSeq int, limit int) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT id, room_id, sender_id, body, mentions, reply_to, priority, seq, created_at, edited_at, deleted_at FROM messages WHERE room_id = ? AND deleted_at IS NULL`
	args := []any{roomID}
	if afterSeq > 0 {
		query += ` AND seq > ?`
		args = append(args, afterSeq)
	}
	query += ` ORDER BY seq ASC LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	populateAttachments(db, msgs)
	return msgs, nil
}

func SearchMessages(db *sql.DB, roomID, query string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := "%" + strings.ToLower(query) + "%"
	rows, err := db.Query(
		`SELECT id, room_id, sender_id, body, mentions, reply_to, priority, seq, created_at, edited_at, deleted_at FROM messages WHERE room_id = ? AND deleted_at IS NULL AND LOWER(body) LIKE ? ORDER BY seq DESC LIMIT ?`,
		roomID, q, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	populateAttachments(db, msgs)
	return msgs, nil
}

func GetMessage(db *sql.DB, id string) (*Message, error) {
	row := db.QueryRow(
		`SELECT id, room_id, sender_id, body, mentions, reply_to, priority, seq, created_at, edited_at, deleted_at FROM messages WHERE id = ?`, id)
	msgs, err := scanMessageRow(row)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("message not found: %s", id)
	}
	populateAttachments(db, msgs)
	return &msgs[0], nil
}

func UpdateReadCursor(db *sql.DB, agentID, roomID string, seq int) error {
	_, err := db.Exec(
		`INSERT INTO read_cursors (agent_id, room_id, last_seq) VALUES (?, ?, ?)
		 ON CONFLICT(agent_id, room_id) DO UPDATE SET last_seq = MAX(last_seq, excluded.last_seq)`,
		agentID, roomID, seq,
	)
	if err != nil {
		return fmt.Errorf("update read cursor: %w", err)
	}
	return nil
}

func GetReadCursor(db *sql.DB, agentID, roomID string) (int, error) {
	var seq int
	err := db.QueryRow(
		`SELECT last_seq FROM read_cursors WHERE agent_id = ? AND room_id = ?`,
		agentID, roomID,
	).Scan(&seq)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return seq, nil
}

func populateAttachments(db *sql.DB, msgs []Message) {
	for i := range msgs {
		atts, err := attachmentspkg.ListAttachments(db, msgs[i].ID)
		if err == nil {
			msgs[i].Attachments = atts
		}
	}
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	var msgs []Message
	for rows.Next() {
		var m Message
		var mentionsJSON sql.NullString
		var replyTo, editedAt, deletedAt sql.NullString
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Body, &mentionsJSON, &replyTo, &m.Priority, &m.Seq, &m.CreatedAt, &editedAt, &deletedAt); err != nil {
			return nil, err
		}
		if mentionsJSON.Valid && mentionsJSON.String != "" && mentionsJSON.String != "[]" {
			json.Unmarshal([]byte(mentionsJSON.String), &m.Mentions)
		}
		if replyTo.Valid {
			m.ReplyTo = replyTo.String
		}
		if editedAt.Valid {
			m.EditedAt = editedAt.String
		}
		if deletedAt.Valid {
			m.DeletedAt = deletedAt.String
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func scanMessageRow(row *sql.Row) ([]Message, error) {
	var m Message
	var mentionsJSON sql.NullString
	var replyTo, editedAt, deletedAt sql.NullString
	err := row.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Body, &mentionsJSON, &replyTo, &m.Priority, &m.Seq, &m.CreatedAt, &editedAt, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if mentionsJSON.Valid && mentionsJSON.String != "" && mentionsJSON.String != "[]" {
		json.Unmarshal([]byte(mentionsJSON.String), &m.Mentions)
	}
	if replyTo.Valid {
		m.ReplyTo = replyTo.String
	}
	if editedAt.Valid {
		m.EditedAt = editedAt.String
	}
	if deletedAt.Valid {
		m.DeletedAt = deletedAt.String
	}
	return []Message{m}, nil
}
