# Planned daemon drain (B2)

This is a transport primitive, not an update policy. Nothing invokes it
periodically or from a version check. Quiet-point selection (10 minutes without
requested activity, background processes excluded) and fleet rollout belong to
D1; policy plumbing belongs to C1.

## Manual control

`DaemonRegistryService.RequestDaemonRestart` requires an authenticated `admin`
role. Ordinary user JWTs, API keys and machine credentials cannot invoke it.
Supply `daemon_id`, a non-empty `reason`, and optionally `deadline_unix_ms`.
An omitted deadline uses ten minutes; a supplied deadline cannot lengthen that
bound. Retries return the same deadline and re-send an unacknowledged notice.
There is deliberately no UI control in B2.

The API routes the request over `daemon.restart.<owner>.<daemon>` to the gateway
holding the stream. This is an internal NATS command: it requires the same
trusted service-bus access as the existing daemon commands, not end-user NATS
access. The gateway's exported `RequestDaemonRestart` is also the test hook.

## Ordering and recovery

1. The gateway persists a restart observation, gates admission, unsubscribes
   and flushes new-work subjects using its existing make-before-break drain.
   Results, cancellation, backgrounding and the administrative retry subject
   stay live. A callback selected before unsubscribe is refused before queueing
   and returns `DAEMON_UNDELIVERED`, not an unknown-outcome interruption.
2. `RestartRequested` tells the daemon the reason and hard deadline. The daemon
   records `updating`, stops new admissions, and waits for running calls and
   held-result acknowledgements. Completions/acks advance the drain; there is
   one deadline timer, no polling or background-process census.
3. At the deadline a still-running tool gets B1's durable `DAEMON_RESTARTED`
   result, telling the model to verify effects before retrying. Cancellation
   follows recording that verdict; a late successful completion cannot replace
   it. Completed, unacknowledged results remain in the persistent outbox.
4. The daemon reports `drained` (and `timed_out` when applicable) and remains
   quiesced. Its supervisor/operator must replace it. Merely restarting a cloud
   container does not deliver a new image; B2 does not perform that replacement.
5. A gateway reconnect by the *same process* carries its restart observation in
   registration, so gateway hand-over cannot re-enable work. A new process has
   none: registration clears the observation and reconnect restores routing.

The observation lives on the daemon identity, not its disappearing attachment.
Writes are fenced to the owning attachment generation. The registry reports
`UPDATING` while attached and through the bounded replacement window when
unattached; it also serves the drain deadline and progress. Workers read that
persisted deadline when a request finds the machine unavailable, extending the
ordinary reconnect grace to drain deadline plus 90 seconds, still bounded by
caller cancellation/deadline. Undelivered requests can then be sent again safely.

Terminals and background processes do not hold the drain open. They may die
when the machine is replaced; B2 makes no promise to preserve them. Announcing
those effects in the rollout UI is later work.

## Deterministic tests

`daemonruntime/daemon_restart_test.go` owns the timer callback and drives
completion/ack events directly. Gateway tests use the real in-process NATS bus,
request/result channels, subscription flushes and reconnect announcements.
Worker tests start outside ordinary last-seen grace, coordinate reconnect from
the wait observer, and assert one execution. The DB test verifies retained
observations, owner scoping, and old-session fencing on an isolated database.
No new test sleeps to establish correctness.
