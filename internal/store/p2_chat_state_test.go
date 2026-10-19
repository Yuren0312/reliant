// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/reliant-labs/forge/pkg/svcerr"

	"github.com/reliant-labs/reliant/internal/db"
	"github.com/reliant-labs/reliant/internal/store"
)

// seedChatStates writes one chat per computed activity the view derives from
// workflows and the chat row, plus an archived one. Returns the chat ids
// with the activity each should compute.
func seedChatStates(tb testing.TB, env *spikeEnv, prefix string) map[string]int64 {
	tb.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	wf := func(chatID, id string, state, reason int64, done *time.Time) {
		if err := store.CreateWorkflow(ctx, env.ORM, &store.Workflow{Id: id, ChatId: chatID, WorkflowName: "builtin://agent",
			Thread: chatID, CreatedAt: now, State: state, StopReason: reason, CompletedAt: done}); err != nil {
			tb.Fatalf("workflow %s: %v", id, err)
		}
	}
	earlier, later := now.Add(-time.Hour), now.Add(-time.Minute)
	want := map[string]int64{}
	chat := func(suffix string, activity int64) string {
		id := prefix + "-" + suffix
		seedChat(tb, env, id)
		want[id] = activity
		return id
	}

	wf(chat("running", 1), prefix+"-wf-running", 2, 0, nil)
	wf(chat("paused", 4), prefix+"-wf-paused", 3, 3, nil)
	cancelled := chat("cancelled", 3)
	wf(cancelled, prefix+"-wf-done", 3, 1, &earlier)
	wf(cancelled, prefix+"-wf-cancel", 3, 2, &later)
	queued := chat("queued", 6)
	if err := store.UpdateChatMasked(ctx, env.ORM, &store.Chat{Id: queued, QueuedForMachineAt: &now},
		[]string{store.ChatFieldQueuedForMachineAt}); err != nil {
		tb.Fatalf("queue chat: %v", err)
	}
	archived := chat("archived", 0)
	three := int64(3)
	if err := store.UpdateChatMasked(ctx, env.ORM, &store.Chat{Id: archived, State: &three},
		[]string{store.ChatFieldState}); err != nil {
		tb.Fatalf("archive chat: %v", err)
	}
	return want
}

// TestP2ChatState_MatchesSQLC: the computed columns read through the ORM
// equal what the sqlc path maps into core.Chat, for every activity the view
// can derive from workflows and the chat row; ListChats filters agree.
func TestP2ChatState_MatchesSQLC(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()
	want := seedChatStates(t, env, "p2")

	for id, activity := range want {
		row, err := store.GetChatWithActivity(ctx, env.ORM, id)
		if err != nil {
			t.Fatalf("orm get %s: %v", id, err)
		}
		sq, err := env.Repo.GetChat(ctx, id)
		if err != nil {
			t.Fatalf("sqlc get %s: %v", id, err)
		}
		if row.Activity == nil || *row.Activity != activity {
			t.Errorf("%s: orm activity %v, want %d", id, row.Activity, activity)
		}
		if sq.Activity == nil || int64(*sq.Activity) != *row.Activity {
			t.Errorf("%s: sqlc activity %v != orm %v", id, sq.Activity, row.Activity)
		}
		if sq.ListInSidebar != row.ListInSidebar || sq.Title != row.Title || sq.Unread != (row.Unread != 0) {
			t.Errorf("%s: base/computed columns differ: sqlc %+v orm %+v", id, sq, row)
		}

		st, reason, err := store.GetRootWorkflowStatusForChat(ctx, env.ORM, id)
		sqm, sqerr := env.Repo.GetRootWorkflowStatusForChats(ctx, []string{id})
		sqst, found := sqm[id]
		switch {
		case sqerr != nil:
			t.Errorf("%s: sqlc root status: %v", id, sqerr)
		case svcerr.IsNotFound(err):
			if found {
				t.Errorf("%s: orm found no root workflow, sqlc found %+v", id, sqst)
			}
		case err != nil:
			t.Errorf("%s: orm root status: %v", id, err)
		case fmt.Sprint(st, reason) != fmt.Sprint(int64(sqst.State), int64(sqst.StopReason)):
			t.Errorf("%s: root status orm %d/%d, sqlc %+v", id, st, reason, sqst)
		}
	}

	if _, err := store.GetChatWithActivity(ctx, env.ORM, "nope"); !svcerr.IsNotFound(err) {
		t.Errorf("missing chat: %v, want NotFound", err)
	}

	project := "test-project"
	archived := int64(3)
	for _, f := range []store.ChatListFilter{
		{UserID: "test-user", Limit: 50},
		{UserID: "test-user", ExcludeArchived: true, Limit: 50},
		{UserID: "test-user", ProjectID: &project, State: &archived, Limit: 50},
		{UserID: "test-user", SidebarOnly: true, Limit: 2, Offset: 1},
	} {
		rows, err := store.ListChatsWithActivity(ctx, env.ORM, f)
		if err != nil {
			t.Fatalf("orm list %+v: %v", f, err)
		}
		cf := db.ChatFilters{UserID: f.UserID, ProjectID: f.ProjectID, ExcludeArchived: f.ExcludeArchived,
			SidebarOnly: f.SidebarOnly, Limit: f.Limit, Offset: f.Offset}
		if f.State != nil {
			s := db.ChatState(*f.State)
			cf.State = &s
		}
		sq, err := env.Repo.ListChats(ctx, cf)
		if err != nil {
			t.Fatalf("sqlc list: %v", err)
		}
		var a, b []string
		for _, r := range rows {
			a = append(a, r.Id)
		}
		for _, r := range sq {
			b = append(b, r.ID)
		}
		if fmt.Sprint(a) != fmt.Sprint(b) {
			t.Errorf("list %+v: orm %v, sqlc %v", f, a, b)
		}
	}
}

// BenchmarkP2ChatState: one GetChat and one 40-row sidebar ListChats per op
// over 60 chats in every computed state.
func BenchmarkP2ChatState(b *testing.B) {
	env := newSpikeEnv(b)
	ctx := context.Background()
	var ids []string
	for i := 0; i < 10; i++ {
		for id := range seedChatStates(b, env, fmt.Sprintf("b%d", i)) {
			ids = append(ids, id)
		}
	}
	b.Run("sqlc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := env.Repo.GetChat(ctx, ids[i%len(ids)]); err != nil {
				b.Fatal(err)
			}
			if rows, err := env.Repo.ListChats(ctx, db.ChatFilters{UserID: "test-user", SidebarOnly: true, ExcludeArchived: true, Limit: 50}); err != nil || len(rows) != 40 {
				b.Fatal(err, len(rows))
			}
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := store.GetChatWithActivity(ctx, client, ids[i%len(ids)]); err != nil {
					b.Fatal(err)
				}
				if rows, err := store.ListChatsWithActivity(ctx, client, store.ChatListFilter{UserID: "test-user", SidebarOnly: true, ExcludeArchived: true, Limit: 50}); err != nil || len(rows) != 40 {
					b.Fatal(err, len(rows))
				}
			}
		})
	}
}
