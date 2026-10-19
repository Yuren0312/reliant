// yours: scaffolded once, never touched again — forge will not overwrite this file.
//
// This is the custom-query seam for Message. The generated message_orm.go is
// Tier-1 (a live projection of the migrations, regenerated every run);
// hand-written queries that outgrow the generated CRUD belong HERE, in a
// sibling file forge never rewrites. You have three building blocks:
//
//  1. The generated repo + delegates: messageRepo (a *crud.Repo[Message]) and the
//     package-level GetMessageByID / ListMessage / CreateMessage / UpdateMessage / DeleteMessage
//     functions — compose orm.QueryOption filters (orm.WhereEq,
//     orm.WhereILikeAny, orm.WithOrderBy, orm.WithLimit) onto List/Count.
//  2. Raw SQL via the Bun handle: db.Bun() (bun.IDB) gives you
//     NewSelect()/NewRaw()/bun.Raw — for joins, aggregates, CTEs, or
//     anything the builder can't express. Scan into the generated
//     Message struct (or your own row type).
//  3. orm.Context: every query takes one. Take no transaction parameter:
//     called with the ctx that s.deps.DB.RunTx hands its fn, a method
//     here (db.Bun() builders and raw SQL included) joins that
//     transaction; with any other ctx it runs on the pool.
//
// Example (uncomment and adapt):
//
//	func FindMessageBy(ctx context.Context, db orm.Context, /* args */) ([]*Message, error) {
//		var rows []*Message
//		err := db.Bun().NewSelect().Model(&rows).
//			Where("chat_id = ?", /* value */).
//			Scan(ctx)
//		return rows, err
//	}
//
// Or reuse the generated repo for a filtered list:
//
//	func ActiveMessages(ctx context.Context, db orm.Context) ([]*Message, error) {
//		return ListMessage(ctx, db, orm.WhereEq("chat_id", true))
//	}
package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// AtomicMessageWrite is the forge-ORM port's input to SaveMessageAtomic: the
// message row, its content blocks and the chat_update that announces it.
// Ordinal and Seq are absent: the statement allocates them.
type AtomicMessageWrite struct {
	MessageID, ChatID, ThreadID, ContextWindowID string

	Role         int64
	DisplayStyle *int64
	Model, Agent *string
	TokenCount   *int64
	Cost         *float64
	WorkflowID   *string
	RunID        *string
	NodeID       *string
	NodePath     *string
	ActivityID   *string
	CreatedAt    time.Time

	Blocks []*MessageContentBlock

	ChatUpdateData   func(ordinal, seq, updateSeq int64) (string, error)
	ChatUpdateType   int64
	ChatUpdateEntity string
}

// AtomicMessageResult reports the numbers the database assigned.
type AtomicMessageResult struct {
	Ordinal, Seq, UpdateSeq int64
	ChatUpdate              *ChatUpdate
}

