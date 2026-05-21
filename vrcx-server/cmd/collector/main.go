// Command collector maintains per-user VRChat WebSocket connections and
// persists feed events to Postgres.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/collector"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/credentials"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/feed"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	encKey := os.Getenv("COOKIE_ENCRYPTION_KEY")
	if encKey == "" {
		slog.Error("COOKIE_ENCRYPTION_KEY is required")
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

	creds, err := credentials.New(db, encKey)
	if err != nil {
		slog.Error("credentials", "error", err)
		os.Exit(1)
	}

	feedStore := feed.NewStore(db)
	mgr := collector.NewManager(creds, feedStore)

	slog.Info("collector starting")
	mgr.Run(ctx) // blocks until SIGTERM / Ctrl-C
	slog.Info("collector stopped")
}
