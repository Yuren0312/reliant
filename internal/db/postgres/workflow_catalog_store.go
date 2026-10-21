package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/reliant-labs/forge/pkg/orm"

	"github.com/reliant-labs/reliant/internal/db/core"
	pgdb "github.com/reliant-labs/reliant/internal/db/postgres/generated"
	"github.com/reliant-labs/reliant/internal/store"
)

// workflowCatalogStore is core.WorkflowCatalogStore. The workflow_drafts and
// workflow_scenarios halves run on the forge ORM (internal/store's
// workflow_draft_repo_ext.go and workflow_scenario_repo_ext.go).
type workflowCatalogStore struct {
	db orm.Context
	// P5b L7: the presets, still sqlc (queries/presets.sql).
	q pgdb.Querier
}

// NewWorkflowCatalogStore creates the workflow catalog store implementation.
func NewWorkflowCatalogStore(db orm.Context, q pgdb.Querier) core.WorkflowCatalogStore {
	return &workflowCatalogStore{db: db, q: q}
}

func (s *workflowCatalogStore) CreateWorkflowDraft(ctx context.Context, draft *core.WorkflowDraft) error {
	return store.InsertWorkflowDraft(ctx, s.db, draft)
}

func (s *workflowCatalogStore) UpsertWorkflowDraft(ctx context.Context, draft *core.WorkflowDraft) (*core.WorkflowDraft, error) {
	return store.UpsertWorkflowDraftBySlug(ctx, s.db, draft)
}

func (s *workflowCatalogStore) GetWorkflowDraft(ctx context.Context, id string) (*core.WorkflowDraft, error) {
	return store.LoadWorkflowDraft(ctx, s.db, id)
}

func (s *workflowCatalogStore) GetWorkflowDraftBySlug(ctx context.Context, userID, slug string) (*core.WorkflowDraft, error) {
	return store.FindWorkflowDraftBySlug(ctx, s.db, userID, slug)
}

func (s *workflowCatalogStore) GetWorkflowDraftByName(ctx context.Context, userID, name string) (*core.WorkflowDraft, error) {
	return store.FindWorkflowDraftByName(ctx, s.db, userID, name)
}

func (s *workflowCatalogStore) GetWorkflowDraftBySourcePath(ctx context.Context, userID, sourcePath string) (*core.WorkflowDraft, error) {
	return store.FindWorkflowDraftBySourcePath(ctx, s.db, userID, sourcePath)
}

func (s *workflowCatalogStore) GetUsableWorkflowBySlug(ctx context.Context, userID, slug string) (*core.WorkflowDraft, error) {
	return store.FindUsableWorkflowBySlug(ctx, s.db, userID, slug)
}

func (s *workflowCatalogStore) ListWorkflowDraftsByUser(ctx context.Context, userID string) ([]*core.WorkflowDraft, error) {
	return store.ListUserWorkflowDrafts(ctx, s.db, userID)
}

func (s *workflowCatalogStore) UpdateWorkflowDraft(ctx context.Context, draft *core.WorkflowDraft) error {
	return store.ReplaceWorkflowDraft(ctx, s.db, draft)
}

func (s *workflowCatalogStore) UpdateWorkflowDraftDefinition(ctx context.Context, id string, name string, slug string, definition string, status core.WorkflowDraftStatus) error {
	return store.SetWorkflowDraftDefinition(ctx, s.db, id, name, slug, definition, status)
}

func (s *workflowCatalogStore) SetWorkflowDraftStatus(ctx context.Context, id string, status core.WorkflowDraftStatus) (*core.WorkflowDraft, error) {
	return store.SetWorkflowDraftStatus(ctx, s.db, id, status)
}

func (s *workflowCatalogStore) SetWorkflowDraftHidden(ctx context.Context, id string, isHidden bool) (*core.WorkflowDraft, error) {
	return store.SetWorkflowDraftHidden(ctx, s.db, id, isHidden)
}

// DeleteWorkflowDraft returns the deleted row, or sql.ErrNoRows when none matched.
func (s *workflowCatalogStore) DeleteWorkflowDraft(ctx context.Context, id string) (*core.WorkflowDraft, error) {
	return store.RemoveWorkflowDraft(ctx, s.db, id)
}

// DeleteWorkflowDraftBySlug returns the deleted row, or sql.ErrNoRows when none matched.
func (s *workflowCatalogStore) DeleteWorkflowDraftBySlug(ctx context.Context, userID, slug string) (*core.WorkflowDraft, error) {
	return store.RemoveWorkflowDraftBySlug(ctx, s.db, userID, slug)
}

func (s *workflowCatalogStore) WorkflowSlugExists(ctx context.Context, userID, slug string) (bool, error) {
	return store.WorkflowSlugExists(ctx, s.db, userID, slug)
}

