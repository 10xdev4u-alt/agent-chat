package db

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "agentchat-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if err := Migrate(tmpFile.Name()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	conn, err := sql.Open("sqlite", tmpFile.Name()+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Verify tables exist
	tables := []string{"agents", "rooms", "messages", "approvals", "audit_events", "attachments"}
	for _, tbl := range tables {
		var name string
		err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", tbl, err)
		}
	}

	// Verify foreign keys are enforced
	_, err = conn.Exec("INSERT INTO rooms (id, name, owner_id) VALUES ('r1', 'test', 'nonexistent')")
	if err == nil {
		t.Error("expected foreign key violation for nonexistent owner")
	}
}
