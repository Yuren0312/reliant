// yours: scaffolded once, never touched again — forge will not overwrite this file.
//
// This is the custom-query seam for Trigger. The generated trigger_orm.go is
// Tier-1 (a live projection of the migrations, regenerated every run);
// hand-written queries that outgrow the generated CRUD belong HERE, in a
// sibling file forge never rewrites. You have three building blocks:
//
//  1. The generated repo + delegates: triggerRepo (a *crud.Repo[Trigger]) and the
//     package-level GetTriggerByID / ListTrigger / CreateTrigger / UpdateTrigger / DeleteTrigger
//     functions — compose orm.QueryOption filters (orm.WhereEq,
//     orm.WhereILikeAny, orm.WithOrderBy, orm.WithLimit) onto List/Count.
//  2. Raw SQL via the Bun handle: db.Bun() (bun.IDB) gives you
//     NewSelect()/NewRaw()/bun.Raw — for joins, aggregates, CTEs, or
//     anything the builder can't express. Scan into the generated
//     Trigger struct (or your own row type).
//  3. orm.Context: every query takes one. Take no transaction parameter:
//     called with the ctx that s.deps.DB.RunTx hands its fn, a method
//     here (db.Bun() builders and raw SQL included) joins that
//     transaction; with any other ctx it runs on the pool.
//
// Example (uncomment and adapt):
//
//	func FindTriggerBy(ctx context.Context, db orm.Context, /* args */) ([]*Trigger, error) {
//		var rows []*Trigger
//		err := db.Bun().NewSelect().Model(&rows).
//			Where("user_id = ?", /* value */).
//			Scan(ctx)
//		return rows, err
//	}
//
// Or reuse the generated repo for a filtered list:
//
//	func ActiveTriggers(ctx context.Context, db orm.Context) ([]*Trigger, error) {
//		return ListTrigger(ctx, db, orm.WhereEq("user_id", true))
//	}
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/reliant-labs/forge/pkg/crud"
	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/reliant-labs/forge/pkg/svcerr"

	"github.com/reliant-labs/reliant/internal/db/core"
)

// InsertTrigger stores a new trigger definition. It is the bun builder rather
// than the generated CreateTrigger, which re-stamps updated_at with its own
// clock: the caller's CreatedAt/UpdatedAt are the row's.
func InsertTrigger(ctx context.Context, db orm.Context, t *core.Trigger) error {
	row, err := triggerFromCore(t)
	if err != nil {
		return err
	}
	_, err = db.Bun().NewInsert().Model(row).
		ExcludeColumn(TriggerFieldWebhookTokenHash, TriggerFieldWebhookSecretSealed).
		Exec(ctx)
	return err
}

// triggerColumnsT is the triggers columns a core.Trigger carries, qualified by
// the alias t, for the hand-written reads that join triggers to other tables.
// triggerScanDest is its scan target, column for column. The webhook
// credentials are not here: they are read only by GetTriggerWebhookCredentials.
const triggerColumnsT = `t.id, t.user_id, t.project_id, t.worktree_id, t.name, t.kind, t.enabled,
	t.workflow, t.presets, t.params, t.message, t.config, t.created_at, t.updated_at,
	t.daemon_id, t.notify_on_complete, t.filter, t.connection_id, t.workflow_trigger,
	t.no_machine, t.acting_identity`

func triggerScanDest(row *Trigger) []any {
	return []any{
		&row.ID, &row.UserID, &row.ProjectID, &row.WorktreeID, &row.Name, (*string)(&row.Kind), &row.Enabled,
		&row.Workflow, (*[]byte)(&row.Presets), (*[]byte)(&row.Params), &row.Message, (*[]byte)(&row.Config), &row.CreatedAt, &row.UpdatedAt,
		&row.DaemonID, &row.NotifyOnComplete, &row.Filter, &row.ConnectionID, &row.WorkflowTrigger,
		&row.NoMachine, (*string)(&row.ActingIdentity),
	}
}