// saveMessageAtomicSQL is internal/db's single-statement message write
// (counters + message + unnest()ed blocks), byte-for-byte, with sqlc's $N
// placeholders rewritten to bun's positional ?N. It stays hand SQL: the
// point of the statement is ONE round trip holding row locks for one
// statement, which no builder chain expresses more clearly.
const saveMessageAtomicSQL = `
WITH
-- Both allocators are SELF-HEALING: they take the greater of the counter's
-- next value and the table's actual high-water mark.
--
-- This is not belt-and-braces, it is required. Three production paths still
-- insert messages directly with their own allocation — the branch repair in
-- chat_branch.go, the orphan repair in cleanup.go, and the denial message in
-- approval.go. A row written by any of them advances the table past the
-- counter, and a counter that then handed out its own stale next value would
-- collide with messages_chat_seq_key (observed: SQLSTATE 23505 under exactly
-- this mix). Healing here means those writers stay correct without every one
-- of them having to know the counter exists.
--
-- The subquery is an indexed MAX on a single chat/thread, not the recursive
-- fork walk this replaced, and it is evaluated once per write rather than
-- scanning a range the insert then lands in — so it does not reintroduce the
-- SSI predicate conflict that motivated the change.
ord AS (
    INSERT INTO message_order_counters (counter_kind, scope_id, last_assigned)
    VALUES ('ordinal', ?0, COALESCE((SELECT MAX(ordinal) FROM messages WHERE thread_id = ?0), -1) + 1)
    ON CONFLICT (counter_kind, scope_id) DO UPDATE
    SET last_assigned = GREATEST(
            message_order_counters.last_assigned + 1,
            COALESCE((SELECT MAX(ordinal) FROM messages WHERE thread_id = ?0), -1) + 1
        )
    RETURNING last_assigned
),
sq AS (
    INSERT INTO message_order_counters (counter_kind, scope_id, last_assigned)
    VALUES ('seq', ?1, COALESCE((SELECT MAX(seq) FROM messages WHERE chat_id = ?1), -1) + 1)
    ON CONFLICT (counter_kind, scope_id) DO UPDATE
    SET last_assigned = GREATEST(
            message_order_counters.last_assigned + 1,
            COALESCE((SELECT MAX(seq) FROM messages WHERE chat_id = ?1), -1) + 1
        )
    RETURNING last_assigned
),
upd AS (
    INSERT INTO update_stream_counters (stream_kind, stream_id, last_assigned_seq)
    VALUES ('chat', ?1, 1)
    ON CONFLICT (stream_kind, stream_id) DO UPDATE
    SET last_assigned_seq = update_stream_counters.last_assigned_seq + 1
    RETURNING last_assigned_seq
),
msg AS (
    INSERT INTO messages (
        id, chat_id, ordinal, seq, thread_id, context_window_id,
        node_id, node_path, role, display_style, model, agent,
        token_count, cost, workflow_id, run_id, activity_id,
        created_at, updated_at
    )
    SELECT
        ?2, ?1, ord.last_assigned, sq.last_assigned, ?0, ?3,
        ?4, ?5, ?6, ?7, ?8, ?9,
        ?10, ?11, ?12, ?13, ?14,
        ?15, ?15
    FROM ord, sq
),
blocks AS (
    INSERT INTO message_content_blocks (
        id, message_id, position, block_type, content,
        tool_name, tool_input, tool_call_id, thought_signature, is_error,
        version, activity_id, workflow_run_id, attempt_number,
        phase,
        created_at, updated_at
    )
    SELECT
        b.id, ?2, b.position, b.block_type, b.content,
        b.tool_name, b.tool_input, b.tool_call_id, b.thought_signature, b.is_error,
        b.version, b.activity_id, b.workflow_run_id, b.attempt_number,
        -- NULLIF: the column-major arrays carry "" for an absent value (see
        -- derefString), but phase is an enum where "absent" and "empty" are
        -- different answers — a stored "" would be resent to the provider as
        -- an invalid phase. Only this column needs it; the rest are free text.
        NULLIF(b.phase, ''),
        ?15, ?15
    FROM unnest(
        ?16::text[], ?17::bigint[], ?18::int[], ?19::text[],
        ?20::text[], ?21::text[], ?22::text[], ?23::text[], ?24::boolean[],
        ?25::bigint[], ?26::text[], ?27::text[], ?28::bigint[], ?29::text[]
    ) AS b(
        id, position, block_type, content,
        tool_name, tool_input, tool_call_id, thought_signature, is_error,
        version, activity_id, workflow_run_id, attempt_number, phase
    )
)
SELECT ord.last_assigned, sq.last_assigned, upd.last_assigned_seq
FROM ord, sq, upd
`

