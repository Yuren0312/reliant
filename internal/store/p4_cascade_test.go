// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"

	"github.com/reliant-labs/reliant/internal/db"
	"github.com/reliant-labs/reliant/internal/db/core"
	"github.com/reliant-labs/reliant/internal/store"
)

// workflow/thread states as stored (core.WorkflowState / ThreadStatus).
const (
	wfActive      = 2
	wfStopped     = 3
	stopPaused    = 3
	stopCancelled = 2
	threadRunning = 2
)

// threadCancelled is the thread status a cancelled stop reason maps to.
var threadCancelled, _ = core.ThreadStatusForStopReason(core.WorkflowStopReason(stopCancelled))

// seedWorkflowTree writes a root with `fan` children per node down to
// `depth` levels, each with the thread it owns, through the GENERATED ORM
// delegates (CreateWorkflow / CreateThread). Returns every non-root id.
func seedWorkflowTree(tb testing.TB, env *spikeEnv, chatID, root string, fan, depth int) []string {
	tb.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	var below []string
	var add func(id string, parent *string, level int)
	add = func(id string, parent *string, level int) {
		wf := &store.Workflow{Id: id, ParentId: parent, ChatId: chatID, WorkflowName: "builtin://agent",
			Thread: id, CreatedAt: now, State: wfActive}
		if level == depth { // leaves are paused, the other live state
			wf.State, wf.StopReason = wfStopped, stopPaused
		}
		if err := store.CreateWorkflow(ctx, env.ORM, wf); err != nil {
			tb.Fatalf("create workflow %s: %v", id, err)
		}
		wid := id
		// threads.status is NOT NULL DEFAULT 2, but the ORM writes the Go
		// zero for a NOT NULL non-timestamp column: the default never fires,
		// so a port must set it explicitly.
		if err := store.CreateThread(ctx, env.ORM, &store.Thread{Id: "th-" + id, ChatId: chatID,
			WorkflowId: &wid, CreatedAt: now, Origin: "spawn", Status: threadRunning}); err != nil {
			tb.Fatalf("create thread %s: %v", id, err)
		}
		if level == depth {
			return
		}
		for i := 0; i < fan; i++ {
			child := fmt.Sprintf("%s.%d", id, i)
			below = append(below, child)
			add(child, &wid, level+1)
		}
	}
	add(root, nil, 0)
	return below
}

func states(tb testing.TB, env *spikeEnv, chatID string) map[string]string {
	tb.Helper()
	rows, err := env.SQL.Query(`SELECT w.id, w.state, w.stop_reason, w.completed_at IS NOT NULL, t.status, t.completed_at IS NOT NULL
		FROM workflows w JOIN threads t ON t.workflow_id = w.id WHERE w.chat_id = $1`, chatID)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id string
		var st, sr, ts int
		var wDone, tDone bool
		if err := rows.Scan(&id, &st, &sr, &wDone, &ts, &tDone); err != nil {
			tb.Fatal(err)
		}
		out[id] = fmt.Sprintf("wf=%d/%d done=%v thread=%d done=%v", st, sr, wDone, ts, tDone)
	}
	return out
}

// TestP4Cascades_MatchSQLC builds the same tree twice and drives one copy
// through the sqlc cascades and the other through the ORM ports: every row's
// end state must match, the root and an unrelated tree untouched, and the
// recursive SELECT must list exactly the subtree.
func TestP4Cascades_MatchSQLC(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()

	for _, c := range []string{"chat-p4-sqlc", "chat-p4-orm", "chat-p4-other"} {
		seedChat(t, env, c)
	}
	below := seedWorkflowTree(t, env, "chat-p4-sqlc", "s", 3, 3)
	seedWorkflowTree(t, env, "chat-p4-orm", "o", 3, 3)
	seedWorkflowTree(t, env, "chat-p4-other", "x", 2, 2)

	ids, err := store.ListWorkflowDescendantIDs(ctx, env.ORM, "o")
	if err != nil {
		t.Fatalf("list descendants: %v", err)
	}
	if len(ids) != len(below) || len(ids) != 3+9+27 {
		t.Fatalf("recursive select found %d descendants, want %d", len(ids), len(below))
	}

	if err := env.Repo.CascadeTerminalStatusToDescendants(ctx, "s", db.WorkflowStopReason(stopCancelled)); err != nil {
		t.Fatalf("sqlc cascade: %v", err)
	}
	if err := env.Repo.CascadeTerminalStatusToThreadSubtree(ctx, "s", db.WorkflowStopReason(stopCancelled)); err != nil {
		t.Fatalf("sqlc thread cascade: %v", err)
	}
	if err := store.CascadeTerminalStatusToDescendants(ctx, env.ORM, "o", stopCancelled); err != nil {
		t.Fatalf("orm cascade: %v", err)
	}
	if err := store.CascadeTerminalStatusToThreadSubtree(ctx, env.ORM, "o", threadCancelled); err != nil {
		t.Fatalf("orm thread cascade: %v", err)
	}

	sq, om := states(t, env, "chat-p4-sqlc"), states(t, env, "chat-p4-orm")
	if len(sq) != 40 || len(om) != 40 {
		t.Fatalf("rows: sqlc %d orm %d, want 40 each", len(sq), len(om))
	}
	var keys []string
	for k := range sq {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		o := "o" + k[1:]
		if sq[k] != om[o] {
			t.Errorf("%s: sqlc %q, orm %s %q", k, sq[k], o, om[o])
		}
	}
	if got := om["o"]; got != fmt.Sprintf("wf=%d/0 done=false thread=%d done=true", wfActive, threadCancelled) {
		t.Errorf("root: %s (the workflow cascade closes descendants only; the thread cascade includes the root)", got)
	}
	for id, st := range states(t, env, "chat-p4-other") {
		if st != fmt.Sprintf("wf=%d/0 done=false thread=%d done=false", wfActive, threadRunning) &&
			st != fmt.Sprintf("wf=%d/%d done=false thread=%d done=false", wfStopped, stopPaused, threadRunning) {
			t.Errorf("unrelated tree row %s changed: %s", id, st)
		}
	}
}

// BenchmarkP4Cascade: both cascades over a 1+3+9 tree; the tree is
// reset to live (untimed) before every op so each op does the full write.
func BenchmarkP4Cascade(b *testing.B) {
	env := newSpikeEnv(b)
	ctx := context.Background()
	seedChat(b, env, "chat-p4b")
	seedWorkflowTree(b, env, "chat-p4b", "r", 3, 2)
	reset := func(b *testing.B) {
		b.StopTimer()
		mustExec(b, env, `UPDATE workflows SET state = 2, stop_reason = 0, completed_at = NULL WHERE chat_id = 'chat-p4b'`)
		mustExec(b, env, `UPDATE threads SET status = 2, completed_at = NULL WHERE chat_id = 'chat-p4b'`)
		b.StartTimer()
	}
	b.Run("sqlc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reset(b)
			if err := env.Repo.CascadeTerminalStatusToDescendants(ctx, "r", db.WorkflowStopReason(stopCancelled)); err != nil {
				b.Fatal(err)
			}
			if err := env.Repo.CascadeTerminalStatusToThreadSubtree(ctx, "r", db.WorkflowStopReason(stopCancelled)); err != nil {
				b.Fatal(err)
			}
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				reset(b)
				if err := store.CascadeTerminalStatusToDescendants(ctx, client, "r", stopCancelled); err != nil {
					b.Fatal(err)
				}
				if err := store.CascadeTerminalStatusToThreadSubtree(ctx, client, "r", threadCancelled); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