// getTriggerSQL joins the display names so a trigger can be shown without a
// second lookup. LEFT JOINs: a daemon id is not a foreign key, and an absent
// name must not hide the trigger.
//
// It and the other joined trigger reads below run through orm.Context's raw
// passthrough with bound $N parameters rather than NewRaw: GetTrigger is on
// every fire and webhook delivery, and ListIntegrationTriggers on every
// inbound integration event, so their text must not vary per call
// (internal/db/SCHEMA.md, "A hot hand-tuned statement binds its values").
const getTriggerSQL = `SELECT ` + triggerColumnsT + `,
	COALESCE(p.name, '')::text AS project_name,
	COALESCE(d.hostname, '')::text AS daemon_name
FROM triggers t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN daemons d ON d.id = t.daemon_id AND d.user_id = t.user_id
WHERE t.id = $1`

// GetTrigger returns the trigger with its project and daemon display names,
// or core.ErrTriggerNotFound.
func GetTrigger(ctx context.Context, db orm.Context, id string) (*core.Trigger, error) {
	var row Trigger
	var projectName, daemonName string
	dest := append(triggerScanDest(&row), &projectName, &daemonName)
	if err := db.QueryRow(ctx, getTriggerSQL, id).Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrTriggerNotFound
		}
		return nil, fmt.Errorf("failed to get trigger: %w", err)
	}
	t, err := triggerToCore(&row)
	if err != nil {
		return nil, err
	}
	t.ProjectName, t.DaemonName = projectName, daemonName
	return t, nil
}

// listTriggersSQL is always scoped to one user; project_id narrows further.
// The unscoped listing is ListAllTriggers, a separate query so "no user" can
// never be reached by passing an empty string.
const listTriggersSQL = `SELECT ` + triggerColumnsT + `,
	COALESCE(p.name, '')::text AS project_name,
	COALESCE(d.hostname, '')::text AS daemon_name
FROM triggers t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN daemons d ON d.id = t.daemon_id AND d.user_id = t.user_id
WHERE t.user_id = $1
	AND ($2::text IS NULL OR t.project_id = $2::text)
ORDER BY t.created_at DESC, t.id`

// ListTriggersForUser lists one user's triggers, newest first, with their
// display names; f.ProjectID narrows to one project.
func ListTriggersForUser(ctx context.Context, db orm.Context, f core.TriggerFilters) ([]*core.Trigger, error) {
	rows, err := db.Query(ctx, listTriggersSQL, f.UserID, f.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list triggers: %w", err)
	}
	defer rows.Close()
	out := []*core.Trigger{}
	for rows.Next() {
		var row Trigger
		var projectName, daemonName string
		if err := rows.Scan(append(triggerScanDest(&row), &projectName, &daemonName)...); err != nil {
			return nil, fmt.Errorf("failed to list triggers: %w", err)
		}
		t, err := triggerToCore(&row)
		if err != nil {
			return nil, err
		}
		t.ProjectName, t.DaemonName = projectName, daemonName
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list triggers: %w", err)
	}
	return out, nil
}

// ListAllTriggers lists every user's triggers, newest first. Only the
// schedule syncer's reconciliation calls it. It is the builder, not the
// generated ListTrigger, whose default limit would silently stop at 10,000
// rows: a reconcile that never sees a trigger never schedules it.
func ListAllTriggers(ctx context.Context, db orm.Context) ([]*core.Trigger, error) {
	var rows []*Trigger
	if err := db.Bun().NewSelect().Model(&rows).OrderExpr("created_at DESC, id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list all triggers: %w", err)
	}
	return triggersToCore(rows)
}

// LockTrigger row-locks the trigger for the rest of the surrounding
// transaction, serializing the overlap check with the launch it guards across
// concurrent fires. A missing trigger is core.ErrTriggerNotFound.
func LockTrigger(ctx context.Context, db orm.Context, id string) error {
	var locked string
	if err := db.QueryRow(ctx, `SELECT id FROM triggers WHERE id = $1 FOR UPDATE`, id).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return core.ErrTriggerNotFound
		}
		return fmt.Errorf("failed to lock trigger: %w", err)
	}
	return nil
}

