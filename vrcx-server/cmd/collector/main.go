// Command collector maintains per-user VRChat WebSocket connections and
// persists feed/friend-log events to Postgres.
// This is Phase 4 work; for now it just starts up and logs readiness.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/store"
)

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	slog.Info("collector ready — Phase 4 WebSocket logic not yet implemented")

	// Block forever (future: per-user goroutine pool maintaining wss:// connections)
	select {}
}
