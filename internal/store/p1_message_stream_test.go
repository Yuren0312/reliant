// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"

	reliantv1 "github.com/reliant-labs/reliant/gen/reliant/v1"
	"github.com/reliant-labs/reliant/internal/db"
	"github.com/reliant-labs/reliant/internal/store"
)

// seedStreamChat: a chat, its root thread and the thread's context window.
func seedStreamChat(tb testing.TB, env *spikeEnv, chatID string) {
	tb.Helper()
	seedChat(tb, env, chatID)
	if err := store.CreateContextWindow(context.Background(), env.ORM, &store.ContextWindow{
		Id: "cw-" + chatID, ThreadId: chatID, CreatedAt: time.Now().UTC(),
	}); err != nil {
		tb.Fatalf("context window: %v", err)
	}
}

func strp(s string) *string { return &s }

const textBlock = int64(reliantv1.ContentBlockType_CONTENT_BLOCK_TYPE_TEXT)

// sqlcWrite / ormWrite build the same message for each layer: nBlocks text
// blocks, the last one with an invalid UTF-8 byte and an empty phase.
func sqlcWrite(chatID, id string, nBlocks int) db.AtomicMessageWrite {
	blocks := make([]db.MessageContentBlock, nBlocks)
	for i := range blocks {
		blocks[i] = db.MessageContentBlock{ID: fmt.Sprintf("%s-b%d", id, i), MessageID: id, Position: i,
			BlockType: reliantv1.ContentBlockType_CONTENT_BLOCK_TYPE_TEXT, Content: strp(fmt.Sprintf("block %d of %s", i, id)), Phase: strp("")}
	}
	blocks[nBlocks-1].Content = strp("bad \xff byte")
	return db.AtomicMessageWrite{MessageID: id, ChatID: chatID, ThreadID: chatID, ContextWindowID: "cw-" + chatID,
		Role: reliantv1.MessageRole(1), CreatedAt: time.Now().UTC(), Blocks: blocks,
		ChatUpdateType: db.UpdateTypeMessage, ChatUpdateEntity: id,
		ChatUpdateData: func(o, s, _ int64) (string, error) { return fmt.Sprintf(`{"ordinal":%d,"seq":%d}`, o, s), nil }}
}

func ormWrite(chatID, id string, nBlocks int) store.AtomicMessageWrite {
	blocks := make([]*store.MessageContentBlock, nBlocks)
	for i := range blocks {
		blocks[i] = &store.MessageContentBlock{Id: fmt.Sprintf("%s-b%d", id, i), MessageId: id, Position: int64(i),
			BlockType: textBlock, Content: strp(fmt.Sprintf("block %d of %s", i, id)), Phase: strp("")}
	}
	blocks[nBlocks-1].Content = strp("bad \xff byte")
	return store.AtomicMessageWrite{MessageID: id, ChatID: chatID, ThreadID: chatID, ContextWindowID: "cw-" + chatID,
		Role: 1, CreatedAt: time.Now().UTC(), Blocks: blocks,
		ChatUpdateType: int64(db.UpdateTypeMessage), ChatUpdateEntity: id,
		ChatUpdateData: func(o, s, _ int64) (string, error) { return fmt.Sprintf(`{"ordinal":%d,"seq":%d}`, o, s), nil }}
}

