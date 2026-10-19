// yours: scaffolded once, never touched again — forge will not overwrite this file.
//
// This is the custom-query seam for RunEvent. The generated run_event_orm.go is
// Tier-1 (a live projection of the migrations, regenerated every run);
// hand-written queries that outgrow the generated CRUD belong HERE, in a
// sibling file forge never rewrites. You have three building blocks:
//
//  1. The generated repo + delegates: runEventRepo (a *crud.Repo[RunEvent]) and the
//     package-level GetRunEventByID / ListRunEvent / CreateRunEvent / UpdateRunEvent / DeleteRunEvent
//     functions — compose orm.QueryOption filters (orm.WhereEq,
//     orm.WhereILikeAny, orm.WithOrderBy, orm.WithLimit) onto List/Count.
//  2. Raw SQL via the Bun handle: db.Bun() (bun.IDB) gives you
//     NewSelect()/NewRaw()/bun.Raw — for joins, aggregates, CTEs, or
//     anything the builder can't express. Scan into the generated
//     RunEvent struct (or your own row type).
//  3. orm.Context: every query takes one. Take no transaction parameter:
//     called with the ctx that s.deps.DB.RunTx hands its fn, a method
//     here (db.Bun() builders and raw SQL included) joins that
//     transaction; with any other ctx it runs on the pool.
//
// Example (uncomment and adapt):
//
//	func FindRunEventBy(ctx context.Context, db orm.Context, /* args */) ([]*RunEvent, error) {
//		var rows []*RunEvent
//		err := db.Bun().NewSelect().Model(&rows).
//			Where("user_id = ?", /* value */).
//			Scan(ctx)
//		return rows, err
//	}
//
// Or reuse the generated repo for a filtered list:
//
//	func ActiveRunEvents(ctx context.Context, db orm.Context) ([]*RunEvent, error) {
//		return ListRunEvent(ctx, db, orm.WhereEq("user_id", true))
//	}
package store

import (
	"context"
	"time"

	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/uptrace/bun"
)

// ClaimRunEvents leases up to max undispatched run events whose lease is free
// or expired — the forge-ORM port of the sqlc ClaimRunEvents query. FOR UPDATE
// SKIP LOCKED in the inner select lets several relays drain the outbox without
// handing one row to two of them. Expressed with bun's builder: no raw SQL.
func ClaimRunEvents(ctx context.Context, db orm.Context, now, leaseUntil time.Time, max int) ([]*RunEvent, error) {
	if max <= 0 {
		return nil, nil
	}
	claimable := db.Bun().NewSelect().
		TableExpr("run_events AS r").
		Column("r.id").
		Where("r.dispatched_at IS NULL").
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("r.claimed_until IS NULL").WhereOr("r.claimed_until < ?", now)
		}).
		OrderExpr("r.created_at, r.id").
		Limit(int64(max)).
		For("UPDATE SKIP LOCKED")

	var rows []*RunEvent
	err := db.Bun().NewUpdate().
		Model((*RunEvent)(nil)).
		Set("claimed_until = ?", leaseUntil).
		Where("id IN (?)", claimable).
		Returning("*").
		Scan(ctx, &rows)
	return rows, err
}

// MarkRunEventDispatched stamps dispatched_at through the generated masked
// update: no hand SQL at all.
func MarkRunEventDispatched(ctx context.Context, db orm.Context, id string, at time.Time) error {
	return UpdateRunEventMasked(ctx, db, &RunEvent{Id: id, DispatchedAt: &at}, []string{RunEventFieldDispatchedAt})
}

// RunEventExtra carries fields on RunEvent that are NOT columns — computed
// values, transient state, anything derived rather than stored. RunEvent is
// embedded on RunEventExtra (see run_event_orm.go), so a field added here is a field on
// every RunEvent value.
//
// Every field MUST carry the `bun:"-"` tag. Bun otherwise treats an
// embedded struct's fields as columns of the parent table and tries to
// persist them — `bun:"-"` is Bun's own idiom for a field it must never
// bind or scan.
type RunEventExtra struct {
}
