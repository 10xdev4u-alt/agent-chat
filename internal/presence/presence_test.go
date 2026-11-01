package presence

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "agentchat-presence-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	if err := dbpkg.Migrate(tmpFile.Name()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conn, err := sql.Open("sqlite", tmpFile.Name()+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestHeartbeatAndGetPresence(t *testing.T) {
	db := openTestDB(t)
	id, _, err := dbpkg.CreateAgent(db, "a1", "h", "", "", "agent")
	if err != nil {
		t.Fatal(err)
	}
	if err := Heartbeat(db, id); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	p, err := GetPresence(db, id)
	if err != nil {
		t.Fatalf("get presence: %v", err)
	}
	if p.Status != "online" || p.LastSeen == "" || !p.IsOnline {
		t.Errorf("bad presence: %+v", p)
	}
}

func TestMarkStaleOffline(t *testing.T) {
	db := openTestDB(t)
	id, _, err := dbpkg.CreateAgent(db, "a2", "h", "", "", "agent")
	if err != nil {
		t.Fatal(err)
	}
	if err := Heartbeat(db, id); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE agents SET last_seen = strftime('%Y-%m-%dT%H:%M:%fZ','now','-10 minutes') WHERE id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	n, err := MarkStaleOffline(db, 5)
	if err != nil || n != 1 {
		t.Fatalf("mark stale: n=%d err=%v", n, err)
	}
	p, _ := GetPresence(db, id)
	if p.Status != "offline" || p.IsOnline {
		t.Errorf("bad: %+v", p)
	}
}

func TestListPresence(t *testing.T) {
	db := openTestDB(t)
	_, _, _ = dbpkg.CreateAgent(db, "a3", "h", "", "", "agent")
	_, _, _ = dbpkg.CreateAgent(db, "a4", "h", "", "", "agent")
	list, err := ListPresence(db)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
}
