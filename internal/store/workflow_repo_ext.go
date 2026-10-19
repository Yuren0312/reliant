// yours: scaffolded once, never touched again — forge will not overwrite this file.
//
// This is the custom-query seam for Workflow. The generated workflow_orm.go is
// Tier-1 (a live projection of the migrations, regenerated every run);
// hand-written queries that outgrow the generated CRUD belong HERE, in a
// sibling file forge never rewrites. You have three building blocks:
//
//  1. The generated repo + delegates: workflowRepo (a *crud.Repo[Workflow]) and the
//     package-level GetWorkflowByID / ListWorkflow / CreateWorkflow / UpdateWorkflow / DeleteWorkflow
//     functions — compose orm.QueryOption filters (orm.WhereEq,
//     orm.WhereILikeAny, orm.WithOrderBy, orm.WithLimit) onto List/Count.
//  2. Raw SQL via the Bun handle: db.Bun() (bun.IDB) gives you
//     NewSelect()/NewRaw()/bun.Raw — for joins, aggregates, CTEs, or
//     anything the builder can't express. Scan into the generated
//     Workflow struct (or your own row type).
//  3. orm.Context: every query takes one. Take no transaction parameter:
//     called with the ctx that s.deps.DB.RunTx hands its fn, a method
//     here (db.Bun() builders and raw SQL included) joins that
//     transaction; with any other ctx it runs on the pool.
//
// Example (uncomment and adapt):
//
//	func FindWorkflowBy(ctx context.Context, db orm.Context, /* args */) ([]*Workflow, error) {
//		var rows []*Workflow
//		err := db.Bun().NewSelect().Model(&rows).
//			Where("parent_id = ?", /* value */).
//			Scan(ctx)
//		return rows, err
//	}
//
// Or reuse the generated repo for a filtered list:
//
//	func ActiveWorkflows(ctx context.Context, db orm.Context) ([]*Workflow, error) {
//		return ListWorkflow(ctx, db, orm.WhereEq("parent_id", true))
//	}
package store

import (
	"context"

	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/uptrace/bun"
)

// workflowDescendants is the recursive CTE body shared by the cascades: every
// workflow under parent_id = ? at any depth. Built with bun's set-operation
// builder (UnionAll) over two selects; the recursive self-reference is a
// TableExpr, because the CTE has no model.
func workflowDescendants(db orm.Context, parentID string) bunQuery {
	anchor := db.Bun().NewSelect().TableExpr("workflows AS c").Column("c.id").Where("c.parent_id = ?", parentID)
	step := db.Bun().NewSelect().TableExpr("workflows AS g").Column("g.id").
		Join("JOIN descendants AS d ON g.parent_id = d.id")
	return anchor.UnionAll(step)
}

// CascadeTerminalStatusToDescendants moves every LIVE descendant (active, or
// paused) of parentID to STOPPED with the caller's stop reason — the
// forge-ORM port of the sqlc query of the same name.
func CascadeTerminalStatusToDescendants(ctx context.Context, db orm.Context, parentID string, stopReason int32) error {
	_, err := db.Bun().NewUpdate().
		WithRecursive("descendants", workflowDescendants(db, parentID)).
		Model((*Workflow)(nil)).
		ModelTableExpr("workflows AS t").
		TableExpr("descendants").
		Set("state = 3").
		Set("stop_reason = ?", stopReason).
		Set("completed_at = NOW()").
		Where("t.id = descendants.id").
		Where("(t.state = 2 OR (t.state = 3 AND t.stop_reason = 3))").
		Exec(ctx)
	return err
}

// ListWorkflowDescendantIDs reads the subtree under parentID through the same
// CTE: a recursive SELECT, for comparison with the UPDATE form.
func ListWorkflowDescendantIDs(ctx context.Context, db orm.Context, parentID string) ([]string, error) {
	var ids []string
	err := db.Bun().NewSelect().
		WithRecursive("descendants", workflowDescendants(db, parentID)).
		TableExpr("descendants").
		Column("id").
		OrderExpr("id").
		Scan(ctx, &ids)
	return ids, err
}

// WorkflowExtra carries fields on Workflow that are NOT columns — computed
// values, transient state, anything derived rather than stored. Workflow is
// embedded on WorkflowExtra (see workflow_orm.go), so a field added here is a field on
// every Workflow value.
//
// Every field MUST carry the `bun:"-"` tag. Bun otherwise treats an
// embedded struct's fields as columns of the parent table and tries to
// persist them — `bun:"-"` is Bun's own idiom for a field it must never
// bind or scan.
type WorkflowExtra struct {
}

// bunQuery is what WithRecursive accepts: any bun query.
type bunQuery = bun.Query
