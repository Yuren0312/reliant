package tools_daemon

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/reliant-labs/forge/pkg/svcerr"
	pb "github.com/reliant-labs/reliant/gen/reliant/v1"
	"github.com/reliant-labs/reliant/internal/toolexec"
)

// RequestDaemonRestart is the manual/operator hook, with no policy selection.
// Repeated requests cannot postpone a running drain. Like handOver, the bridge
// unsubscribes and flushes before the daemon sees the notice. Unlike handOver,
// it does not mark the entire gateway replica as draining.
func (s *Service) RequestDaemonRestart(ctx context.Context, userID, daemonID string, req *pb.RestartRequested) (*pb.DaemonRestartStatus, error) {
	conn, err := s.connForUser(userID, daemonID)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Reason) == "" {
		return nil, svcerr.InvalidArgument("restart reason is required")
	}
	conn.admissionMu.Lock()
	defer conn.admissionMu.Unlock()
	if status := conn.restart.Load(); status != nil {
		if !status.Drained {
			err := s.sendControlToConn(conn, &pb.ServerMessage{Message: &pb.ServerMessage_RestartRequested{RestartRequested: &pb.RestartRequested{DeadlineUnixMs: status.DeadlineUnixMs, Reason: status.Reason}}})
			return status, err
		}
		return status, nil
	}
	deadline := time.Now().Add(toolexec.DaemonDrainTimeout)
	if req.DeadlineUnixMs > 0 {
		deadline = minTime(deadline, time.UnixMilli(req.DeadlineUnixMs))
	}
	status := &pb.DaemonRestartStatus{DeadlineUnixMs: deadline.UnixMilli(), Reason: req.Reason}
	if err := s.persistRestart(ctx, conn, status); err != nil {
		return nil, err
	}
	conn.restart.Store(status)
	s.notifyUpdating(userID, daemonID)
	notice := &pb.ServerMessage{Message: &pb.ServerMessage_RestartRequested{RestartRequested: &pb.RestartRequested{DeadlineUnixMs: status.DeadlineUnixMs, Reason: status.Reason}}}
	// New work is excluded under admissionMu; the notice itself is control traffic.
	if err := s.sendControlToConn(conn, notice); err != nil {
		return status, err
	}
	s.publishDaemonListChanged(userID, daemonID)
	return status, nil
}

// notifyUpdating leaves a gateway hand-over's administrative subscriptions
// distinct from a daemon drain's: the former must move all request routing.
func (s *Service) notifyUpdating(userID, daemonID string) {
	s.listenersMu.RLock()
	defer s.listenersMu.RUnlock()
	for _, listener := range s.listeners {
		if l, ok := listener.(interface{ OnDaemonUpdating(string, string) }); ok {
			l.OnDaemonUpdating(userID, daemonID)
		} else if l, ok := listener.(DaemonDrainListener); ok {
			l.OnDaemonDraining(userID, daemonID)
		}
	}
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func (s *Service) persistRestart(ctx context.Context, conn *daemonConnection, status *pb.DaemonRestartStatus) error {
	if s.deps.Repo == nil {
		return nil
	}
	data := ""
	if status != nil {
		b, err := json.Marshal(status)
		if err != nil {
			return err
		}
		data = string(b)
	}
	return s.deps.Repo.SetDaemonRestartStatus(ctx, conn.userID, conn.daemonID, conn.attachedAt, data)
}

// restoreRestart runs before announcing a registered stream. A reconnecting
// draining process carries its status; a new process carries none and clears
// the old observation, including while a previous stream tears down.
func (s *Service) restoreRestart(ctx context.Context, conn *daemonConnection, reg *pb.DaemonRegister) error {
	conn.admissionMu.Lock()
	defer conn.admissionMu.Unlock()
	status := reg.GetRestartStatus()
	// Registration has already published this connection's admission state.
	// A manual request racing the final registration writes may have advanced
	// it; never clear that request using the older registration snapshot.
	if current := conn.restart.Load(); current != nil {
		status = current
	}
	conn.restart.Store(status)
	return s.persistRestart(ctx, conn, status)
}

// recordRestart runs on the receive loop so a drained report is not lost to
// disconnect overtaking a queued state write. It is a rare bounded write.
func (s *Service) recordRestart(ctx context.Context, conn *daemonConnection, status *pb.DaemonRestartStatus) error {
	conn.admissionMu.Lock()
	defer conn.admissionMu.Unlock()
	prev := conn.restart.Load()
	if prev == nil || status == nil || status.DeadlineUnixMs != prev.DeadlineUnixMs {
		return nil
	}
	if prev.Drained && !status.Drained {
		status = prev // never regress, but re-persist after lease recreation
	}
	ctx, cancel := context.WithTimeout(ctx, inboundStateApplyTimeout)
	defer cancel()
	if err := s.persistRestart(ctx, conn, status); err != nil {
		return err
	}
	conn.restart.Store(status)
	s.publishDaemonListChanged(conn.userID, conn.daemonID)
	return nil
}

// requestNeedsAdmission excludes control traffic: results/acks, heartbeat,
// cancellation and tool.status must still cross a draining stream.
func requestNeedsAdmission(msg *pb.ServerMessage) bool {
	switch m := msg.GetMessage().(type) {
	case *pb.ServerMessage_ToolRequest, *pb.ServerMessage_LocalModelHttpRequest, *pb.ServerMessage_TerminalInput:
		return true
	case *pb.ServerMessage_DaemonCommand:
		return m.DaemonCommand.GetCommandType() != toolexec.ToolStatusCommand
	default:
		return false
	}
}
