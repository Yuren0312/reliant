// Copyright (c) 2025 Reliant Labs
package storetest_test

import (
	"context"
	"slices"
	"testing"
	"time"

	reliantv1 "github.com/reliant-labs/reliant/gen/reliant/v1"
	"github.com/reliant-labs/reliant/internal/chatselection"
	"github.com/reliant-labs/reliant/internal/db"
	"github.com/reliant-labs/reliant/internal/db/core"
	"github.com/reliant-labs/reliant/internal/store"
	"github.com/reliant-labs/reliant/internal/store/storetest"
)

// TestTransactionOpenersBeginAtTheirLevel pins the isolation level each
// transaction opener puts on the wire, as observed by the server-facing
// driver rather than read off an options struct. The levels are the pre-P5b
// ones (the hand SQL and Repository transactions the ORM port replaced),
// except where a row says otherwise; internal/db/SCHEMA.md rule 7 is why a
// port must keep them. A port that silently moves a lock-based transaction
// onto the ORM's SERIALIZABLE default fails here, by name.
func TestTransactionOpenersBeginAtTheirLevel(t *testing.T) {
	env := storetest.New(t)
	newChat(t, env, "chat-iso")
	now := time.Now().UTC()
	if err := store.InsertWorktree(context.Background(), env.ORM, &core.Worktree{
		ID: "wt-iso", Name: "iso", Path: "/wt/iso", Branch: "feat/iso", BaseBranch: "main",
		ProjectID: storetest.SeedProjectID, Status: int32(reliantv1.WorktreeStatus_WORKTREE_STATUS_ACTIVE),
		CreatedAt: now, UpdatedAt: now, LastActive: now,
	}); err != nil {
		t.Fatalf("seed worktree: %v", err)
	}
	p := env.Probe(t)
	noop := func(context.Context) error { return nil }

	const (
		serializable  = "serializable"
		readCommitted = "read committed"
	)
	cases := []struct {
		name string
		want string
		run  func(ctx context.Context) error
	}{
		// The Repository's own transactions: SERIALIZABLE unless asked.
		{"Repo.RunTx", serializable, func(ctx context.Context) error { return p.Repo.RunTx(ctx, noop) }},
		{"Repo.RunTxReadOnly", serializable, func(ctx context.Context) error { return p.Repo.RunTxReadOnly(ctx, noop) }},
		{"Repo.RunTxNoRetry", serializable, func(ctx context.Context) error { return p.Repo.RunTxNoRetry(ctx, noop) }},
		{"Repo.RunTxWithOptions(read committed)", readCommitted, func(ctx context.Context) error {
			return p.Repo.RunTxWithOptions(ctx, db.TxOptions{Isolation: db.IsolationReadCommitted}, noop)
		}},
		// The ORM's default, which every lock-based port must override.
		{"orm.Client.RunTx (forge default)", serializable, func(ctx context.Context) error { return p.ORM.RunTx(ctx, noop) }},

		// Serializable Repository writers.
		{"Repo.CreateChatUpdate", serializable, func(ctx context.Context) error {
			return p.Repo.CreateChatUpdate(ctx, "chat-iso", reliantv1.ChatUpdateType_CHAT_UPDATE_TYPE_MESSAGE, "e", `{}`)
		}},
		{"Repo.MutateChatSelection", serializable, func(ctx context.Context) error {
			_, err := p.Repo.MutateChatSelection(ctx, "chat-iso", func(cur *chatselection.Selection) (*chatselection.Selection, error) { return cur, nil })
			return err
		}},

		// READ COMMITTED: correctness from row/advisory locks or a single
		// allocating statement, never from snapshot isolation.
		{"Repo.CreateUserUpdate", readCommitted, func(ctx context.Context) error {
			return p.Repo.CreateUserUpdate(ctx, &db.UserUpdate{
				UserID: storetest.SeedUserID, UpdateType: db.UserUpdateChatActivityChanged,
				EntityType: db.EntityTypeChat, EntityID: "chat-iso", Data: []byte(`{}`),
			})
		}},
		{"Repo.RemoveDaemon", readCommitted, func(ctx context.Context) error {
			_, _, err := p.Repo.RemoveDaemon(ctx, "daemon-none")
			return err
		}},
		{"Repo.CreateWorkflowDraft", readCommitted, func(ctx context.Context) error {
			return p.Repo.CreateWorkflowDraft(ctx, &db.WorkflowDraft{
				ID: "wd-iso", UserID: storetest.SeedUserID, Name: "Iso", Slug: "iso",
				Definition: "name: iso\n", Status: core.WorkflowDraftStatusDraft, CreatedAt: now, UpdatedAt: now,
			})
		}},
		{"Repo.MergeWorktreeCleanupMetadata", readCommitted, func(ctx context.Context) error {
			return p.Repo.MergeWorktreeCleanupMetadata(ctx, "wt-iso", func(*core.CleanupMetadata, bool) bool { return false })
		}},
		{"Repo.LockChatRunControl", readCommitted, func(ctx context.Context) error {
			release, err := p.Repo.LockChatRunControl(ctx, "chat-iso")
			if err == nil {
				release()
			}
			return err
		}},
		{"Repo.LockProviderKey", readCommitted, func(ctx context.Context) error {
			release, err := p.Repo.LockProviderKey(ctx, storetest.SeedUserID, "anthropic")
			if err == nil {
				release()
			}
			return err
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p.Reset()
			if err := c.run(context.Background()); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if got := p.Isolations(); !slices.Equal(got, []string{c.want}) {
				t.Fatalf("%s began transactions at %v, want exactly one at %s", c.name, got, c.want)
			}
		})
	}

	// A transaction joined is not begun: an inner opener inherits the outer
	// transaction's level, whatever it asks for (RunTxWithOptions).
	t.Run("a read committed opener inside Repo.RunTx joins it", func(t *testing.T) {
		p.Reset()
		err := p.Repo.RunTx(context.Background(), func(ctx context.Context) error {
			return store.MergeWorktreeCleanupMetadata(ctx, p.ORM, "wt-iso", func(*core.CleanupMetadata, bool) bool { return false })
		})
		if err != nil {
			t.Fatalf("Repo.RunTx: %v", err)
		}
		if got := p.Isolations(); !slices.Equal(got, []string{serializable}) {
			t.Fatalf("began %v, want only the outer [serializable]", got)
		}
	})
}
