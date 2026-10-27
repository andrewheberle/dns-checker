package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
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

// answers returns a Handler that replies to every query with records built
// for the question.
func answers(records func(q dnsmessage.Question) []dnsmessage.Resource) dnstest.Handler {
	return func(q dnsmessage.Question) []dnstest.Reply {
		return []dnstest.Reply{{Answers: records(q)}}
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		lookup  string
		handler dnstest.Handler
		wantErr string
	}{
		{
			name:    "answer",
			handler: dnstest.Answer,
		},
		{
			name: "aaaa only",
			handler: answers(func(q dnsmessage.Question) []dnsmessage.Resource {
				if q.Type == dnsmessage.TypeAAAA {
					return []dnsmessage.Resource{dnstest.AAAA(q.Name.String(), "2001:db8::1")}
				}
				return nil
			}),
		},
		{
			name: "no records",
			handler: answers(func(dnsmessage.Question) []dnsmessage.Resource {
				return nil
			}),
			wantErr: "no A, AAAA or CNAME records",
		},
		{
			name: "cname and a",
			handler: answers(func(q dnsmessage.Question) []dnsmessage.Resource {
				return []dnsmessage.Resource{
					dnstest.CNAME(q.Name.String(), "target.test."),
					dnstest.A("target.test.", "192.0.2.1"),
				}
			}),
		},
		{
			name: "cname only",
			handler: answers(func(q dnsmessage.Question) []dnsmessage.Resource {
				return []dnsmessage.Resource{dnstest.CNAME(q.Name.String(), "target.test.")}
			}),
		},
		{
			name: "unrelated records only",
			handler: answers(func(dnsmessage.Question) []dnsmessage.Resource {
				return []dnsmessage.Resource{dnstest.A("other.test.", "192.0.2.1")}
			}),
			wantErr: "no A, AAAA or CNAME records",
		},
		{
			name: "answer name in different case",
			handler: answers(func(q dnsmessage.Question) []dnsmessage.Resource {
				return []dnsmessage.Resource{dnstest.A(strings.ToUpper(q.Name.String()), "192.0.2.1")}
			}),
		},
		{
			name:    "lookup without trailing dot",
			lookup:  "probe.test",
			handler: dnstest.Answer,
		},
		{
			name:    "hosts file name is still queried",
			lookup:  "localhost",
			handler: dnstest.RCode(dnsmessage.RCodeServerFailure),
			wantErr: "SERVFAIL",
		},
		{
			name: "wrong id ignored",
			handler: func(q dnsmessage.Question) []dnstest.Reply {
				return []dnstest.Reply{
					{WrongID: true, RCode: dnsmessage.RCodeNameError},
					{Answers: []dnsmessage.Resource{dnstest.A(q.Name.String(), "192.0.2.1")}},
				}
			},
		},
		{
			name: "truncated",
			handler: func(dnsmessage.Question) []dnstest.Reply {
				return []dnstest.Reply{{Truncated: true}}
			},
		},
		{
			name:    "servfail",
			handler: dnstest.RCode(dnsmessage.RCodeServerFailure),
			wantErr: "SERVFAIL",
		},
		{
			name:    "nxdomain",
			handler: dnstest.RCode(dnsmessage.RCodeNameError),
			wantErr: "NXDOMAIN",
		},
		{
			name:    "refused",
			handler: dnstest.RCode(dnsmessage.RCodeRefused),
			wantErr: "REFUSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := tt.lookup
			if lookup == "" {
				lookup = testLookup
			}

			server := dnstest.Start(t, tt.handler)
			p, err := New(server, lookup, WithTimeout(500*time.Millisecond))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			err = p.Check(context.Background())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Check() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Check() error = nil, want %q", tt.wantErr)
			}

			for _, want := range []string{tt.wantErr, server, fqdn(lookup)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Check() error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

func TestCheckResend(t *testing.T) {
	var queries atomic.Int32
	server := dnstest.Start(t, func(q dnsmessage.Question) []dnstest.Reply {
		// drop the first query, as if the packet was lost
		if queries.Add(1) == 1 {
			return nil
		}

		return dnstest.Answer(q)
	})

	p, err := New(server, testLookup)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := p.Check(context.Background()); err != nil {
		t.Errorf("Check() error = %v, want nil", err)
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
