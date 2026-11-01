package reactions

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/10xdev4u-alt/agent-chat/internal/db"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "agentchat-react-*.db")
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
	_, _ = db.Exec(`INSERT INTO agents (id, name, harness, api_key_hash) VALUES ('a1','alice','h','x')`)
	_, _ = db.Exec(`INSERT INTO agents (id, name, harness, api_key_hash) VALUES ('a2','bob','h','x')`)
	_, _ = db.Exec(`INSERT INTO rooms (id, name, owner_id) VALUES ('r1','r','a1')`)
	_, _ = db.Exec(`INSERT INTO room_members (room_id, agent_id, role) VALUES ('r1','a1','owner')`)
	msgID := "m1"
	_, _ = db.Exec(`INSERT INTO messages (id, room_id, sender_id, body, mentions, priority, seq) VALUES (?,?,?,?,?,?,?)`, msgID, "r1", "a1", "hi", "[]", "normal", 1)
	return msgID
}

func TestAddAndList(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	if err := AddReaction(db, mid, "a1", "👍"); err != nil {
		t.Fatal(err)
	}
	if err := AddReaction(db, mid, "a1", "👍"); err != nil {
		t.Fatal("idempotent add should not error")
	}
	rs, err := ListReactions(db, mid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(rs))
	}
	if rs[0].AgentName != "alice" {
		t.Fatalf("expected agent name alice, got %s", rs[0].AgentName)
	}
}

func TestRemove(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	_ = AddReaction(db, mid, "a1", "❤️")
	if err := RemoveReaction(db, mid, "a1", "❤️"); err != nil {
		t.Fatal(err)
	}
	rs, _ := ListReactions(db, mid)
	if len(rs) != 0 {
		t.Fatalf("expected 0, got %d", len(rs))
	}
}

func TestSummary(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	_ = AddReaction(db, mid, "a1", "👍")
	_ = AddReaction(db, mid, "a2", "👍")
	_ = AddReaction(db, mid, "a1", "🎉")
	sums, err := GetReactionSummary(db, mid)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(sums))
	}
	if sums[0].Emoji != "👍" || sums[0].Count != 2 {
		t.Fatalf("unexpected summary: %+v", sums[0])
	}
	if len(sums[0].Agents) != 2 {
		t.Fatalf("expected 2 agents")
	}
}

func TestEmpty(t *testing.T) {
	db := setupDB(t)
	mid := seedMessage(t, db)
	rs, err := ListReactions(db, mid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 0 {
		t.Fatalf("expected empty")
	}
	sums, _ := GetReactionSummary(db, mid)
	if len(sums) != 0 {
		t.Fatalf("expected empty summary")
	}
}
