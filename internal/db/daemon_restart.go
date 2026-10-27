package db

import (
	"context"
	"time"

	"github.com/reliant-labs/reliant/internal/store"
)

// SetDaemonRestartStatus records the owning stream's planned drain, including
// clearing it when a replacement process registers.
func (r *Repo) SetDaemonRestartStatus(ctx context.Context, userID, daemonID string, attachedAt time.Time, status string) error {
	return store.SetDaemonRestartStatus(ctx, r.ORM(), userID, daemonID, attachedAt, status)
}
