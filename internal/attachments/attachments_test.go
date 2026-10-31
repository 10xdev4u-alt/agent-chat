package attachments

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/10xdev4u-alt/agent-chat/internal/db"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "agentchat-attach-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	if err := db.Migrate(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func seedMessage(t *testing.T, db *sql.DB) string {
	t.Helper()
	_, _ = db.Exec(`INSERT INTO agents (id, name, harness) VALUES ('a1','a','h')`)
	_, _ = db.Exec(`INSERT INTO rooms (id, name, owner_id) VALUES ('r1','r','a1')`)
	_, _ = db.Exec(`INSERT INTO room_members (room_id, agent_id, role) VALUES ('r1','a1','owner')`)
	msgID := "m1"
	_, _ = db.Exec(`INSERT INTO messages (id, room_id, sender_id, body, mentions, priority, seq) VALUES (?,?,?,?,?,?,?)`, msgID, "r1", "a1", "hi", "[]", "normal", 1)
	return msgID
}

func TestCreateAndGet(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	id, err := CreateAttachment(db, mid, "file.txt", "text/plain", 10, "/tmp/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	a, err := GetAttachment(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if a.Filename != "file.txt" || a.SizeBytes != 10 || a.StoragePath != "/tmp/file.txt" || a.ContentType != "text/plain" {
		t.Fatalf("unexpected: %+v", a)
	}
}

func TestListAndTotal(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	CreateAttachment(db, mid, "a", "t", 5, "p1")
	CreateAttachment(db, mid, "b", "t", 7, "p2")
	atts, err := ListAttachments(db, mid)
	if err != nil {
		t.Fatal(err)
	}
	if len(atts) != 2 {
		t.Fatalf("expected 2, got %d", len(atts))
	}
	total, err := TotalSizeForMessage(db, mid)
	if err != nil {
		t.Fatal(err)
	}
	if total != 12 {
		t.Fatalf("expected 12, got %d", total)
	}
}

func TestDelete(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	id, _ := CreateAttachment(db, mid, "a", "t", 5, "p1")
	if err := DeleteAttachment(db, id); err != nil {
		t.Fatal(err)
	}
	if _, err := GetAttachment(db, id); err == nil {
		t.Fatal("expected not found")
	}
}
