// yours: scaffolded once, never touched again — forge will not overwrite this file.
//
// This is the custom-query seam for Chat. The generated chat_orm.go is
// Tier-1 (a live projection of the migrations, regenerated every run);
// hand-written queries that outgrow the generated CRUD belong HERE, in a
// sibling file forge never rewrites. You have three building blocks:
//
//  1. The generated repo + delegates: chatRepo (a *crud.Repo[Chat]) and the
//     package-level GetChatByID / ListChat / CreateChat / UpdateChat / DeleteChat
//     functions — compose orm.QueryOption filters (orm.WhereEq,
//     orm.WhereILikeAny, orm.WithOrderBy, orm.WithLimit) onto List/Count.
//  2. Raw SQL via the Bun handle: db.Bun() (bun.IDB) gives you
//     NewSelect()/NewRaw()/bun.Raw — for joins, aggregates, CTEs, or
//     anything the builder can't express. Scan into the generated
//     Chat struct (or your own row type).
//  3. orm.Context: every query takes one. Take no transaction parameter:
//     called with the ctx that s.deps.DB.RunTx hands its fn, a method
//     here (db.Bun() builders and raw SQL included) joins that
//     transaction; with any other ctx it runs on the pool.
//
// Example (uncomment and adapt):
//
//	func FindChatBy(ctx context.Context, db orm.Context, /* args */) ([]*Chat, error) {
//		var rows []*Chat
//		err := db.Bun().NewSelect().Model(&rows).
//			Where("title = ?", /* value */).
//			Scan(ctx)
//		return rows, err
//	}
//
// Or reuse the generated repo for a filtered list:
//
//	func ActiveChats(ctx context.Context, db orm.Context) ([]*Chat, error) {
//		return ListChat(ctx, db, orm.WhereEq("title", true))
//	}
package store

import (
	"context"
	"errors"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/reliant-labs/forge/pkg/svcerr"
	"github.com/uptrace/bun"
)

// ChatWithActivity is one row of the chats_with_activity VIEW: the chats
// columns plus the computed state the view derives from messages, approvals,
// questions, workflows and usage limits. forge projects tables, not views, so
// this row type is hand-written here. It embeds the generated Chat for the
// base columns; the view does not carry paused_for_machine_at, which
// therefore stays nil (columns are selected with *, not from the struct).
type ChatWithActivity struct {
	bun.BaseModel `bun:"table:chats_with_activity,alias:chats_with_activity"`
	Chat

	LastMessageAt          *time.Time `bun:"last_message_at"`
	Activity               *int64     `bun:"activity"`
	RootWorkflowState      *int64     `bun:"root_workflow_state"`
	RootWorkflowStopReason *int64     `bun:"root_workflow_stop_reason"`
	LaunchKind             *string    `bun:"launch_kind"`
	TriggerId              *string    `bun:"trigger_id"`
	ListInSidebar          bool       `bun:"list_in_sidebar"`
	DisplayState           int64      `bun:"display_state"`
	UsageLimitResumesAt    *time.Time `bun:"usage_limit_resumes_at"`
	UsageLimitProvider     *string    `bun:"usage_limit_provider"`
	UsageLimitProbing      *bool      `bun:"usage_limit_probing"`
}

func chatsWithActivity(db orm.Context, dst any) *bun.SelectQuery {
	return db.Bun().NewSelect().Model(dst).ColumnExpr("chats_with_activity.*")
}

// GetChatWithActivity is the port of sqlc GetChat (SELECT * FROM
// chats_with_activity WHERE id = $1).
func GetChatWithActivity(ctx context.Context, db orm.Context, id string) (*ChatWithActivity, error) {
	row := new(ChatWithActivity)
	err := chatsWithActivity(db, row).Where("chats_with_activity.id = ?", id).Limit(1).Scan(ctx)
	if errors.Is(err, orm.ErrNoRows) {
		return nil, svcerr.NotFound("chat")
	}
	return row, err
}