// triggerUpdatableColumns are what ReplaceTrigger writes. Identity columns
// (id, user_id, kind) are not updatable: changing the owner would silently
// re-point which identity the run executes as, and changing the kind would
// leave Config describing a source that no longer applies. The webhook
// credentials have their own write paths, so no definition update can touch
// them.
var triggerUpdatableColumns = []string{
	TriggerFieldProjectID, TriggerFieldWorktreeID, TriggerFieldName, TriggerFieldEnabled,
	TriggerFieldWorkflow, TriggerFieldPresets, TriggerFieldParams, TriggerFieldMessage,
	TriggerFieldConfig, TriggerFieldUpdatedAt, TriggerFieldDaemonID, TriggerFieldNotifyOnComplete,
	TriggerFieldFilter, TriggerFieldConnectionID, TriggerFieldWorkflowTrigger, TriggerFieldNoMachine,
	TriggerFieldActingIdentity,
}

// ReplaceTrigger rewrites a trigger's definition, or returns
// core.ErrTriggerNotFound. It is the builder rather than the generated
// UpdateTriggerMasked, which re-stamps updated_at with its own clock: the
// caller's UpdatedAt is the row's.
func ReplaceTrigger(ctx context.Context, db orm.Context, t *core.Trigger) error {
	row, err := triggerFromCore(t)
	if err != nil {
		return err
	}
	res, err := db.Bun().NewUpdate().Model(row).Column(triggerUpdatableColumns...).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update trigger: %w", err)
	}
	return triggerTouched(res, core.ErrTriggerNotFound)
}

// RemoveTrigger deletes a trigger. Deleting one that does not exist is not an
// error.
func RemoveTrigger(ctx context.Context, db orm.Context, id string) error {
	if err := DeleteTrigger(ctx, db, id); err != nil && !errors.Is(err, orm.ErrNoRows) {
		return err
	}
	return nil
}

// SetTriggerEnabled turns a trigger on or off, or returns
// core.ErrTriggerNotFound.
func SetTriggerEnabled(ctx context.Context, db orm.Context, id string, enabled bool) error {
	return stampTrigger(ctx, db, &Trigger{ID: id, Enabled: &enabled}, TriggerFieldEnabled)
}

// SetTriggerWebhookTokenHash rotates a webhook trigger's token hash. The hash
// has its own write path: no definition update can touch it, and rotating it
// is one statement. A missing trigger is core.ErrTriggerNotFound.
func SetTriggerWebhookTokenHash(ctx context.Context, db orm.Context, id string, hash []byte) error {
	return stampTrigger(ctx, db, &Trigger{ID: id, WebhookTokenHash: hash}, TriggerFieldWebhookTokenHash)
}

// SetTriggerWebhookSecret stores the HMAC secret, sealed by the caller under
// the owner's tenant; nil clears it. A missing trigger is
// core.ErrTriggerNotFound.
func SetTriggerWebhookSecret(ctx context.Context, db orm.Context, id string, sealed []byte) error {
	return stampTrigger(ctx, db, &Trigger{ID: id, WebhookSecretSealed: sealed}, TriggerFieldWebhookSecretSealed)
}

// stampTrigger writes one column of a trigger and re-stamps updated_at.
func stampTrigger(ctx context.Context, db orm.Context, row *Trigger, field string) error {
	err := triggerRepo.UpdateMasked(ctx, db, row, []string{field}, crud.NoReadBack())
	if errors.Is(err, orm.ErrNoRows) {
		return core.ErrTriggerNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to set trigger %s: %w", field, err)
	}
	return nil
}

// GetTriggerWebhookCredentials returns a webhook trigger's token hash and
// sealed secret, or core.ErrTriggerNotFound. Only the webhook receiver reads
// these; they are never rendered.
func GetTriggerWebhookCredentials(ctx context.Context, db orm.Context, id string) (*core.TriggerWebhookCredentials, error) {
	row, err := GetTriggerByID(ctx, db, id)
	if errors.Is(err, svcerr.ErrNotFound) {
		return nil, core.ErrTriggerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook credentials: %w", err)
	}
	return &core.TriggerWebhookCredentials{TokenHash: row.WebhookTokenHash, SecretSealed: row.WebhookSecretSealed}, nil
}

