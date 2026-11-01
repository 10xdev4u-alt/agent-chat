package rooms

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	tmpFile, err := os.CreateTemp("", "agentchat-rooms-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if err := dbpkg.Migrate(tmpFile.Name()); err != nil {
		t.Fatal(err)
	}
	conn, err := sql.Open("sqlite", tmpFile.Name()+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestCreateRoom(t *testing.T) {
	db := setupTestDB(t)
	// Create an agent first.
	agentID, _, err := createTestAgent(db, "alice", "claude-code", "repo", "main")
	if err != nil {
		t.Fatal(err)
	}
	roomID, err := CreateRoom(db, "general", agentID)
	if err != nil {
		t.Fatal(err)
	}
	if roomID == "" {
		t.Fatal("expected room ID")
	}
	room, err := GetRoom(db, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if room.Name != "general" {
		t.Errorf("expected name general, got %s", room.Name)
	}
}

func TestJoinLeaveRoom(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	agent2, _, _ := createTestAgent(db, "bob", "codex", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	if err := JoinRoom(db, roomID, agent2, "member"); err != nil {
		t.Fatal(err)
	}
	members, err := ListMembers(db, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	ok, _ := IsMember(db, roomID, agent2)
	if !ok {
		t.Error("agent2 should be a member")
	}
	if err := LeaveRoom(db, roomID, agent2); err != nil {
		t.Fatal(err)
	}
	ok, _ = IsMember(db, roomID, agent2)
	if ok {
		t.Error("agent2 should not be a member after leave")
	}
}

func TestSendMessage(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	msgID, seq, err := SendMessage(db, roomID, agent1, "hello world", nil, "", "normal")
	if err != nil {
		t.Fatal(err)
	}
	if msgID == "" || seq != 1 {
		t.Errorf("expected msgID and seq 1, got %s %d", msgID, seq)
	}

	// Second message should have seq 2.
	_, seq2, err := SendMessage(db, roomID, agent1, "second message", nil, "", "normal")
	if err != nil {
		t.Fatal(err)
	}
	if seq2 != 2 {
		t.Errorf("expected seq 2, got %d", seq2)
	}
}

func TestReadMessages(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	SendMessage(db, roomID, agent1, "msg1", nil, "", "normal")
	SendMessage(db, roomID, agent1, "msg2", nil, "", "normal")
	SendMessage(db, roomID, agent1, "msg3", nil, "", "normal")

	// Read all.
	msgs, err := ReadMessages(db, roomID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}

	// Read after seq 1.
	msgs, err = ReadMessages(db, roomID, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages after seq 1, got %d", len(msgs))
	}
}

func TestNonMemberCannotSend(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	agent2, _, _ := createTestAgent(db, "bob", "codex", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	_, _, err := SendMessage(db, roomID, agent2, "hello", nil, "", "normal")
	if err == nil {
		t.Error("expected error for non-member sending message")
	}
}

func TestSearchMessages(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	SendMessage(db, roomID, agent1, "hello world", nil, "", "normal")
	SendMessage(db, roomID, agent1, "goodbye world", nil, "", "normal")

	results, err := SearchMessages(db, roomID, "hello", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchMessagesFTS(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	SendMessage(db, roomID, agent1, "hello world", nil, "", "normal")
	SendMessage(db, roomID, agent1, "goodbye world", nil, "", "normal")
	SendMessage(db, roomID, agent1, "hello again", nil, "", "normal")

	like, err := SearchMessages(db, roomID, "hello", 10)
	if err != nil {
		t.Fatal(err)
	}
	fts, err := SearchMessagesFTS(db, roomID, "hello", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(fts) != len(like) {
		t.Errorf("FTS count %d != LIKE count %d", len(fts), len(like))
	}
	if len(fts) != 2 {
		t.Errorf("expected 2 results, got %d", len(fts))
	}
}

func TestReadCursor(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	cursor, err := GetReadCursor(db, agent1, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 0 {
		t.Errorf("expected cursor 0, got %d", cursor)
	}

	if err := UpdateReadCursor(db, agent1, roomID, 5); err != nil {
		t.Fatal(err)
	}
	cursor, _ = GetReadCursor(db, agent1, roomID)
	if cursor != 5 {
		t.Errorf("expected cursor 5, got %d", cursor)
	}
}

// dbpkgCreateAgent is a test helper to create an agent.
func createTestAgent(db *sql.DB, name, harness, repo, branch string) (string, string, error) {
	return dbpkg.CreateAgent(db, name, harness, repo, branch, "agent")
}

func TestEditMessage(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	agent2, _, _ := createTestAgent(db, "bob", "codex", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)

	msgID, _, _ := SendMessage(db, roomID, agent1, "hello", nil, "", "normal")

	// Non-sender cannot edit.
	if err := EditMessage(db, msgID, agent2, "changed"); err == nil {
		t.Error("expected error for non-sender edit")
	}

	// Sender can edit.
	if err := EditMessage(db, msgID, agent1, "hello edited"); err != nil {
		t.Fatal(err)
	}
	msg, err := GetMessage(db, msgID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "hello edited" {
		t.Errorf("expected edited body, got %s", msg.Body)
	}
	if msg.EditedAt == "" {
		t.Error("expected edited_at set")
	}
	if IsDeleted(*msg) {
		t.Error("should not be deleted")
	}

	// Empty body rejected.
	if err := EditMessage(db, msgID, agent1, ""); err == nil {
		t.Error("expected error for empty body")
	}
}

func TestDeleteMessage(t *testing.T) {
	db := setupTestDB(t)
	agent1, _, _ := createTestAgent(db, "alice", "claude-code", "repo", "main")
	agent2, _, _ := createTestAgent(db, "bob", "codex", "repo", "main")
	agent3, _, _ := createTestAgent(db, "carol", "codex", "repo", "main")
	roomID, _ := CreateRoom(db, "general", agent1)
	JoinRoom(db, roomID, agent2, "member")
	JoinRoom(db, roomID, agent3, "member")

	msgID, _, _ := SendMessage(db, roomID, agent2, "hello", nil, "", "normal")

	// Non-sender non-owner cannot delete.
	if err := DeleteMessage(db, msgID, agent3); err == nil {
		t.Error("expected error for unauthorized delete")
	}

	// Sender can delete.
	if err := DeleteMessage(db, msgID, agent2); err != nil {
		t.Fatal(err)
	}
	msg, err := GetMessage(db, msgID)
	if err != nil {
		t.Fatal(err)
	}
	if !IsDeleted(*msg) {
		t.Error("expected deleted")
	}
	// ReadMessages excludes deleted.
	msgs, _ := ReadMessages(db, roomID, 0, 50)
	if len(msgs) != 0 {
		t.Errorf("expected 0 live messages, got %d", len(msgs))
	}

	// Room owner can delete another's message.
	msgID2, _, _ := SendMessage(db, roomID, agent2, "second", nil, "", "normal")
	if err := DeleteMessage(db, msgID2, agent1); err != nil {
		t.Fatal(err)
	}
	msg2, _ := GetMessage(db, msgID2)
	if !IsDeleted(*msg2) {
		t.Error("expected deleted by owner")
	}

	// Idempotent-ish: deleting already deleted returns nil.
	if err := DeleteMessage(db, msgID, agent2); err != nil {
		t.Errorf("expected nil on already deleted, got %v", err)
	}
}
