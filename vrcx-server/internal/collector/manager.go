package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/credentials"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/feed"
)

const refreshInterval = 5 * time.Minute

// Manager watches the credential store and keeps one Worker alive per user.
type Manager struct {
	creds     *credentials.Store
	feedStore *feed.Store

	mu      sync.Mutex
	workers map[string]context.CancelFunc // vrchatUserID → cancel func
}

// NewManager creates a Manager.
func NewManager(creds *credentials.Store, feedStore *feed.Store) *Manager {
	return &Manager{
		creds:     creds,
		feedStore: feedStore,
		workers:   make(map[string]context.CancelFunc),
	}
}

// Run blocks until ctx is cancelled, refreshing the worker pool every 5 min.
func (m *Manager) Run(ctx context.Context) {
	m.refresh(ctx)
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.stopAll()
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

func (m *Manager) refresh(ctx context.Context) {
	ids, err := m.creds.LoadAll(ctx)
	if err != nil {
		slog.Error("manager: loadAll credentials", "err", err)
		return
	}

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel workers for removed users
	for id, cancel := range m.workers {
		if !idSet[id] {
			slog.Info("manager: stopping worker", "vrchatUserID", id)
			cancel()
			delete(m.workers, id)
		}
	}

	// Spawn workers for new users
	for _, id := range ids {
		if _, running := m.workers[id]; running {
			continue
		}
		slog.Info("manager: starting worker", "vrchatUserID", id)
		wctx, cancel := context.WithCancel(ctx)
		m.workers[id] = cancel
		w := newWorker(id, m.creds, m.feedStore)
		go w.Run(wctx)
	}
}

func (m *Manager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cancel := range m.workers {
		slog.Info("manager: stopping worker (shutdown)", "vrchatUserID", id)
		cancel()
		delete(m.workers, id)
	}
}