// ChatListFilter mirrors core.ChatFilters.
type ChatListFilter struct {
	UserID          string
	ProjectID       *string
	State           *int64
	ExcludeArchived bool
	SidebarOnly     bool
	Limit           int
	Offset          int
}

// ListChatsWithActivity is the port of sqlc ListChats: the optional filters
// that sqlc spells as `(narg IS NULL OR col = narg)` become orm.QueryOptions
// applied only when set, so each call sends exactly the predicates it uses.
func ListChatsWithActivity(ctx context.Context, db orm.Context, f ChatListFilter) ([]*ChatWithActivity, error) {
	opts := []orm.QueryOption{orm.WhereEq("chats_with_activity.user_id", f.UserID)}
	if f.ProjectID != nil {
		opts = append(opts, orm.WhereEq("chats_with_activity.project_id", *f.ProjectID))
	}
	if f.State != nil {
		opts = append(opts, orm.WhereEq("chats_with_activity.state", *f.State))
	}
	if f.ExcludeArchived {
		opts = append(opts, orm.WhereNotEq("chats_with_activity.state", 3))
	}
	if f.SidebarOnly {
		opts = append(opts, orm.WhereEq("chats_with_activity.list_in_sidebar", true))
	}
	opts = append(opts, orm.WithOrderBy("chats_with_activity.last_active", orm.Desc), orm.WithLimit(f.Limit), orm.WithOffset(f.Offset))

	var rows []*ChatWithActivity
	q := chatsWithActivity(db, &rows)
	for _, o := range opts {
		o(q)
	}
	return rows, q.Scan(ctx)
}

// rootWorkflowStatusSQL is the computed root-workflow lifecycle (sqlc
// GetRootWorkflowStatusForChat), kept as hand SQL through bun's NewRaw: a
// CASE over correlated EXISTS reads better as SQL than as a builder chain.
const rootWorkflowStatusSQL = `
SELECT
    CASE
        WHEN EXISTS (SELECT 1 FROM workflows w3 WHERE w3.chat_id = w.chat_id AND w3.state = 2) THEN 2
        WHEN EXISTS (SELECT 1 FROM workflows w4 WHERE w4.chat_id = w.chat_id AND w4.state = 3 AND w4.stop_reason = 3) THEN 3
        ELSE w.state
    END AS state,
    CASE
        WHEN EXISTS (SELECT 1 FROM workflows w3 WHERE w3.chat_id = w.chat_id AND w3.state = 2) THEN 0
        WHEN EXISTS (SELECT 1 FROM workflows w4 WHERE w4.chat_id = w.chat_id AND w4.state = 3 AND w4.stop_reason = 3) THEN 3
        ELSE w.stop_reason
    END AS stop_reason
FROM workflows w
WHERE w.parent_id IS NULL AND w.chat_id = ?
ORDER BY w.created_at DESC
LIMIT 1`

// GetRootWorkflowStatusForChat returns the chat's effective workflow
// lifecycle; svcerr.NotFound when the chat has no root workflow.
func GetRootWorkflowStatusForChat(ctx context.Context, db orm.Context, chatID string) (state, stopReason int64, err error) {
	err = db.Bun().NewRaw(rootWorkflowStatusSQL, chatID).Scan(ctx, &state, &stopReason)
	if errors.Is(err, orm.ErrNoRows) {
		return 0, 0, svcerr.NotFound("workflow")
	}
	return state, stopReason, err
}

// ChatExtra carries fields on Chat that are NOT columns — computed
// values, transient state, anything derived rather than stored. Chat is
// embedded on ChatExtra (see chat_orm.go), so a field added here is a field on
// every Chat value.
//
// Every field MUST carry the `bun:"-"` tag. Bun otherwise treats an
// embedded struct's fields as columns of the parent table and tries to
// persist them — `bun:"-"` is Bun's own idiom for a field it must never
// bind or scan.
type ChatExtra struct {
}
