package attachments

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// Attachment represents a file attached to a message.
type Attachment struct {
	ID          string `json:"id"`
	MessageID   string `json:"message_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	StoragePath string `json:"storage_path"`
	CreatedAt   string `json:"created_at"`
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func hasColumn(db *sql.DB, name string) bool {
	rows, err := db.Query("PRAGMA table_info(attachments)")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var cname, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if cname == name {
			return true
		}
	}
	return false
}

func newSchema(db *sql.DB) bool {
	return hasColumn(db, "size_bytes")
}

// CreateAttachment inserts a new attachment and returns its ID.
func CreateAttachment(db *sql.DB, messageID, filename, contentType string, sizeBytes int64, storagePath string) (string, error) {
	if messageID == "" {
		return "", fmt.Errorf("message_id is required")
	}
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	id := newID()
	if id == "" {
		return "", fmt.Errorf("generate id")
	}
	if newSchema(db) {
		_, err := db.Exec(
			`INSERT INTO attachments (id, message_id, filename, content_type, size_bytes, storage_path) VALUES (?, ?, ?, ?, ?, ?)`,
			id, messageID, filename, nullable(contentType), sizeBytes, storagePath,
		)
		if err != nil {
			return "", fmt.Errorf("create attachment: %w", err)
		}
	} else {
		_, err := db.Exec(
			`INSERT INTO attachments (id, message_id, filename, size, path) VALUES (?, ?, ?, ?, ?)`,
			id, messageID, filename, sizeBytes, storagePath,
		)
		if err != nil {
			return "", fmt.Errorf("create attachment: %w", err)
		}
	}
	return id, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetAttachment fetches an attachment by ID.
func GetAttachment(db *sql.DB, id string) (*Attachment, error) {
	var a Attachment
	if newSchema(db) {
		row := db.QueryRow(
			`SELECT id, message_id, filename, content_type, size_bytes, storage_path, created_at FROM attachments WHERE id = ?`, id)
		var ct sql.NullString
		err := row.Scan(&a.ID, &a.MessageID, &a.Filename, &ct, &a.SizeBytes, &a.StoragePath, &a.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment not found: %s", id)
		}
		if err != nil {
			return nil, err
		}
		if ct.Valid {
			a.ContentType = ct.String
		}
	} else {
		row := db.QueryRow(
			`SELECT id, message_id, filename, size, path, created_at FROM attachments WHERE id = ?`, id)
		err := row.Scan(&a.ID, &a.MessageID, &a.Filename, &a.SizeBytes, &a.StoragePath, &a.CreatedAt)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attachment not found: %s", id)
		}
		if err != nil {
			return nil, err
		}
	}
	if a.ID == "" {
		return nil, fmt.Errorf("attachment not found: %s", id)
	}
	return &a, nil
}

// ListAttachments returns attachments for a message.
func ListAttachments(db *sql.DB, messageID string) ([]Attachment, error) {
	var out []Attachment
	if newSchema(db) {
		rows, err := db.Query(
			`SELECT id, message_id, filename, content_type, size_bytes, storage_path, created_at FROM attachments WHERE message_id = ?`, messageID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var a Attachment
			var ct sql.NullString
			if err := rows.Scan(&a.ID, &a.MessageID, &a.Filename, &ct, &a.SizeBytes, &a.StoragePath, &a.CreatedAt); err != nil {
				return nil, err
			}
			if ct.Valid {
				a.ContentType = ct.String
			}
			out = append(out, a)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else {
		rows, err := db.Query(
			`SELECT id, message_id, filename, size, path, created_at FROM attachments WHERE message_id = ?`, messageID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var a Attachment
			if err := rows.Scan(&a.ID, &a.MessageID, &a.Filename, &a.SizeBytes, &a.StoragePath, &a.CreatedAt); err != nil {
				return nil, err
			}
			out = append(out, a)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	if out == nil {
		out = []Attachment{}
	}
	return out, nil
}

// DeleteAttachment deletes an attachment by ID.
func DeleteAttachment(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM attachments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("attachment not found: %s", id)
	}
	return nil
}

// TotalSizeForMessage returns sum of attachment sizes for a message.
func TotalSizeForMessage(db *sql.DB, messageID string) (int64, error) {
	var col string
	if newSchema(db) {
		col = "size_bytes"
	} else {
		col = "size"
	}
	var total sql.NullInt64
	err := db.QueryRow(`SELECT COALESCE(SUM(`+col+`), 0) FROM attachments WHERE message_id = ?`, messageID).Scan(&total)
	if err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}
