// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/reliant-labs/reliant/internal/store"
)

// TestP0CreateIgnoresNotNullDefaults pins an ORM projection gap the port must
// design around (F12): for a NOT NULL non-timestamp column the generator
// emits no bun `default:` tag, so Create writes the Go zero value and the
// column's DEFAULT never fires. threads.status is `NOT NULL DEFAULT 2`; an
// ORM-created thread that does not set Status is stored as 0. A NOT NULL
// jsonb DEFAULT '{}' fares worse: the projected Go type is string, the zero
// is "", and the insert fails.
func TestP0CreateIgnoresNotNullDefaults(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()
	seedChat(t, env, "chat-p0")

	if err := store.CreateThread(ctx, env.ORM, &store.Thread{Id: "th-p0", ChatId: "chat-p0",
		CreatedAt: time.Now().UTC(), Origin: "spawn"}); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	var status int
	if err := env.SQL.QueryRow(`SELECT status FROM threads WHERE id = 'th-p0'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != 0 {
		t.Fatalf("status %d: the ORM now honors the DB default — update 08-p5b-spike.md and drop this pin", status)
	}

	err := store.CreateRunEvent(ctx, env.ORM, &store.RunEvent{Id: "ev-p0", UserId: "test-user", ChatId: "chat-p0",
		Outcome: "finished", DedupeKey: "ev-p0", OccurredAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("a run event with no payload inserted: the ORM now honors DEFAULT '{}'::jsonb — update the doc")
	}
	t.Logf("jsonb NOT NULL DEFAULT '{}' with no Go value: %v", err)
}
