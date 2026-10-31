package mcp

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/10xdev4u-alt/agent-chat/internal/db"
)

func TestNewServerToolRegistration(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "agentchat-mcp-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if err := db.Migrate(tmpFile.Name()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	conn, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	srv, err := NewServer(conn)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if srv == nil {
		t.Fatal("nil server")
	}

	tools := srv.ListTools()
	if len(tools) < 11 {
		t.Fatalf("expected at least 11 tools, got %d", len(tools))
	}
	if _, ok := tools["register"]; !ok {
		t.Fatal("register tool not registered")
	}
}

func TestResourceRegistration(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "agentchat-mcp-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if err := db.Migrate(tmpFile.Name()); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	conn, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	srv, err := NewServer(conn)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	resources := srv.ListResources()
	if _, ok := resources["rooms://"]; !ok {
		t.Fatal("rooms:// resource not registered")
	}
	if _, ok := resources["agents://"]; !ok {
		t.Fatal("agents:// resource not registered")
	}
	if srv.Subscriptions == nil {
		t.Fatal("subscription manager not initialized")
	}
}

func TestSubscriptionManager(t *testing.T) {
	m := NewSubscriptionManager()
	m.Subscribe("room1", "sess1")
	m.Subscribe("room1", "sess1")
	m.Subscribe("room1", "sess2")
	got := m.Subscribers("room1")
	if len(got) != 2 {
		t.Fatalf("expected 2 subscribers, got %d", len(got))
	}
	m.Unsubscribe("room1", "sess1")
	got = m.Subscribers("room1")
	if len(got) != 1 || got[0] != "sess2" {
		t.Fatalf("unexpected subscribers: %v", got)
	}
}