// SetTriggerProjection refreshes an activation's projection of its workflow's
// declaration: the config and filter routing and the schedule syncer read.
// The kind is not here: an activation whose declaration changed kind is
// broken (its connection or webhook token belongs to the old kind), never
// re-projected. An id that is not an activation is core.ErrTriggerNotFound.
func SetTriggerProjection(ctx context.Context, db orm.Context, id string, p core.TriggerProjection) error {
	res, err := db.Bun().NewUpdate().Model((*Trigger)(nil)).
		Set("config = ?", string(triggerConfigJSON(p.Config))).
		Set("filter = ?", p.Filter).
		Set("updated_at = NOW()").
		Where("id = ?", id).
		Where("workflow_trigger IS NOT NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set trigger projection: %w", err)
	}
	return triggerTouched(res, core.ErrTriggerNotFound)
}

// ListAllWorkflowTriggerActivations lists every activation of a declared
// trigger, for the periodic reconcile. Like ListAllTriggers it is the builder,
// unbounded, because the generated list stops at 10,000 rows.
func ListAllWorkflowTriggerActivations(ctx context.Context, db orm.Context) ([]*core.Trigger, error) {
	var rows []*Trigger
	if err := db.Bun().NewSelect().Model(&rows).Where("workflow_trigger IS NOT NULL").OrderExpr("id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list workflow trigger activations: %w", err)
	}
	return triggersToCore(rows)
}

// listIntegrationTriggersSQL is every ENABLED integration trigger for one
// integration, with its connection's routing identity. The inner join drops
// triggers whose connection is gone, revoked or belongs to someone else: an
// event can only reach a trigger through its own owner's live connection.
// Status is returned rather than filtered so a caller can tell needs_reauth
// apart. Served by idx_triggers_integration.
const listIntegrationTriggersSQL = `SELECT ` + triggerColumnsT + `,
	c.external_account_id AS connection_account,
	c.status AS connection_status
FROM triggers t
JOIN connections c
	ON c.id = t.connection_id
	AND c.user_id = t.user_id
	AND c.owner_kind = 'user'
	AND c.deleted_at IS NULL
WHERE t.kind = 'integration'
	AND t.enabled
	AND t.config ->> 'integration' = $1::text
ORDER BY t.id`

// ListIntegrationTriggers lists the routes an inbound event of one
// integration may take.
func ListIntegrationTriggers(ctx context.Context, db orm.Context, integration string) ([]*core.IntegrationTriggerRoute, error) {
	rows, err := db.Query(ctx, listIntegrationTriggersSQL, integration)
	if err != nil {
		return nil, fmt.Errorf("failed to list integration triggers: %w", err)
	}
	defer rows.Close()
	out := []*core.IntegrationTriggerRoute{}
	for rows.Next() {
		var row Trigger
		var account sql.NullString
		var status string
		if err := rows.Scan(append(triggerScanDest(&row), &account, &status)...); err != nil {
			return nil, fmt.Errorf("failed to list integration triggers: %w", err)
		}
		t, err := triggerToCore(&row)
		if err != nil {
			return nil, err
		}
		out = append(out, &core.IntegrationTriggerRoute{Trigger: t, ConnectionAccount: account.String, ConnectionStatus: status})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list integration triggers: %w", err)
	}
	return out, nil
}

// triggerTouched maps a write that matched no row to notFound.
func triggerTouched(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}

