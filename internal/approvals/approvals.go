package approvals

import (
	"database/sql"
	"fmt"
	"strings"

	"crypto/rand"
	"encoding/hex"
)

// Approval represents a pending/approved/denied/expired approval request.
type Approval struct {
	ID          string `json:"id"`
	RoomID      string `json:"room_id"`
	RequesterID string `json:"requester_id"`
	ApproverID  string `json:"approver_id,omitempty"`
	Action      string `json:"action"`
	Description string `json:"description,omitempty"`
	RiskLevel   string `json:"risk_level"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ResolvedAt  string `json:"resolved_at,omitempty"`
	Note        string `json:"note,omitempty"`
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

// CreateApproval inserts a new pending approval and returns its ID.
func CreateApproval(db *sql.DB, roomID, requesterID, action, description, riskLevel string) (string, error) {
	if roomID == "" {
		return "", fmt.Errorf("room_id is required")
	}
	if requesterID == "" {
		return "", fmt.Errorf("requester_id is required")
	}
	if action == "" {
		return "", fmt.Errorf("action is required")
	}
	if riskLevel == "" {
		riskLevel = "medium"
	}
	id := newID()
	if id == "" {
		return "", fmt.Errorf("generate id")
	}
	_, err := db.Exec(
		`INSERT INTO approvals (id, room_id, requester_id, action, description, risk_level) VALUES (?, ?, ?, ?, ?, ?)`,
		id, roomID, requesterID, action, nullable(description), riskLevel,
	)
	if err != nil {
		return "", fmt.Errorf("create approval: %w", err)
	}
	return id, nil
}

// RequestApproval is an alias for CreateApproval.
func RequestApproval(db *sql.DB, roomID, requesterID, action, description, riskLevel string) (string, error) {
	return CreateApproval(db, roomID, requesterID, action, description, riskLevel)
}

// GetApproval fetches an approval by ID.
func GetApproval(db *sql.DB, id string) (*Approval, error) {
	row := db.QueryRow(
		`SELECT id, room_id, requester_id, approver_id, action, description, risk_level, status, created_at, resolved_at FROM approvals WHERE id = ?`, id)
	var a Approval
	var approverID, description, resolvedAt sql.NullString
	err := row.Scan(&a.ID, &a.RoomID, &a.RequesterID, &approverID, &a.Action, &description, &a.RiskLevel, &a.Status, &a.CreatedAt, &resolvedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("approval not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	if approverID.Valid {
		a.ApproverID = approverID.String
	}
	if description.Valid {
		a.Description = description.String
	}
	if resolvedAt.Valid {
		a.ResolvedAt = resolvedAt.String
	}
	return &a, nil
}

// ListPending returns pending approvals for a room.
func ListPending(db *sql.DB, roomID string) ([]Approval, error) {
	query := `SELECT id, room_id, requester_id, approver_id, action, description, risk_level, status, created_at, resolved_at FROM approvals WHERE status = 'pending'`
	args := []any{}
	if roomID != "" {
		query += ` AND room_id = ?`
		args = append(args, roomID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		var a Approval
		var approverID, description, resolvedAt sql.NullString
		if err := rows.Scan(&a.ID, &a.RoomID, &a.RequesterID, &approverID, &a.Action, &description, &a.RiskLevel, &a.Status, &a.CreatedAt, &resolvedAt); err != nil {
			return nil, err
		}
		if approverID.Valid {
			a.ApproverID = approverID.String
		}
		if description.Valid {
			a.Description = description.String
		}
		if resolvedAt.Valid {
			a.ResolvedAt = resolvedAt.String
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Approve marks an approval as approved.
func Approve(db *sql.DB, approvalID, approverID, note string) error {
	if approvalID == "" {
		return fmt.Errorf("approval_id is required")
	}
	a, err := GetApproval(db, approvalID)
	if err != nil {
		return err
	}
	if a.Status != "pending" {
		return fmt.Errorf("approval is not pending: %s", a.Status)
	}
	res, err := db.Exec(
		`UPDATE approvals SET status = 'approved', approver_id = ?, resolved_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ? AND status = 'pending'`,
		nullable(approverID), approvalID,
	)
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("approval not updated: %s", approvalID)
	}
	_ = note
	return nil
}

// Deny marks an approval as denied.
func Deny(db *sql.DB, approvalID, approverID, note string) error {
	if approvalID == "" {
		return fmt.Errorf("approval_id is required")
	}
	a, err := GetApproval(db, approvalID)
	if err != nil {
		return err
	}
	if a.Status != "pending" {
		return fmt.Errorf("approval is not pending: %s", a.Status)
	}
	res, err := db.Exec(
		`UPDATE approvals SET status = 'denied', approver_id = ?, resolved_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ? AND status = 'pending'`,
		nullable(approverID), approvalID,
	)
	if err != nil {
		return fmt.Errorf("deny: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("approval not updated: %s", approvalID)
	}
	_ = note
	return nil
}

// ExpireStale expires pending approvals older than maxAgeMinutes.
func ExpireStale(db *sql.DB, maxAgeMinutes int) (int, error) {
	if maxAgeMinutes < 0 {
		maxAgeMinutes = 0
	}
	res, err := db.Exec(
		`UPDATE approvals SET status = 'expired', resolved_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE status = 'pending' AND created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d minutes", maxAgeMinutes),
	)
	if err != nil {
		return 0, fmt.Errorf("expire stale: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
