package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/reliant-labs/reliant/internal/db/core"
	pgdb "github.com/reliant-labs/reliant/internal/db/postgres/generated"
)

// TriggerFromRow maps a triggers row read by a sqlc query outside the trigger
// aggregate: the access-gated routing query (integration_access.sql) joins
// triggers to integration_event_access. Every trigger query proper lives in
// internal/store; this goes when that routing query ports too (P5b lane L5).
func TriggerFromRow(row pgdb.Trigger) (*core.Trigger, error) {
	presets := map[string]string{}
	if len(row.Presets) > 0 {
		if err := json.Unmarshal(row.Presets, &presets); err != nil {
			return nil, fmt.Errorf("unmarshal trigger presets (trigger %s): %w", row.ID, err)
		}
	}
	params := map[string]any{}
	if len(row.Params) > 0 {
		if err := json.Unmarshal(row.Params, &params); err != nil {
			return nil, fmt.Errorf("unmarshal trigger params (trigger %s): %w", row.ID, err)
		}
	}
	var config json.RawMessage
	if len(row.Config) > 0 {
		config = append(json.RawMessage(nil), row.Config...)
	}
	return &core.Trigger{
		ID:               row.ID,
		UserID:           row.UserID,
		ProjectID:        row.ProjectID,
		WorktreeID:       nullStringToPtr(row.WorktreeID),
		Name:             row.Name,
		Kind:             core.TriggerKind(row.Kind),
		Enabled:          row.Enabled,
		Workflow:         row.Workflow,
		Presets:          presets,
		Params:           params,
		Message:          row.Message,
		Config:           config,
		Filter:           row.Filter,
		ConnectionID:     nullStringToPtr(row.ConnectionID),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		DaemonID:         row.DaemonID.String,
		NoMachine:        row.NoMachine,
		ActingIdentity:   core.ActingIdentity(row.ActingIdentity),
		NotifyOnComplete: row.NotifyOnComplete,
		WorkflowTrigger:  nullStringToPtr(row.WorkflowTrigger),
	}, nil
}

// triggerMapToJSON always produces a jsonb OBJECT, never a JSON null: a nil Go
// map marshals to `null`, which reads back as a nil map and would force every
// consumer to nil-check a field the schema declares NOT NULL with a '{}'
// default. run_event_store.go's payloads share it.
func triggerMapToJSON[V any](m map[string]V) (json.RawMessage, error) {
	if m == nil {
		return json.RawMessage(`{}`), nil
	}
	return json.Marshal(m)
}

func triggerJSONToMap(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
