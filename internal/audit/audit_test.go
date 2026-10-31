package audit

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
)

func createAgent(t *testing.T, db *sql.DB, name string) string {
	id, _, err := dbpkg.CreateAgent(db, name, "h", "repo", "main", "agent")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func setupTestDB(t *testing.T) *sql.DB {
	tmpFile, err := os.CreateTemp("", "agentchat-audit-*.db")
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

func TestLogAndList(t *testing.T) {
	db := setupTestDB(t)
	actorID := createAgent(t, db, "alice")
	err := Log(db, actorID, "send_message", "message", "msg1", map[string]any{"room_id": "r1"})
	if err != nil {
		t.Fatal(err)
	}
	events, err := List(db, 10, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1, got %d", len(events))
	}
	if events[0].Action != "send_message" {
		t.Errorf("action mismatch")
	}
	if events[0].Metadata["room_id"] != "r1" {
		t.Errorf("metadata mismatch")
	}
}

func TestListFilters(t *testing.T) {
	db := setupTestDB(t)
	a1 := createAgent(t, db, "alice")
	a2 := createAgent(t, db, "bob")
	Log(db, a1, "act1", "t", "id1", nil)
	Log(db, a2, "act2", "t", "id2", nil)

	events, err := List(db, 10, 0, a1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1, got %d", len(events))
	}
	events2, _ := List(db, 10, 0, "", "act2")
	if len(events2) != 1 {
		t.Errorf("expected 1, got %d", len(events2))
	}
}

func TestLogValidation(t *testing.T) {
	db := setupTestDB(t)
	if err := Log(db, "a", "", "t", "id", nil); err == nil {
		t.Error("expected error")
	}
}

func TestListPagination(t *testing.T) {
	db := setupTestDB(t)
	for i := 0; i < 5; i++ {
		aid := createAgent(t, db, "agent")
		Log(db, aid, "act", "t", "id", nil)
	}
	page1, _ := List(db, 2, 0, "", "")
	if len(page1) != 2 {
		t.Errorf("expected 2, got %d", len(page1))
	}
	page2, _ := List(db, 2, 2, "", "")
	if len(page2) != 2 {
		t.Errorf("expected 2, got %d", len(page2))
	}
}