// TestP1SaveMessageAtomic_InterleavesWithSQLC: messages written alternately
// through both layers share one ordinal/seq/update-sequence allocator, their
// blocks read back identically through either layer's batch read, and the
// ORM's onCommit fires only after commit.
func TestP1SaveMessageAtomic_InterleavesWithSQLC(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()
	chat := "chat-p1"
	seedStreamChat(t, env, chat)

	var committed []int64
	var ids []string
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("m%d", i)
		ids = append(ids, id)
		var ord, seq, upd int64
		if i%2 == 0 {
			r, err := env.Repo.SaveMessageAtomic(ctx, sqlcWrite(chat, id, 3))
			if err != nil {
				t.Fatalf("sqlc save %s: %v", id, err)
			}
			ord, seq, upd = r.Ordinal, r.Seq, r.UpdateSeq
		} else {
			r, err := store.SaveMessageAtomic(ctx, env.ORM, ormWrite(chat, id, 3), func(u store.ChatUpdate) {
				committed = append(committed, u.SequenceNumber)
			})
			if err != nil {
				t.Fatalf("orm save %s: %v", id, err)
			}
			ord, seq, upd = r.Ordinal, r.Seq, r.UpdateSeq
		}
		if ord != int64(i) || seq != int64(i) || upd != int64(i+1) {
			t.Fatalf("%s: ordinal/seq/update %d/%d/%d, want %d/%d/%d", id, ord, seq, upd, i, i, i+1)
		}
	}
	if fmt.Sprint(committed) != "[2 4]" {
		t.Fatalf("onCommit saw %v, want [2 4]", committed)
	}

	sq, err := env.Repo.ListContentBlocksForMessages(ctx, ids)
	if err != nil {
		t.Fatalf("sqlc read: %v", err)
	}
	om, err := store.ListContentBlocksForMessages(ctx, env.ORM, ids)
	if err != nil {
		t.Fatalf("orm read: %v", err)
	}
	if len(sq) != 12 || len(om) != 12 {
		t.Fatalf("blocks: sqlc %d orm %d, want 12", len(sq), len(om))
	}
	for i := range sq {
		a, b := sq[i], om[i]
		if a.ID != b.Id || int64(a.Position) != b.Position || *a.Content != *b.Content || (a.Phase == nil) != (b.Phase == nil) {
			t.Errorf("block %d: sqlc %s/%d/%q/%v orm %s/%d/%q/%v", i, a.ID, a.Position, *a.Content, a.Phase, b.Id, b.Position, *b.Content, b.Phase)
		}
	}
	if *om[2].Content != "bad \uFFFD byte" || om[5].Phase != nil {
		t.Errorf("normalization: content %q phase %v", *om[5].Content, om[5].Phase)
	}
}

// TestP1SaveMessageAtomic_RollsBackWithoutCommitHook: a failing payload
// builder rolls the whole write back and never fires onCommit.
func TestP1SaveMessageAtomic_RollsBackWithoutCommitHook(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()
	seedStreamChat(t, env, "chat-p1-rb")
	w := ormWrite("chat-p1-rb", "rb", 2)
	boom := errors.New("boom")
	w.ChatUpdateData = func(int64, int64, int64) (string, error) { return "", boom }
	fired := false
	if _, err := store.SaveMessageAtomic(ctx, env.ORM, w, func(store.ChatUpdate) { fired = true }); !errors.Is(err, boom) {
		t.Fatalf("got %v, want boom", err)
	}
	n, err := store.CountMessage(ctx, env.ORM, orm.WhereEq(store.MessageFieldChatId, "chat-p1-rb"))
	if err != nil || n != 0 || fired {
		t.Fatalf("after rollback: %d messages (err %v), hook fired %v", n, err, fired)
	}
}

// TestP1SaveMessageAtomic_ConcurrentWritersAcrossLayers: 8 writers, half on
// each layer, 10 messages each into one chat: no unique violation, and seq
// is a gapless 0..79.
func TestP1SaveMessageAtomic_ConcurrentWritersAcrossLayers(t *testing.T) {
	env := newSpikeEnv(t)
	chat := "chat-p1-cc"
	seedStreamChat(t, env, chat)
	var mu sync.Mutex
	var seqs []int
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				id := fmt.Sprintf("cc-%d-%d", w, i)
				var seq int64
				if w%2 == 0 {
					r, err := env.Repo.SaveMessageAtomic(context.Background(), sqlcWrite(chat, id, 2))
					if err != nil {
						t.Errorf("sqlc %s: %v", id, err)
						return
					}
					seq = r.Seq
				} else {
					r, err := store.SaveMessageAtomic(context.Background(), env.ORM, ormWrite(chat, id, 2), nil)
					if err != nil {
						t.Errorf("orm %s: %v", id, err)
						return
					}
					seq = r.Seq
				}
				mu.Lock()
				seqs = append(seqs, int(seq))
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	sort.Ints(seqs)
	for i, s := range seqs {
		if s != i {
			t.Fatalf("seq %d at position %d (want gapless 0..79): %v", s, i, seqs)
		}
	}
}

