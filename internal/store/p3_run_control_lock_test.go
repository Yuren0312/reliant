// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"

	"github.com/reliant-labs/reliant/internal/store"
)

// TestP3RunControlLock_ExcludesAcrossLayers: a lock held through the ORM port
// blocks a taker on the sqlc/hand path and vice versa — the two share one
// lock namespace, so a lane-by-lane cutover never has a window where both
// critical sections run at once.
func TestP3RunControlLock_ExcludesAcrossLayers(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()

	release, err := store.LockChatRunControl(ctx, env.ORM, "chat-p3")
	if err != nil {
		t.Fatalf("orm lock: %v", err)
	}

	got := make(chan time.Time, 1)
	go func() {
		rel, err := env.Repo.LockChatRunControl(ctx, "chat-p3")
		if err != nil {
			t.Errorf("repo lock: %v", err)
			got <- time.Time{}
			return
		}
		got <- time.Now()
		rel()
	}()

	held := 150 * time.Millisecond
	time.Sleep(held)
	releasedAt := time.Now()
	release()
	release() // idempotent

	acquired := <-got
	if acquired.Before(releasedAt) {
		t.Fatalf("repo path acquired at %v, before the ORM holder released at %v", acquired, releasedAt)
	}

	// Another chat's lock is independent.
	other, err := store.LockChatRunControl(ctx, env.ORM, "chat-p3-other")
	if err != nil {
		t.Fatalf("independent chat: %v", err)
	}
	other()
}

// TestP3RunControlLock_ReleasedWhenCtxEnds: database/sql rolls the owning
// transaction back when ctx is done, so an abandoned request frees the lock.
func TestP3RunControlLock_ReleasedWhenCtxEnds(t *testing.T) {
	env := newSpikeEnv(t)
	reqCtx, cancel := context.WithCancel(context.Background())
	if _, err := store.LockChatRunControl(reqCtx, env.ORM, "chat-p3-ctx"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	cancel()

	waitCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	rel, err := store.LockChatRunControl(waitCtx, env.ORM, "chat-p3-ctx")
	if err != nil {
		t.Fatalf("lock after the holder's ctx ended: %v", err)
	}
	rel()
}

// TestP3RunControlLock_RefusedInsideTx: taken inside an orm.RunTx transaction
// the lock would be released at that transaction's commit.
func TestP3RunControlLock_RefusedInsideTx(t *testing.T) {
	env := newSpikeEnv(t)
	err := env.ORM.RunTx(context.Background(), func(txCtx context.Context) error {
		_, err := store.LockChatRunControl(txCtx, env.ORM, "chat-p3-tx")
		return err
	})
	if !errors.Is(err, store.ErrRunControlInTx) {
		t.Fatalf("got %v, want ErrRunControlInTx", err)
	}
}

// BenchmarkP3RunControlLock compares one uncontended acquire+release cycle.
func BenchmarkP3RunControlLock(b *testing.B) {
	env := newSpikeEnv(b)
	ctx := context.Background()
	b.Run("sqlc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rel, err := env.Repo.LockChatRunControl(ctx, "chat-bench")
			if err != nil {
				b.Fatal(err)
			}
			rel()
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rel, err := store.LockChatRunControl(ctx, client, "chat-bench")
				if err != nil {
					b.Fatal(err)
				}
				rel()
			}
		})
	}
}
