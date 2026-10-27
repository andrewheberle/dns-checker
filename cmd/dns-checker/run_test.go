package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewheberle/dns-checker/internal/dnstest"
	"golang.org/x/net/dns/dnsmessage"
)

const testLookup = "probe.test."

// syncBuffer is a bytes.Buffer that is safe to write from the HTTP server
// while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// freeAddr returns a local address with a port that was free when checked.
func freeAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	return addr
}

// startRun calls run in the background and waits until it is listening on
// addr. The returned cancel stops run and waits for it to return nil.
func startRun(t *testing.T, addr string, args ...string) (stderr *syncBuffer, cancel func()) {
	t.Helper()

	stderr = &syncBuffer{}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, args, io.Discard, stderr)
	}()

	var once sync.Once
	cancel = func() {
		once.Do(func() {
			stop()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("run() error = %v, want nil", err)
				}
			case <-time.After(10 * time.Second):
				t.Error("run() did not return after cancel")
			}
		})
	}
	t.Cleanup(cancel)

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return stderr, cancel
		}

		select {
		case err := <-done:
			t.Fatalf("run() returned before listening: %v", err)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatalf("run() not listening on %s: %v", addr, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// healthz returns the status code from the health check at addr.
func healthz(t *testing.T, addr string) int {
	t.Helper()

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"--version"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	if got, want := stdout.String(), "dns-checker dev\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunHelp(t *testing.T) {
	if err := run(context.Background(), []string{"--help"}, io.Discard, io.Discard); err != nil {
		t.Errorf("run() error = %v, want nil", err)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	if err := run(context.Background(), []string{"--nope"}, io.Discard, io.Discard); err == nil {
		t.Error("run() error = nil, want error")
	}
}

func TestRunHealthz(t *testing.T) {
	tests := []struct {
		name    string
		handler dnstest.Handler
		want    int
	}{
		{"answer", dnstest.Answer, http.StatusOK},
		{"servfail", dnstest.RCode(dnsmessage.RCodeServerFailure), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := dnstest.Start(t, tt.handler)
			addr := freeAddr(t)
			stderr, _ := startRun(t, addr, "--listen", addr, "--server", server, "--lookup", testLookup)

			if got := healthz(t, addr); got != tt.want {
				t.Errorf("GET /healthz = %d, want %d", got, tt.want)
			}

			if tt.want != http.StatusOK && !strings.Contains(stderr.String(), server) {
				t.Errorf("stderr = %q, want it to name %s", stderr.String(), server)
			}
		})
	}
}

func TestRunEnv(t *testing.T) {
	server := dnstest.Start(t, dnstest.Answer)
	addr := freeAddr(t)
	t.Setenv("DNS_LISTEN", addr)
	t.Setenv("DNS_SERVER", server)
	t.Setenv("DNS_LOOKUP", testLookup)

	startRun(t, addr)

	if got := healthz(t, addr); got != http.StatusOK {
		t.Errorf("GET /healthz = %d, want %d", got, http.StatusOK)
	}
}

func TestRunFlagOverridesEnv(t *testing.T) {
	bad := dnstest.Start(t, dnstest.RCode(dnsmessage.RCodeServerFailure))
	good := dnstest.Start(t, dnstest.Answer)
	addr := freeAddr(t)
	t.Setenv("DNS_SERVER", bad)

	startRun(t, addr, "--listen", addr, "--server", good, "--lookup", testLookup)

	if got := healthz(t, addr); got != http.StatusOK {
		t.Errorf("GET /healthz = %d, want %d", got, http.StatusOK)
	}
}

func TestRunShutdown(t *testing.T) {
	addr := freeAddr(t)
	_, cancel := startRun(t, addr, "--listen", addr)

	start := time.Now()
	cancel()

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("run() took %s to return after cancel", elapsed)
	}
}

func TestRunListenInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := run(ctx, []string{"--listen", ln.Addr().String()}, io.Discard, io.Discard); err == nil {
		t.Error("run() error = nil, want listen error")
	}
}

func TestRunInvalidServer(t *testing.T) {
	addr := freeAddr(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := run(ctx, []string{"--listen", addr, "--server", "10.0.0.1:abc"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "invalid target server address") {
		t.Errorf("run() error = %v, want invalid target server address error", err)
	}
}
