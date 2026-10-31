package main

import (
	"fmt"
	"os"

	"github.com/10xdev4u-alt/agent-chat/internal/db"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: agentchat <command>")
		fmt.Println("Commands: serve, migrate")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "migrate":
		if err := db.Migrate(":memory:"); err != nil {
			fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied.")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
