// Copyright (c) 2025 Reliant Labs
package debugserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestStartServesPprofOnLoopbackOnly: the endpoints answer on 127.0.0.1, and
// the port is not bound on any other address — a goroutine dump carries
// prompts and request bodies, so it must never be reachable off the machine.
func TestStartServesPprofOnLoopbackOnly(t *testing.T) {
	port := freePort(t)
	Start(port)

	url := fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/goroutine?debug=1", port)
	var res *http.Response
	var err error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if res, err = http.Get(url); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("debug server never answered on loopback: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", url, res.StatusCode)
	}

	// Dialing the port on a non-loopback address of this machine must be
	// refused: that is what a listener on 0.0.0.0 would accept.
	ip := nonLoopbackIP(t)
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, fmt.Sprint(port)), time.Second); err == nil {
		_ = conn.Close()
		t.Fatalf("debug server accepted a connection on %s: it must bind loopback only", ip)
	}
}

func nonLoopbackIP(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			return n.IP.String()
		}
	}
	t.Skip("no non-loopback IPv4 address to probe")
	return ""
}

// As a supervised node, the listener serves until its context ends and is
// gone after; PPROF_PORT=0 is no listener at all.
func TestServerRunsUntilItsContextEnds(t *testing.T) {
	if New(0) != nil {
		t.Fatal("PPROF_PORT=0 built a listener")
	}
	port := freePort(t)
	s := New(port)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	url := fmt.Sprintf("http://127.0.0.1:%d/debug/db", port)
	var err error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var res *http.Response
		if res, err = http.Get(url); err == nil {
			_ = res.Body.Close()
			break
		}
	}
	if err != nil {
		t.Fatalf("debug server never answered: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context ended")
	}
	if res, err := http.Get(url); err == nil {
		_ = res.Body.Close()
		t.Fatal("the listener still answers after Run returned")
	}
}