func (s *workflowCatalogStore) CountWorkflowDraftsByUser(ctx context.Context, userID string) (int64, error) {
	return store.CountUserWorkflowDrafts(ctx, s.db, userID)
}

func (s *workflowCatalogStore) UpdateWorkflowForkedFrom(ctx context.Context, draftID string, forkedFrom string) (*core.WorkflowDraft, error) {
	return store.SetWorkflowDraftForkedFrom(ctx, s.db, draftID, forkedFrom)
}

func (s *workflowCatalogStore) CreatePreset(ctx context.Context, preset *core.Preset) (*core.Preset, error) {
	params, err := presetToCreateParams(preset)
	if err != nil {
		return nil, err
	}
	row, err := s.q.CreatePreset(ctx, params)
	if err != nil {
		return nil, err
	}
	return presetFromPG(row), nil
}

func (s *workflowCatalogStore) UpsertPreset(ctx context.Context, preset *core.Preset) (*core.Preset, error) {
	params, err := presetToUpsertParams(preset)
	if err != nil {
		return nil, err
	}
	row, err := s.q.UpsertPreset(ctx, params)
	if err != nil {
		return nil, err
	}
	return presetFromPG(row), nil
}

func (s *workflowCatalogStore) GetPreset(ctx context.Context, id string) (*core.Preset, error) {
	row, err := s.q.GetPreset(ctx, id)
	if err != nil {
		return nil, err
	}
	return presetFromPG(row), nil
}

func (s *workflowCatalogStore) GetPresetBySlug(ctx context.Context, userID, slug string) (*core.Preset, error) {
	row, err := s.q.GetPresetBySlug(ctx, pgdb.GetPresetBySlugParams{UserID: userID, Slug: slug})
	if err != nil {
		return nil, err
	}
	return presetFromPG(row), nil
}

func (s *workflowCatalogStore) GetPresetBySlugAndProject(ctx context.Context, userID, slug, projectID string) (*core.Preset, error) {
	row, err := s.q.GetPresetBySlugAndProject(ctx, pgdb.GetPresetBySlugAndProjectParams{UserID: userID, Slug: slug, ProjectID: workflowPtrToNullString(&projectID)})
	if err != nil {
		return nil, err
	}
	return presetFromPG(row), nil
}

func (s *workflowCatalogStore) ListUserPresets(ctx context.Context, userID string) ([]*core.Preset, error) {
	rows, err := s.q.ListUserPresets(ctx, userID)
	if err != nil {
		return nil, err
	}
	return presetsFromPG(rows), nil
}

func (s *workflowCatalogStore) ListUserPresetsGlobal(ctx context.Context, userID string) ([]*core.Preset, error) {
	rows, err := s.q.ListUserPresetsGlobal(ctx, userID)
	if err != nil {
		return nil, err
	}
	return presetsFromPG(rows), nil
}

func (s *workflowCatalogStore) ListUserPresetsByProject(ctx context.Context, userID, projectID string) ([]*core.Preset, error) {
	rows, err := s.q.ListUserPresetsByProject(ctx, pgdb.ListUserPresetsByProjectParams{UserID: userID, ProjectID: workflowPtrToNullString(&projectID)})
	if err != nil {
		return nil, err
	}
	return presetsFromPG(rows), nil
}

func (s *workflowCatalogStore) ListPresetsByTag(ctx context.Context, userID, tag, projectID string) ([]*core.Preset, error) {
	rows, err := s.q.ListPresetsByTag(ctx, pgdb.ListPresetsByTagParams{UserID: userID, Tag: tag, ProjectID: workflowPtrToNullString(&projectID)})
	if err != nil {
		return nil, err
	}
	return presetsFromPG(rows), nil
}

func (s *workflowCatalogStore) UpdatePreset(ctx context.Context, preset *core.Preset) (*core.Preset, error) {
	params, err := presetToUpdateParams(preset)
	if err != nil {
		return nil, err
	}
	row, err := s.q.UpdatePreset(ctx, params)
	if err != nil {
		return nil, err
	}
	return presetFromPG(row), nil
}

func (s *workflowCatalogStore) DeletePreset(ctx context.Context, id string) error {
	return s.q.DeletePreset(ctx, id)
}

func (s *workflowCatalogStore) DeletePresetBySlug(ctx context.Context, userID, slug string) error {
	return s.q.DeletePresetBySlug(ctx, pgdb.DeletePresetBySlugParams{UserID: userID, Slug: slug})
}

func (s *workflowCatalogStore) DeletePresetBySlugAndProject(ctx context.Context, userID, slug, projectID string) error {
	return s.q.DeletePresetBySlugAndProject(ctx, pgdb.DeletePresetBySlugAndProjectParams{UserID: userID, Slug: slug, ProjectID: workflowPtrToNullString(&projectID)})
}

func (s *workflowCatalogStore) CreateWorkflowScenario(ctx context.Context, scenario *core.WorkflowScenario) error {
	return store.InsertWorkflowScenario(ctx, s.db, scenario)
}

