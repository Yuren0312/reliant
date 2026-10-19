// yours: scaffolded once, never touched again — forge will not overwrite this file.
//
// This is the custom-query seam for MessageContentBlock. The generated message_content_block_orm.go is
// Tier-1 (a live projection of the migrations, regenerated every run);
// hand-written queries that outgrow the generated CRUD belong HERE, in a
// sibling file forge never rewrites. You have three building blocks:
//
//  1. The generated repo + delegates: messageContentBlockRepo (a *crud.Repo[MessageContentBlock]) and the
//     package-level GetMessageContentBlockByID / ListMessageContentBlock / CreateMessageContentBlock / UpdateMessageContentBlock / DeleteMessageContentBlock
//     functions — compose orm.QueryOption filters (orm.WhereEq,
//     orm.WhereILikeAny, orm.WithOrderBy, orm.WithLimit) onto List/Count.
//  2. Raw SQL via the Bun handle: db.Bun() (bun.IDB) gives you
//     NewSelect()/NewRaw()/bun.Raw — for joins, aggregates, CTEs, or
//     anything the builder can't express. Scan into the generated
//     MessageContentBlock struct (or your own row type).
//  3. orm.Context: every query takes one. Take no transaction parameter:
//     called with the ctx that s.deps.DB.RunTx hands its fn, a method
//     here (db.Bun() builders and raw SQL included) joins that
//     transaction; with any other ctx it runs on the pool.
//
// Example (uncomment and adapt):
//
//	func FindMessageContentBlockBy(ctx context.Context, db orm.Context, /* args */) ([]*MessageContentBlock, error) {
//		var rows []*MessageContentBlock
//		err := db.Bun().NewSelect().Model(&rows).
//			Where("message_id = ?", /* value */).
//			Scan(ctx)
//		return rows, err
//	}
//
// Or reuse the generated repo for a filtered list:
//
//	func ActiveMessageContentBlocks(ctx context.Context, db orm.Context) ([]*MessageContentBlock, error) {
//		return ListMessageContentBlock(ctx, db, orm.WhereEq("message_id", true))
//	}
package store

import (
	"context"

	"github.com/reliant-labs/forge/pkg/orm"
)

// AppendToContentBlock appends a streamed delta to a block — the hottest
// write on the streaming path (one per provider delta). bun builder, no raw
// SQL. The delta is interpolated into the statement text (bun formats every
// argument client-side), so its bytes travel inside the SQL string.
func AppendToContentBlock(ctx context.Context, db orm.Context, blockID, delta string) error {
	delta = validUTF8(delta)
	if delta == "" {
		return nil
	}
	_, err := db.Bun().NewUpdate().
		Model((*MessageContentBlock)(nil)).
		Set("content = COALESCE(content, '') || ?", delta).
		Set("version = version + 1").
		Set("updated_at = NOW()").
		Where("id = ?", blockID).
		Exec(ctx)
	return err
}

// ListContentBlocksForMessages is the read path's batch block load: the
// GENERATED ListMessageContentBlock with QueryOptions, no hand query at all.
func ListContentBlocksForMessages(ctx context.Context, db orm.Context, messageIDs []string) ([]*MessageContentBlock, error) {
	if len(messageIDs) == 0 {
		return []*MessageContentBlock{}, nil
	}
	return ListMessageContentBlock(ctx, db,
		orm.WhereIn(MessageContentBlockFieldMessageId, messageIDs),
		orm.WithOrderBy(MessageContentBlockFieldMessageId, orm.Asc),
		orm.WithOrderBy(MessageContentBlockFieldPosition, orm.Asc),
	)
}

// MessageContentBlockExtra carries fields on MessageContentBlock that are NOT columns — computed
// values, transient state, anything derived rather than stored. MessageContentBlock is
// embedded on MessageContentBlockExtra (see message_content_block_orm.go), so a field added here is a field on
// every MessageContentBlock value.
//
// Every field MUST carry the `bun:"-"` tag. Bun otherwise treats an
// embedded struct's fields as columns of the parent table and tries to
// persist them — `bun:"-"` is Bun's own idiom for a field it must never
// bind or scan.
type MessageContentBlockExtra struct {
}
