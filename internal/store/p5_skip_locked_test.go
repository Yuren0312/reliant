// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"

	"github.com/reliant-labs/reliant/internal/db/core"
	"github.com/reliant-labs/reliant/internal/store"
)

var p5Base = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// seedRunEvents writes n outbox rows through the sqlc path, created one
// second apart so ORDER BY created_at is total.
func seedRunEvents(tb testing.TB, env *spikeEnv, prefix string, n int) []string {
	tb.Helper()
	chatID := "chat-p5-" + prefix
	seedChat(tb, env, chatID)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = fmt.Sprintf("%s-%04d", prefix, i)
		ok, err := env.Repo.CreateRunEvent(context.Background(), &core.RunEvent{
			ID: ids[i], UserID: "test-user", ChatID: chatID, WorkflowName: "wf",
			Outcome: core.RunEventOutcome("finished"), DedupeKey: ids[i], Payload: map[string]any{"i": i},
			OccurredAt: p5Base, CreatedAt: p5Base.Add(time.Duration(i) * time.Second),
		})
		if err != nil || !ok {
			tb.Fatalf("seed run event %d: ok=%v err=%v", i, ok, err)
		}
	}
	return ids
}

func ormIDs(rows []*store.RunEvent) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Id
	}
	sort.Strings(out)
	return out
}

func coreIDs(rows []*core.RunEvent) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	sort.Strings(out)
	return out
}

// TestP5ClaimRunEvents_MatchesSQLC: on the same queue state — one dispatched,
// one leased into the future, one with an expired lease — both paths claim the
// same rows, oldest first, capped at max, and leave the same lease behind.
func TestP5ClaimRunEvents_MatchesSQLC(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()
	ids := seedRunEvents(t, env, "ev", 6)

	now := p5Base.Add(time.Hour)
	mustExec(t, env, `UPDATE run_events SET dispatched_at = $1 WHERE id = $2`, now, ids[0])
	mustExec(t, env, `UPDATE run_events SET claimed_until = $1 WHERE id = $2`, now.Add(time.Minute), ids[1])
	mustExec(t, env, `UPDATE run_events SET claimed_until = $1 WHERE id = $2`, now.Add(-time.Minute), ids[2])

	lease := now.Add(30 * time.Second)
	got, err := store.ClaimRunEvents(ctx, env.ORM, now, lease, 3)
	if err != nil {
		t.Fatalf("orm claim: %v", err)
	}
	want := []string{ids[2], ids[3], ids[4]}
	if g := ormIDs(got); fmt.Sprint(g) != fmt.Sprint(want) {
		t.Fatalf("orm claimed %v, want %v", g, want)
	}
	for _, r := range got {
		if r.ClaimedUntil == nil || !r.ClaimedUntil.Equal(lease) {
			t.Fatalf("%s: RETURNING scanned claimed_until %v, want %v", r.Id, r.ClaimedUntil, lease)
		}
	}

	// Reset the three leases and let the sqlc path claim the same state.
	mustExec(t, env, `UPDATE run_events SET claimed_until = $1 WHERE id = ANY($2)`, now.Add(-time.Minute), want)
	sq, err := env.Repo.ClaimRunEvents(ctx, now, lease, 3)
	if err != nil {
		t.Fatalf("sqlc claim: %v", err)
	}
	if g := coreIDs(sq); fmt.Sprint(g) != fmt.Sprint(want) {
		t.Fatalf("sqlc claimed %v, want %v", g, want)
	}

	if err := store.MarkRunEventDispatched(ctx, env.ORM, ids[2], now); err != nil {
		t.Fatalf("mark dispatched: %v", err)
	}
	ev, err := env.Repo.GetRunEvent(ctx, ids[2])
	if err != nil || ev.DispatchedAt == nil || !ev.DispatchedAt.Equal(now) {
		t.Fatalf("sqlc read of an ORM-dispatched event: %+v, %v", ev, err)
	}
}

// TestP5ClaimRunEvents_ConcurrentClaimersAreDisjoint: relays on both layers
// drain one queue at once; SKIP LOCKED must hand every row to exactly one.
func TestP5ClaimRunEvents_ConcurrentClaimersAreDisjoint(t *testing.T) {
	env := newSpikeEnv(t)
	ids := seedRunEvents(t, env, "cc", 120)
	now, lease := p5Base.Add(time.Hour), p5Base.Add(2*time.Hour)

	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for {
				var claimed []string
				if w%2 == 0 {
					rows, err := store.ClaimRunEvents(context.Background(), env.ORM, now, lease, 7)
					if err != nil {
						t.Errorf("orm claimer %d: %v", w, err)
						return
					}
					claimed = ormIDs(rows)
				} else {
					rows, err := env.Repo.ClaimRunEvents(context.Background(), now, lease, 7)
					if err != nil {
						t.Errorf("sqlc claimer %d: %v", w, err)
						return
					}
					claimed = coreIDs(rows)
				}
				if len(claimed) == 0 {
					return
				}
				mu.Lock()
				for _, id := range claimed {
					seen[id]++
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	for _, id := range ids {
		if seen[id] != 1 {
			t.Fatalf("%s claimed %d times, want exactly once", id, seen[id])
		}
	}
}

func mustExec(tb testing.TB, env *spikeEnv, q string, args ...any) {
	tb.Helper()
	if _, err := env.SQL.Exec(q, toPGArgs(args)...); err != nil {
		tb.Fatalf("%s: %v", q, err)
	}
}

// toPGArgs lets a []string reach pgx as a text[].
func toPGArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.([]string); ok {
			out[i] = orm.StringArray(s)
			continue
		}
		out[i] = a
	}
	return out
}

// BenchmarkP5ClaimRunEvents claims 10 rows from a 1000-row outbox per op.
// Each op's lease expires before the next op's now, so the steady state
// re-claims the 10 oldest rows: same plan, same row count every iteration.
func BenchmarkP5ClaimRunEvents(b *testing.B) {
	env := newSpikeEnv(b)
	seedRunEvents(b, env, "bench", 1000)
	ctx := context.Background()
	run := func(b *testing.B, claim func(now, lease time.Time) (int, error)) {
		b.ReportAllocs()
		now := p5Base.Add(time.Hour)
		for i := 0; i < b.N; i++ {
			now = now.Add(time.Second)
			n, err := claim(now, now.Add(time.Millisecond))
			if err != nil || n != 10 {
				b.Fatalf("claimed %d, err %v", n, err)
			}
		}
	}
	b.Run("sqlc", func(b *testing.B) {
		run(b, func(now, lease time.Time) (int, error) {
			rows, err := env.Repo.ClaimRunEvents(ctx, now, lease, 10)
			return len(rows), err
		})
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			run(b, func(now, lease time.Time) (int, error) {
				rows, err := store.ClaimRunEvents(ctx, client, now, lease, 10)
				return len(rows), err
			})
		})
	}
}

// BenchmarkP5MarkDispatched: the sqlc :exec UPDATE vs the generated masked
// update (which RETURNs the row it did not write).
func BenchmarkP5MarkDispatched(b *testing.B) {
	env := newSpikeEnv(b)
	ids := seedRunEvents(b, env, "mk", 1)
	ctx := context.Background()
	// The stamp varies per op, as it does in production.
	b.Run("sqlc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := env.Repo.MarkRunEventDispatched(ctx, ids[0], p5Base.Add(time.Duration(i))); err != nil {
				b.Fatal(err)
			}
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := store.MarkRunEventDispatched(ctx, client, ids[0], p5Base.Add(time.Duration(i))); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
