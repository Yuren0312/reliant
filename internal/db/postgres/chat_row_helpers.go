package postgres

import (
	"database/sql"
	"time"

	"github.com/reliant-labs/reliant/internal/db/core"
)

// Helpers the chat store used to own, kept for the stores still on sqlc that
// read chat columns (run_store.go, trigger_store.go). The chats queries
// themselves are internal/store's (P5b L1b).

func chatPtrToNullString(s *string) sql.NullString {
	if s != nil {
		return sql.NullString{String: *s, Valid: true}
	}
	return sql.NullString{Valid: false}
}

func chatNullTimeToPtr(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

// chatRootStatus maps a root_workflow_state / stop_reason pair (the
// chats_with_activity columns, or a join on the root workflow) onto a
// WorkflowStatus. Both are NULL together when the chat has no root workflow
// row, which is the zero status — WORKFLOW_STATE_UNSPECIFIED — and not any of
// the real lifecycle states.
func chatRootStatus(state, stopReason sql.NullInt32) core.WorkflowStatus {
	if !state.Valid {
		return core.WorkflowStatus{}
	}
	return core.WorkflowStatus{
		State:      core.WorkflowState(state.Int32),
		StopReason: core.WorkflowStopReason(stopReason.Int32),
	}
}
