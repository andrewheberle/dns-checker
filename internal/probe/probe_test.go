package probe

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const testLookup = "probe.test."

// handler decides how the test server responds to a query. Returning false
// drops the query without a reply.
type handler func(q dnsmessage.Question) (rcode dnsmessage.RCode, reply bool)

// startServer runs a UDP DNS server on a random local port and returns its
// address.
func startServer(t *testing.T, h handler) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}

			resp, ok := respond(buf[:n], h)
			if !ok {
				continue
			}

			_, _ = conn.WriteTo(resp, addr)
		}
	}()

	return conn.LocalAddr().String()
}

// respond builds a reply to the query in msg, answering A questions with
// 192.0.2.1 when the handler returns RCodeSuccess.
func respond(msg []byte, h handler) ([]byte, bool) {
	var p dnsmessage.Parser
	hdr, err := p.Start(msg)
	if err != nil {
		return nil, false
	}

	q, err := p.Question()
	if err != nil {
		return nil, false
	}

	rcode, reply := h(q)
	if !reply {
		return nil, false
	}

	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
		ID:                 hdr.ID,
		Response:           true,
		Authoritative:      true,
		RecursionDesired:   hdr.RecursionDesired,
		RecursionAvailable: true,
		RCode:              rcode,
	})
	b.EnableCompression()

	if err := b.StartQuestions(); err != nil {
		return nil, false
	}
	if err := b.Question(q); err != nil {
		return nil, false
	}
	if err := b.StartAnswers(); err != nil {
		return nil, false
	}

	if rcode == dnsmessage.RCodeSuccess && q.Type == dnsmessage.TypeA {
		err := b.AResource(dnsmessage.ResourceHeader{
			Name:  q.Name,
			Class: dnsmessage.ClassINET,
			TTL:   60,
		}, dnsmessage.AResource{A: [4]byte{192, 0, 2, 1}})
		if err != nil {
			return nil, false
		}
	}

	resp, err := b.Finish()
	if err != nil {
		return nil, false
	}

	return resp, true
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		handler handler
		wantErr bool
	}{
		{
			name: "answer",
			handler: func(dnsmessage.Question) (dnsmessage.RCode, bool) {
				return dnsmessage.RCodeSuccess, true
			},
		},
		{
			name: "servfail",
			handler: func(dnsmessage.Question) (dnsmessage.RCode, bool) {
				return dnsmessage.RCodeServerFailure, true
			},
			wantErr: true,
		},
		{
			name: "nxdomain",
			handler: func(dnsmessage.Question) (dnsmessage.RCode, bool) {
				return dnsmessage.RCodeNameError, true
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := startServer(t, tt.handler)
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
	server := startServer(t, func(dnsmessage.Question) (dnsmessage.RCode, bool) {
		return 0, false
	})
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
	server := startServer(t, func(dnsmessage.Question) (dnsmessage.RCode, bool) {
		return dnsmessage.RCodeSuccess, true
	})
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
