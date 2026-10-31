package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	approvalspkg "github.com/10xdev4u-alt/agent-chat/internal/approvals"
	dbpkg "github.com/10xdev4u-alt/agent-chat/internal/db"
	roomspkg "github.com/10xdev4u-alt/agent-chat/internal/rooms"
)

func setupWebDB(t *testing.T) *sql.DB {
	tmpFile, err := os.CreateTemp("", "agentchat-web-*.db")
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

func TestWebHandlersBasic(t *testing.T) {
	db := setupWebDB(t)
	h := Handler(db)

	for _, path := range []string{"/", "/rooms", "/approvals", "/audit"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, rr.Code)
		}
	}

	// POST approve/deny flow
	agentID, _, err := dbpkg.CreateAgent(db, "alice", "h", "repo", "main", "agent")
	if err != nil {
		t.Fatal(err)
	}
	roomID, err := roomspkg.CreateRoom(db, "r", agentID)
	if err != nil {
		t.Fatal(err)
	}
	approvalID, err := approvalspkg.CreateApproval(db, roomID, agentID, "action", "desc", "low")
	if err != nil {
		t.Fatal(err)
	}

	for _, verb := range []string{"approve", "deny"} {
		// recreate pending for second
		if verb == "deny" {
			approvalID, _ = approvalspkg.CreateApproval(db, roomID, agentID, "action2", "desc", "low")
		}
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/approvals/%s/%s", approvalID, verb), nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusSeeOther {
			t.Errorf("%s: expected 303, got %d", verb, rr.Code)
		}
	}
}
