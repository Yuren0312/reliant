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
