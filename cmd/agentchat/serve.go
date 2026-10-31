package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/10xdev4u-alt/agent-chat/internal/auth"
	"github.com/10xdev4u-alt/agent-chat/internal/db"
	"github.com/10xdev4u-alt/agent-chat/internal/mcp"
	relaypkg "github.com/10xdev4u-alt/agent-chat/internal/relay"
	"github.com/10xdev4u-alt/agent-chat/internal/web"
)

func runServe() {
	dbPath := "agentchat.db"
	if v := os.Getenv("AGENTCHAT_DB"); v != "" {
		dbPath = v
	}

	if err := db.Migrate(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}

	conn, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}

	auth.SetDB(conn)

	srv, err := mcp.NewServer(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp server: %v\n", err)
		os.Exit(1)
	}

	relay := relaypkg.New(conn)
	srv.Relay = relay

	// Mount web UI on / and MCP on /mcp.
	mux := http.NewServeMux()
	mux.Handle("/mcp", srv.HTTPServer)
	mux.Handle("/relay/ws", relay)
	mux.Handle("/", web.Handler(conn))

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Println("Starting agent-chat server on :8080")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "listen: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown: %v\n", err)
	}
	conn.Close()
}

func openDB(dbPath string) (*sql.DB, error) {
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	}
	return sql.Open("sqlite", dsn)
}
