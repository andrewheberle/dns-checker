package probe

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/andrewheberle/dns-checker/internal/dnstest"
	"golang.org/x/net/dns/dnsmessage"
)

const testLookup = "probe.test."

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
			p := New(server, testLookup)

			err := p.Check(context.Background())
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
	p := New(server, testLookup, WithTimeout(200*time.Millisecond))

	start := time.Now()
	err := p.Check(context.Background())
	if err == nil {
		t.Fatal("Check() error = nil, want timeout error")
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Check() took %s, want it to stop near the 200ms timeout", elapsed)
	}
}

func TestCheckCancelled(t *testing.T) {
	server := dnstest.Start(t, dnstest.Answer)
	p := New(server, testLookup)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Check(ctx)
	if err == nil {
		t.Fatal("Check() error = nil, want cancellation error")
	}

	var dnsErr *net.DNSError
	if !errors.As(err, &dnsErr) && !errors.Is(err, context.Canceled) {
		t.Errorf("Check() error = %v, want a DNS or cancellation error", err)
	}
}