func triggerFromCore(t *core.Trigger) (*Trigger, error) {
	presets, err := triggerJSONObject(t.Presets)
	if err != nil {
		return nil, fmt.Errorf("marshal trigger presets: %w", err)
	}
	params, err := triggerJSONObject(t.Params)
	if err != nil {
		return nil, fmt.Errorf("marshal trigger params: %w", err)
	}
	enabled := t.Enabled
	row := &Trigger{
		ID:               t.ID,
		UserID:           t.UserID,
		ProjectID:        t.ProjectID,
		WorktreeID:       t.WorktreeID,
		Name:             t.Name,
		Kind:             t.Kind,
		Enabled:          &enabled,
		Workflow:         t.Workflow,
		Presets:          presets,
		Params:           params,
		Message:          t.Message,
		Config:           triggerConfigJSON(t.Config),
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		NotifyOnComplete: t.NotifyOnComplete,
		Filter:           t.Filter,
		ConnectionID:     t.ConnectionID,
		WorkflowTrigger:  t.WorkflowTrigger,
		NoMachine:        t.NoMachine,
		ActingIdentity:   t.ActingIdentity.OrOwner(),
	}
	// A no-machine trigger's empty daemon is NULL, which is what
	// triggers_no_machine_names_no_daemon_check pairs with no_machine.
	if t.DaemonID != "" {
		daemonID := t.DaemonID
		row.DaemonID = &daemonID
	}
	return row, nil
}

func triggerToCore(row *Trigger) (*core.Trigger, error) {
	presets := map[string]string{}
	if err := triggerJSONInto(row.Presets, &presets); err != nil {
		return nil, fmt.Errorf("unmarshal trigger presets (trigger %s): %w", row.ID, err)
	}
	params := map[string]any{}
	if err := triggerJSONInto(row.Params, &params); err != nil {
		return nil, fmt.Errorf("unmarshal trigger params (trigger %s): %w", row.ID, err)
	}
	var config json.RawMessage
	if len(row.Config) > 0 {
		config = append(json.RawMessage(nil), row.Config...)
	}
	t := &core.Trigger{
		ID:               row.ID,
		UserID:           row.UserID,
		ProjectID:        row.ProjectID,
		WorktreeID:       row.WorktreeID,
		Name:             row.Name,
		Kind:             row.Kind,
		Enabled:          row.Enabled != nil && *row.Enabled,
		Workflow:         row.Workflow,
		Presets:          presets,
		Params:           params,
		Message:          row.Message,
		Config:           config,
		Filter:           row.Filter,
		ConnectionID:     row.ConnectionID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		NoMachine:        row.NoMachine,
		ActingIdentity:   row.ActingIdentity,
		NotifyOnComplete: row.NotifyOnComplete,
		WorkflowTrigger:  row.WorkflowTrigger,
	}
	if row.DaemonID != nil {
		t.DaemonID = *row.DaemonID
	}
	return t, nil
}

func triggersToCore(rows []*Trigger) ([]*core.Trigger, error) {
	out := make([]*core.Trigger, 0, len(rows))
	for _, row := range rows {
		t, err := triggerToCore(row)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// triggerJSONObject always produces a jsonb OBJECT, never a JSON null: a nil
// Go map marshals to `null`, which reads back as a nil map and would force
// every consumer to nil-check a field the schema declares NOT NULL with a
// '{}' default.
func triggerJSONObject[V any](m map[string]V) (json.RawMessage, error) {
	if m == nil {
		return json.RawMessage(`{}`), nil
	}
	return json.Marshal(m)
}

// triggerConfigJSON passes a stored config through, substituting an empty
// object for an absent one so the NOT NULL column is always satisfied.
func triggerConfigJSON(config json.RawMessage) json.RawMessage {
	if len(config) == 0 {
		return json.RawMessage(`{}`)
	}
	return config
}

func triggerJSONInto(raw []byte, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// TriggerExtra carries fields on Trigger that are NOT columns — computed
// values, transient state, anything derived rather than stored. Trigger is
// embedded on TriggerExtra (see trigger_orm.go), so a field added here is a field on
// every Trigger value.
//
// Every field MUST carry the `bun:"-"` tag. Bun otherwise treats an
// embedded struct's fields as columns of the parent table and tries to
// persist them — `bun:"-"` is Bun's own idiom for a field it must never
// bind or scan.
type TriggerExtra struct {
}
