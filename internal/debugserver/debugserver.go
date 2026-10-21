// Copyright (c) 2025 Reliant Labs

// Package debugserver is the localhost-only diagnostics listener every reliant
// server process runs: pprof, DB write-queue gauges, and Prometheus metrics.
//
// It binds 127.0.0.1 and nothing else. Nothing here is authenticated, and a
// goroutine dump carries prompts and request bodies, so it must never share a
// port with a public listener or bind a routable address. Reaching it on a
// cluster means `kubectl port-forward`, which is exactly the access it should
// require.
package debugserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: registered on DefaultServeMux, served only on 127.0.0.1 below
	"time"

	"github.com/reliant-labs/reliant/internal/db"
	"github.com/reliant-labs/reliant/internal/logging"
	"github.com/reliant-labs/reliant/internal/observability"
)

// Start serves the diagnostics endpoints on 127.0.0.1:port in the background
// (New, run until the process exits).
func Start(port int) {
	if s := New(port); s != nil {
		go func() { _ = s.Run(context.Background()) }()
	}
}

// Server is the diagnostics listener as a supervised component: Run serves
// until its context ends, Close stops it.
type Server struct {
	addr string
	srv  *http.Server
}

// New is the diagnostics listener on 127.0.0.1:port. Port 0 (PPROF_PORT=0)
// serves nothing, and New returns nil: the listener is off, not on a random
// port.
func New(port int) *Server {
	if port == 0 {
		logging.Info("Debug server (pprof) disabled", "pprof_port", 0)
		return nil
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	return &Server{addr: addr, srv: &http.Server{Addr: addr, Handler: handler(), ReadHeaderTimeout: 10 * time.Second}}
}

// Run serves until ctx ends. A bind failure is logged, not returned: a
// process without a profiler still does its job, and failing it over one
// would be the wrong trade.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		logging.Error("debug server failed", "address", s.addr, "error", err)
		<-ctx.Done()
		return nil
	}
	logging.Info("Starting debug server (pprof)", "address", s.addr)
	stop := context.AfterFunc(ctx, func() { _ = s.srv.Close() })
	defer stop()
	if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logging.Error("debug server failed", "address", s.addr, "error", err)
	}
	return nil
}

// Close stops the listener. A profile in flight is cut off: nothing here is
// worth holding the process's stop for.
func (s *Server) Close(context.Context) error { return s.srv.Close() }

// handler is the diagnostics endpoints.
func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	mux.HandleFunc("/debug/db", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"pending_writes": %d, "peak_pending_writes": %d}`,
			db.GetPendingWrites(), db.GetPeakPendingWrites())
	})
	mux.HandleFunc("/debug/db/reset-peak", func(w http.ResponseWriter, r *http.Request) {
		db.ResetPeakPendingWrites()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status": "ok", "message": "peak reset"}`)
	})
	mux.Handle("/metrics", observability.MetricsHandler())
	return mux
}
