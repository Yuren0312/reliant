# Approval-backed activation management

Agents can load `create_trigger_activation`, `update_trigger_activation`,
`enable_trigger`, and `pause_trigger` in an interactive orchestrator-tier chat.
Creation names a workflow and one declared trigger, a project, an explicit
`machine_id` (daemon ID) or `no_machine`, typed `params`, and a connection.
Updates preserve omitted fields; a supplied params map replaces the map.
Enable and pause accept only `trigger_id`.

The worker's existing tool factory and TriggerService perform the work. A
preview activity uses the app's definition/setup/match/access validators and
Enable dialog reachability check. The workflow uses the existing
ApprovalCreate/signal/ApprovalResolve flow, not an agent-authored question.
The card displays the resolved definition, machine ID/name, acting user and
identity, declaration, graph/tools/permissions, and previous definition. The
executor requires the runtime's call-scoped approval fingerprint and validates
it again. Changed workflow, connection selection or activation revision requires
a new approval. Declining, timing out or interrupting never grants authority.

Unattended runs are refused even if their tools explicitly name these tools;
no approval card is raised. There is **no delegation setting in this phase**.
The CLI's agent-shell refusal stays unchanged. A later delegation policy needs
an owner decision; this PR does not let triggered runs grant themselves standing
privileges.

Configuration mutations are recorded in `trigger_changes` in the same DB
transaction as the trigger row, with user, originating chat/thread/tool call,
and before/after definitions (no credential bytes). GetTrigger exposes the
newest 100 entries in Configuration history, separately from firing history
and health. App and CLI changes are audited too; their chat/run fields are empty.

Roll out API and worker together. New tools become discoverable through
load_tool on the next turn of a run whose tool filter/tier permits them. The
workflow approval wait survives worker restarts through Temporal history and
the existing durable approval rows.
