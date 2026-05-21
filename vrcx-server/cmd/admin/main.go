// Command admin is the CLI for managing the VRCX Mobile deployment.
// Run via: vrcx-admin <subcommand> [args]
// Or in k8s: kubectl exec -n vrcx-mobile deploy/vrcx-proxy -- vrcx-admin allowlist list
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: vrcx-admin <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  allowlist list")
		fmt.Println("  allowlist add <vrchat-user-id> [note]")
		fmt.Println("  allowlist remove <vrchat-user-id>")
		os.Exit(1)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer db.Close()

	al := auth.NewAllowlist(db)

	cmd := os.Args[1]
	switch cmd {
	case "allowlist":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "allowlist requires a subcommand: list|add|remove")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			entries, err := al.List(ctx)
			if err != nil {
				fmt.Fprintln(os.Stderr, "list:", err)
				os.Exit(1)
			}
			if len(entries) == 0 {
				fmt.Println("(no entries)")
				return
			}
			fmt.Printf("%-30s %-40s %s\n", "VRChat User ID", "Note", "Added At")
			fmt.Printf("%-30s %-40s %s\n", "---", "---", "---")
			for _, e := range entries {
				fmt.Printf("%-30s %-40s %s\n", e.VRChatUserID, e.Note, e.AddedAt.Format("2006-01-02 15:04"))
			}
		case "add":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "allowlist add <vrchat-user-id> [note]")
				os.Exit(1)
			}
			userID := os.Args[3]
			note := ""
			if len(os.Args) >= 5 {
				note = os.Args[4]
			}
			if err := al.Add(ctx, userID, note); err != nil {
				fmt.Fprintln(os.Stderr, "add:", err)
				os.Exit(1)
			}
			fmt.Printf("Added %s to allowlist\n", userID)
		case "remove":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "allowlist remove <vrchat-user-id>")
				os.Exit(1)
			}
			userID := os.Args[3]
			if err := al.Remove(ctx, userID); err != nil {
				fmt.Fprintln(os.Stderr, "remove:", err)
				os.Exit(1)
			}
			fmt.Printf("Removed %s from allowlist\n", userID)
		default:
			fmt.Fprintln(os.Stderr, "unknown allowlist subcommand:", os.Args[2])
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		os.Exit(1)
	}
}
