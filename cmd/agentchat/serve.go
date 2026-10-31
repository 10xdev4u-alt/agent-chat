package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/10xdev4u-alt/agent-chat/internal/auth"
	"github.com/10xdev4u-alt/agent-chat/internal/db"
	"github.com/10xdev4u-alt/agent-chat/internal/mcp"
	"github.com/10xdev4u-alt/agent-chat/internal/oauth"
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
	oauth.SetDB(conn)

	srv, err := mcp.NewServer(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp server: %v\n", err)
		os.Exit(1)
	}

	// Mount web UI on / and MCP on /mcp.
	webHandler := web.Handler(conn)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/"):
			srv.HTTPServer.ServeHTTP(w, r)
		case r.URL.Path == "/relay/ws":
			srv.Relay.ServeHTTP(w, r)
		case r.URL.Path == "/.well-known/oauth-authorization-server":
			oauth.HandleDiscovery(w, r)
		case r.URL.Path == "/oauth/token":
			oauth.HandleToken(w, r)
		case r.URL.Path == "/oauth/authorize":
			oauth.HandleAuthorize(w, r)
		case r.URL.Path == "/oauth/register":
			oauth.HandleRegister(w, r)
		default:
			webHandler.ServeHTTP(w, r)
		}
	})

	httpSrv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
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
