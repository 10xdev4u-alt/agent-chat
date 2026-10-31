package approvals

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	tmpFile, err := os.CreateTemp("", "agentchat-approvals-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if err := dbpkg.Migrate(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("sqlite", tmpFile.Name()+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func createAgent(t *testing.T, db *sql.DB, name string) string {
	id, _, err := dbpkg.CreateAgent(db, name, "h", "repo", "main", "agent")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func createRoom(t *testing.T, db *sql.DB, ownerID string) string {
	// use rooms package? Avoid import cycle; insert directly.
	id := newID()
	_, err := db.Exec(`INSERT INTO rooms (id, name, owner_id) VALUES (?, ?, ?)`, id, "r", ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO room_members (room_id, agent_id, role) VALUES (?, ?, ?)`, id, ownerID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateAndGetApproval(t *testing.T) {
	db := setupTestDB(t)
	agentID := createAgent(t, db, "alice")
	roomID := createRoom(t, db, agentID)

	id, err := CreateApproval(db, roomID, agentID, "deploy", "desc", "high")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected id")
	}
	ap, err := GetApproval(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if ap.Status != "pending" {
		t.Errorf("expected pending, got %s", ap.Status)
	}
	if ap.RiskLevel != "high" {
		t.Errorf("expected high, got %s", ap.RiskLevel)
	}
	if ap.Description != "desc" {
		t.Errorf("expected desc, got %s", ap.Description)
	}
}

func TestApproveDeny(t *testing.T) {
	db := setupTestDB(t)
	agentID := createAgent(t, db, "alice")
	approverID := createAgent(t, db, "bob")
	roomID := createRoom(t, db, agentID)

	id1, _ := CreateApproval(db, roomID, agentID, "a1", "", "low")
	if err := Approve(db, id1, approverID, "ok"); err != nil {
		t.Fatal(err)
	}
	ap, _ := GetApproval(db, id1)
	if ap.Status != "approved" {
		t.Errorf("expected approved, got %s", ap.Status)
	}
	if ap.ApproverID != approverID {
		t.Errorf("approver mismatch")
	}
	if ap.ResolvedAt == "" {
		t.Error("expected resolved_at")
	}

	id2, _ := CreateApproval(db, roomID, agentID, "a2", "", "low")
	if err := Deny(db, id2, approverID, "no"); err != nil {
		t.Fatal(err)
	}
	ap2, _ := GetApproval(db, id2)
	if ap2.Status != "denied" {
		t.Errorf("expected denied, got %s", ap2.Status)
	}
}

func TestApproveNonPending(t *testing.T) {
	db := setupTestDB(t)
	agentID := createAgent(t, db, "alice")
	roomID := createRoom(t, db, agentID)
	id, _ := CreateApproval(db, roomID, agentID, "a", "", "low")
	Approve(db, id, agentID, "")
	if err := Approve(db, id, agentID, ""); err == nil {
		t.Error("expected error on double approve")
	}
}

func TestListPending(t *testing.T) {
	db := setupTestDB(t)
	agentID := createAgent(t, db, "alice")
	roomID := createRoom(t, db, agentID)
	CreateApproval(db, roomID, agentID, "a1", "", "low")
	id2, _ := CreateApproval(db, roomID, agentID, "a2", "", "low")
	Approve(db, id2, agentID, "")

	pending, err := ListPending(db, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
}

func TestExpireStale(t *testing.T) {
	db := setupTestDB(t)
	agentID := createAgent(t, db, "alice")
	roomID := createRoom(t, db, agentID)
	id, _ := CreateApproval(db, roomID, agentID, "a", "", "low")
	// Make it old by updating created_at
	_, err := db.Exec(`UPDATE approvals SET created_at = datetime('now', '-100 minutes') WHERE id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	n, err := ExpireStale(db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 expired, got %d", n)
	}
	ap, _ := GetApproval(db, id)
	if ap.Status != "expired" {
		t.Errorf("expected expired, got %s", ap.Status)
	}
}

func TestGetApprovalNotFound(t *testing.T) {
	db := setupTestDB(t)
	if _, err := GetApproval(db, "nope"); err == nil {
		t.Error("expected error")
	}
}
