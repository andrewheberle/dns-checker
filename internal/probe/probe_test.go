package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/andrewheberle/dns-checker/internal/dnstest"
	"golang.org/x/net/dns/dnsmessage"
)

const testLookup = "probe.test."

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		want    string
		wantErr bool
	}{
		{name: "ipv4 with port", server: "10.0.0.1:53", want: "10.0.0.1:53"},
		{name: "hostname with port", server: "dns.example:5353", want: "dns.example:5353"},
		{name: "ipv4 without port", server: "10.0.0.1", want: "10.0.0.1:53"},
		{name: "hostname without port", server: "coredns.kube-system.svc", want: "coredns.kube-system.svc:53"},
		{name: "bare ipv6", server: "::1", want: "[::1]:53"},
		{name: "bare ipv6 global", server: "fd00::10", want: "[fd00::10]:53"},
		{name: "bracketed ipv6 without port", server: "[::1]", want: "[::1]:53"},
		{name: "bracketed ipv6 with port", server: "[::1]:5353", want: "[::1]:5353"},
		{name: "empty", server: "", wantErr: true},
		{name: "empty host", server: ":53", wantErr: true},
		{name: "empty port", server: "10.0.0.1:", wantErr: true},
		{name: "non-numeric port", server: "10.0.0.1:abc", wantErr: true},
		{name: "zero port", server: "10.0.0.1:0", wantErr: true},
		{name: "port out of range", server: "10.0.0.1:70000", wantErr: true},
		{name: "named port", server: "10.0.0.1:domain", wantErr: true},
		{name: "unclosed bracket", server: "[::1", wantErr: true},
		{name: "bracketed hostname", server: "[dns.example]", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.server, testLookup)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New(%q) error = %v, wantErr %v", tt.server, err, tt.wantErr)
			}

			if err != nil {
				if !strings.Contains(err.Error(), fmt.Sprintf("%q", tt.server)) {
					t.Errorf("New(%q) error = %q, want it to name the address", tt.server, err)
				}
				return
			}

			// the default port can't be checked with a probe, as the test
			// DNS server can't listen on port 53
			if p.server != tt.want {
				t.Errorf("New(%q) server = %q, want %q", tt.server, p.server, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		handler dnstest.Handler
		wantErr bool
	}{
		{
			name:    "answer",
			handler: dnstest.Answer,
		},
		{
			name:    "servfail",
			handler: dnstest.RCode(dnsmessage.RCodeServerFailure),
			wantErr: true,
		},
		{
			name:    "nxdomain",
			handler: dnstest.RCode(dnsmessage.RCodeNameError),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := dnstest.Start(t, tt.handler)
			p, err := New(server, testLookup)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			err = p.Check(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				if !strings.Contains(err.Error(), server) || !strings.Contains(err.Error(), testLookup) {
					t.Errorf("Check() error = %q, want it to name %s and %s", err, testLookup, server)
				}
			}
		})
	}
}

func TestCheckTimeout(t *testing.T) {
	server := dnstest.Start(t, dnstest.Drop)
	p, err := New(server, testLookup, WithTimeout(200*time.Millisecond))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	start := time.Now()
	err = p.Check(context.Background())
	if err == nil {
		t.Fatal("Check() error = nil, want timeout error")
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Check() took %s, want it to stop near the 200ms timeout", elapsed)
	}
}

func TestCheckCancelled(t *testing.T) {
	server := dnstest.Start(t, dnstest.Answer)
	p, err := New(server, testLookup)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = p.Check(ctx)
	if err == nil {
		t.Fatal("Check() error = nil, want cancellation error")
	}

	var dnsErr *net.DNSError
	if !errors.As(err, &dnsErr) && !errors.Is(err, context.Canceled) {
		t.Errorf("Check() error = %v, want a DNS or cancellation error", err)
	}
}