// TestP1AppendToContentBlock_InterleavesWithSQLC: deltas appended through
// both layers concatenate in order and bump version once each.
func TestP1AppendToContentBlock_InterleavesWithSQLC(t *testing.T) {
	env := newSpikeEnv(t)
	ctx := context.Background()
	seedStreamChat(t, env, "chat-p1-ap")
	if _, err := store.SaveMessageAtomic(ctx, env.ORM, ormWrite("chat-p1-ap", "ap", 1), nil); err != nil {
		t.Fatal(err)
	}
	block := "ap-b0"
	mustExec(t, env, `UPDATE message_content_blocks SET content = '' WHERE id = $1`, block)
	for i := 0; i < 6; i++ {
		delta := fmt.Sprintf("<%d '\\ ?>", i) // quotes, a backslash and a ? survive bun's interpolation
		var err error
		if i%2 == 0 {
			err = env.Repo.AppendToContentBlock(ctx, block, delta)
		} else {
			err = store.AppendToContentBlock(ctx, env.ORM, block, delta)
		}
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	got, err := store.GetMessageContentBlockByID(ctx, env.ORM, block)
	if err != nil {
		t.Fatal(err)
	}
	want := "<0 '\\ ?><1 '\\ ?><2 '\\ ?><3 '\\ ?><4 '\\ ?><5 '\\ ?>"
	if *got.Content != want || got.Version == nil || *got.Version != 6 {
		t.Fatalf("content %q version %v, want %q version 6", *got.Content, got.Version, want)
	}
}

// BenchmarkP1SaveMessageAtomic: one 3-block message per op into one chat.
func BenchmarkP1SaveMessageAtomic(b *testing.B) {
	env := newSpikeEnv(b)
	ctx := context.Background()
	var n atomic.Int64
	// Seeded outside b.Run: a sub-benchmark body runs more than once.
	for _, c := range []string{"chat-b-sqlc", "chat-b-orm", "chat-b-orm-execmode"} {
		seedStreamChat(b, env, c)
	}
	b.Run("sqlc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := env.Repo.SaveMessageAtomic(ctx, sqlcWrite("chat-b-sqlc", fmt.Sprintf("s%d", n.Add(1)), 3)); err != nil {
				b.Fatal(err)
			}
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			chat := "chat-b-" + name
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := store.SaveMessageAtomic(ctx, client, ormWrite(chat, fmt.Sprintf("o%d", n.Add(1)), 3), nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkP1AppendDelta: one ~60-byte streamed delta per op, varying text.
func BenchmarkP1AppendDelta(b *testing.B) {
	env := newSpikeEnv(b)
	ctx := context.Background()
	seedStreamChat(b, env, "chat-b-ap")
	if _, err := store.SaveMessageAtomic(ctx, env.ORM, ormWrite("chat-b-ap", "bap", 1), nil); err != nil {
		b.Fatal(err)
	}
	delta := func(i int) string {
		return fmt.Sprintf("token %08d of a streamed assistant reply, quoted 'like this'", i)
	}
	// Every append rewrites the whole (growing) text value, so each sub-run
	// starts from an empty block: otherwise later sub-runs pay for earlier
	// ones' content.
	reset := func(b *testing.B) {
		b.StopTimer()
		mustExec(b, env, `UPDATE message_content_blocks SET content = '' WHERE id = 'bap-b0'`)
		b.StartTimer()
	}
	b.Run("sqlc", func(b *testing.B) {
		reset(b)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := env.Repo.AppendToContentBlock(ctx, "bap-b0", delta(i)); err != nil {
				b.Fatal(err)
			}
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			reset(b)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := store.AppendToContentBlock(ctx, client, "bap-b0", delta(i)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkP1ListBlocks: the batch block read for 20 messages x 3 blocks.
func BenchmarkP1ListBlocks(b *testing.B) {
	env := newSpikeEnv(b)
	ctx := context.Background()
	seedStreamChat(b, env, "chat-b-ls")
	var ids []string
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("ls%d", i)
		ids = append(ids, id)
		if _, err := store.SaveMessageAtomic(ctx, env.ORM, ormWrite("chat-b-ls", id, 3), nil); err != nil {
			b.Fatal(err)
		}
	}
	b.Run("sqlc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if rows, err := env.Repo.ListContentBlocksForMessages(ctx, ids); err != nil || len(rows) != 60 {
				b.Fatal(err, len(rows))
			}
		}
	})
	for name, client := range map[string]*orm.Client{"orm": env.ORM, "orm-execmode": env.ORMExec} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if rows, err := store.ListContentBlocksForMessages(ctx, client, ids); err != nil || len(rows) != 60 {
					b.Fatal(err, len(rows))
				}
			}
		})
	}
}
