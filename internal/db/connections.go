// Copyright (c) 2025 Reliant Labs

package db

import (
	"context"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"

	"github.com/reliant-labs/reliant/internal/db/core"
	"github.com/reliant-labs/reliant/internal/store"
)

// Connections returns the store for user connections (the credential vault's
// metadata and ciphertext tables, their audit trail, and OAuth flow state).
func (r *Repo) Connections() core.ConnectionStore {
	return connectionStore{db: r.ORM()}
}

// connectionStore is core.ConnectionStore over the connection queries in
// internal/store. A call made with a ctx carrying a transaction joins it.
type connectionStore struct{ db orm.Context }

func (s connectionStore) CreateConnection(ctx context.Context, c *core.Connection, secrets []core.ConnectionSecret, ev core.ConnectionEvent) error {
	return store.InsertUserConnection(ctx, s.db, c, secrets, ev)
}

func (s connectionStore) ReauthorizeConnection(ctx context.Context, userID, id string, upd core.ConnectionUpdate, secrets []core.ConnectionSecret, ev core.ConnectionEvent) (*core.Connection, error) {
	return store.ReauthorizeConnection(ctx, s.db, userID, id, upd, secrets, ev)
}

func (s connectionStore) GetConnection(ctx context.Context, userID, id string) (*core.Connection, error) {
	return store.GetUserConnection(ctx, s.db, userID, id)
}

func (s connectionStore) ListConnections(ctx context.Context, userID, integrationID string) ([]*core.Connection, error) {
	return store.ListUserConnections(ctx, s.db, userID, integrationID)
}

func (s connectionStore) DefaultConnection(ctx context.Context, userID, integrationID string) (*core.Connection, error) {
	return store.DefaultUserConnection(ctx, s.db, userID, integrationID)
}

func (s connectionStore) FindByExternalAccount(ctx context.Context, userID, integrationID, externalAccountID string) (*core.Connection, error) {
	return store.FindConnectionByExternalAccount(ctx, s.db, userID, integrationID, externalAccountID)
}

func (s connectionStore) RenameConnection(ctx context.Context, userID, id, name string, ev core.ConnectionEvent) error {
	return store.RenameConnection(ctx, s.db, userID, id, name, ev)
}

func (s connectionStore) SetDefaultConnection(ctx context.Context, userID, id string, ev core.ConnectionEvent) error {
	return store.SetDefaultConnection(ctx, s.db, userID, id, ev)
}

func (s connectionStore) DeleteConnection(ctx context.Context, userID, id string, ev core.ConnectionEvent) error {
	return store.SoftDeleteConnection(ctx, s.db, userID, id, ev)
}

func (s connectionStore) RecordTestResult(ctx context.Context, userID, id, accountLabel, senderID string) error {
	return store.RecordConnectionTestResult(ctx, s.db, userID, id, accountLabel, senderID)
}

func (s connectionStore) TouchConnectionUsed(ctx context.Context, userID, id string) error {
	return store.TouchConnectionUsed(ctx, s.db, userID, id)
}

func (s connectionStore) GetSecrets(ctx context.Context, userID, id string) (map[string]core.ConnectionSecret, error) {
	return store.GetConnectionSecrets(ctx, s.db, userID, id)
}

func (s connectionStore) WithSecretsLock(ctx context.Context, userID, id string, fn func(core.SecretsTx) error) error {
	return store.WithConnectionSecretsLock(ctx, s.db, userID, id, fn)
}

func (s connectionStore) CreateOAuthFlow(ctx context.Context, f *core.OAuthFlow) error {
	return store.InsertOAuthFlow(ctx, s.db, f)
}

func (s connectionStore) PeekOAuthFlow(ctx context.Context, stateHash []byte, now time.Time) (*core.OAuthFlow, error) {
	return store.PeekOAuthFlow(ctx, s.db, stateHash, now)
}

func (s connectionStore) ConsumeOAuthFlow(ctx context.Context, stateHash []byte, now time.Time) (*core.OAuthFlow, error) {
	return store.ConsumeOAuthFlow(ctx, s.db, stateHash, now)
}

func (s connectionStore) AppendConnectionEvent(ctx context.Context, ev core.ConnectionEvent) error {
	return store.AppendConnectionEvent(ctx, s.db, ev)
}

func (s connectionStore) ListConnectionEvents(ctx context.Context, userID, id string, limit int, beforeID int64) ([]core.ConnectionEvent, error) {
	return store.ListConnectionEvents(ctx, s.db, userID, id, limit, beforeID)
}