func (s *workflowCatalogStore) GetWorkflowScenario(ctx context.Context, id string) (*core.WorkflowScenario, error) {
	return store.FindWorkflowScenario(ctx, s.db, id)
}

func (s *workflowCatalogStore) GetWorkflowScenarioByName(ctx context.Context, workflowDraftID string, name string) (*core.WorkflowScenario, error) {
	return store.FindWorkflowScenarioByName(ctx, s.db, workflowDraftID, name)
}

func (s *workflowCatalogStore) ListWorkflowScenariosByDraft(ctx context.Context, workflowDraftID string) ([]*core.WorkflowScenario, error) {
	return store.ListDraftWorkflowScenarios(ctx, s.db, workflowDraftID)
}

func (s *workflowCatalogStore) ListWorkflowScenariosByUser(ctx context.Context, userID string) ([]*core.WorkflowScenario, error) {
	return store.ListUserWorkflowScenarios(ctx, s.db, userID)
}

func (s *workflowCatalogStore) UpdateWorkflowScenario(ctx context.Context, scenario *core.WorkflowScenario) error {
	return store.ReplaceWorkflowScenario(ctx, s.db, scenario)
}

func (s *workflowCatalogStore) UpdateWorkflowScenarioResult(ctx context.Context, id string, status string, result string) error {
	return store.RecordWorkflowScenarioResult(ctx, s.db, id, status, result)
}

func (s *workflowCatalogStore) DeleteWorkflowScenario(ctx context.Context, id string) error {
	return store.RemoveWorkflowScenario(ctx, s.db, id)
}

func (s *workflowCatalogStore) DeleteWorkflowScenariosByDraft(ctx context.Context, workflowDraftID string) error {
	return store.RemoveDraftWorkflowScenarios(ctx, s.db, workflowDraftID)
}

func presetFromPG(sp pgdb.Preset) *core.Preset {
	params := make(map[string]interface{})
	if sp.Params != "" {
		_ = json.Unmarshal([]byte(sp.Params), &params)
	}

	return &core.Preset{
		ID:          sp.ID,
		UserID:      sp.UserID,
		ProjectID:   workflowNullStringToPtr(sp.ProjectID),
		Name:        sp.Name,
		Slug:        sp.Slug,
		Description: workflowNullStringToPtr(sp.Description),
		Tag:         sp.Tag,
		Params:      params,
		CreatedAt:   sp.CreatedAt,
		UpdatedAt:   sp.UpdatedAt,
	}
}

func presetsFromPG(rows []pgdb.Preset) []*core.Preset {
	presets := make([]*core.Preset, len(rows))
	for i, row := range rows {
		presets[i] = presetFromPG(row)
	}
	return presets
}

func presetToCreateParams(preset *core.Preset) (pgdb.CreatePresetParams, error) {
	paramsJSON, err := json.Marshal(preset.Params)
	if err != nil {
		return pgdb.CreatePresetParams{}, fmt.Errorf("failed to marshal preset params: %w", err)
	}
	return pgdb.CreatePresetParams{
		ID:          preset.ID,
		UserID:      preset.UserID,
		ProjectID:   workflowPtrToNullString(preset.ProjectID),
		Name:        preset.Name,
		Slug:        preset.Slug,
		Description: workflowPtrToNullString(preset.Description),
		Tag:         preset.Tag,
		Params:      string(paramsJSON),
	}, nil
}

func presetToUpsertParams(preset *core.Preset) (pgdb.UpsertPresetParams, error) {
	paramsJSON, err := json.Marshal(preset.Params)
	if err != nil {
		return pgdb.UpsertPresetParams{}, fmt.Errorf("failed to marshal preset params: %w", err)
	}
	return pgdb.UpsertPresetParams{
		ID:          preset.ID,
		UserID:      preset.UserID,
		ProjectID:   workflowPtrToNullString(preset.ProjectID),
		Name:        preset.Name,
		Slug:        preset.Slug,
		Description: workflowPtrToNullString(preset.Description),
		Tag:         preset.Tag,
		Params:      string(paramsJSON),
	}, nil
}

func presetToUpdateParams(preset *core.Preset) (pgdb.UpdatePresetParams, error) {
	paramsJSON, err := json.Marshal(preset.Params)
	if err != nil {
		return pgdb.UpdatePresetParams{}, fmt.Errorf("failed to marshal preset params: %w", err)
	}
	return pgdb.UpdatePresetParams{
		Name:        preset.Name,
		Description: workflowPtrToNullString(preset.Description),
		Tag:         preset.Tag,
		Params:      string(paramsJSON),
		ID:          preset.ID,
	}, nil
}

func workflowPtrToNullString(s *string) sql.NullString {
	if s != nil {
		return sql.NullString{String: *s, Valid: true}
	}
	return sql.NullString{Valid: false}
}

func workflowNullStringToPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}