// SaveMessageAtomic is the forge-ORM port of db.Repo.SaveMessageAtomic: one
// READ COMMITTED transaction, the durable rows in one round trip, the
// chat_update written through the GENERATED CreateChatUpdate, and onCommit
// (may be nil) fired through orm.AfterCommit once the transaction commits.
func SaveMessageAtomic(ctx context.Context, db orm.Context, w AtomicMessageWrite, onCommit func(ChatUpdate)) (*AtomicMessageResult, error) {
	if w.MessageID == "" || w.ChatID == "" || w.ThreadID == "" {
		return nil, errors.New("message, chat and thread IDs are required")
	}
	cols := newBlockColumns(w.Blocks)

	var result AtomicMessageResult
	err := db.RunTxWithOptions(ctx, orm.TxOptions{Isolation: orm.IsolationReadCommitted}, func(txCtx context.Context) error {
		err := db.Bun().NewRaw(saveMessageAtomicSQL,
			w.ThreadID, w.ChatID, w.MessageID, w.ContextWindowID,
			emptyNil(w.NodeID), emptyNil(w.NodePath), w.Role, w.DisplayStyle,
			emptyNil(w.Model), emptyNil(w.Agent), w.TokenCount, w.Cost,
			emptyNil(w.WorkflowID), emptyNil(w.RunID), emptyNil(w.ActivityID), w.CreatedAt,
			pgdialect.Array(cols.ids), pgdialect.Array(cols.positions), pgdialect.Array(cols.blockTypes), pgdialect.Array(cols.contents),
			pgdialect.Array(cols.toolNames), pgdialect.Array(cols.toolInputs), pgdialect.Array(cols.toolCallIDs),
			pgdialect.Array(cols.thoughtSignatures), pgdialect.Array(cols.isErrors),
			pgdialect.Array(cols.versions), pgdialect.Array(cols.activityIDs), pgdialect.Array(cols.workflowRunIDs),
			pgdialect.Array(cols.attemptNumbers), pgdialect.Array(cols.phases),
		).Scan(txCtx, &result.Ordinal, &result.Seq, &result.UpdateSeq)
		if err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}

		data, err := w.ChatUpdateData(result.Ordinal, result.Seq, result.UpdateSeq)
		if err != nil {
			return fmt.Errorf("failed to build chat_update data: %w", err)
		}
		update := &ChatUpdate{
			Id:             fmt.Sprintf("%s-%d", w.ChatID, result.UpdateSeq),
			ChatId:         w.ChatID,
			SequenceNumber: result.UpdateSeq,
			UpdateType:     w.ChatUpdateType,
			EntityId:       w.ChatUpdateEntity,
			Data:           data,
			CreatedAt:      time.Now().UTC(),
		}
		if err := CreateChatUpdate(txCtx, db, update); err != nil {
			return fmt.Errorf("failed to create chat_update: %w", err)
		}
		result.ChatUpdate = update
		if onCommit != nil {
			committed := *update
			return orm.AfterCommit(txCtx, func(context.Context) { onCommit(committed) })
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// blockColumns is the column-major form unnest() consumes.
type blockColumns struct {
	ids, contents, toolNames, toolInputs, toolCallIDs, thoughtSignatures []string
	activityIDs, workflowRunIDs, phases                                  []string
	positions, versions, attemptNumbers                                  []int64
	blockTypes                                                           []int32
	isErrors                                                             []bool
}

func newBlockColumns(blocks []*MessageContentBlock) blockColumns {
	n := len(blocks)
	c := blockColumns{
		ids: make([]string, n), contents: make([]string, n), toolNames: make([]string, n),
		toolInputs: make([]string, n), toolCallIDs: make([]string, n), thoughtSignatures: make([]string, n),
		activityIDs: make([]string, n), workflowRunIDs: make([]string, n), phases: make([]string, n),
		positions: make([]int64, n), versions: make([]int64, n), attemptNumbers: make([]int64, n),
		blockTypes: make([]int32, n), isErrors: make([]bool, n),
	}
	for i, b := range blocks {
		c.ids[i] = b.Id
		c.positions[i] = b.Position
		c.blockTypes[i] = int32(b.BlockType)
		c.contents[i] = derefUTF8(b.Content)
		c.toolNames[i] = derefUTF8(b.ToolName)
		c.toolInputs[i] = derefUTF8(b.ToolInput)
		c.toolCallIDs[i] = derefUTF8(b.ToolCallId)
		c.thoughtSignatures[i] = derefUTF8(b.ThoughtSignature)
		c.isErrors[i] = b.IsError != nil && *b.IsError
		if b.Version != nil {
			c.versions[i] = *b.Version
		}
		c.activityIDs[i] = derefUTF8(b.ActivityId)
		c.workflowRunIDs[i] = derefUTF8(b.WorkflowRunId)
		if b.AttemptNumber != nil {
			c.attemptNumbers[i] = *b.AttemptNumber
		}
		c.phases[i] = derefUTF8(b.Phase)
	}
	return c
}

func derefUTF8(s *string) string {
	if s == nil {
		return ""
	}
	return validUTF8(*s)
}

// validUTF8: postgres text rejects malformed UTF-8, so one bad provider byte
// must not fail a whole write.
func validUTF8(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	return strings.ToValidUTF8(text, "\uFFFD")
}

// emptyNil maps nil and "" to SQL NULL, as internal/db's nullString does.
func emptyNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// MessageExtra carries fields on Message that are NOT columns — computed
// values, transient state, anything derived rather than stored. Message is
// embedded on MessageExtra (see message_orm.go), so a field added here is a field on
// every Message value.
//
// Every field MUST carry the `bun:"-"` tag. Bun otherwise treats an
// embedded struct's fields as columns of the parent table and tries to
// persist them — `bun:"-"` is Bun's own idiom for a field it must never
// bind or scan.
type MessageExtra struct {
}
