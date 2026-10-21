package postgres

import (
	"database/sql"

	"github.com/reliant-labs/reliant/internal/db/core"
)

// The helper the chat store used to own, kept for trigger_store.go, which is
// still on sqlc and reads a chat's root workflow status. The chats queries
// themselves are internal/store's (P5b L1b).

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
