// Package dnstest provides a local DNS server for tests.
package dnstest

import (
	"net"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// Handler decides how the server responds to a query. Returning false drops
// the query without a reply.
type Handler func(q dnsmessage.Question) (rcode dnsmessage.RCode, reply bool)

// Answer replies successfully, answering A questions with 192.0.2.1.
func Answer(dnsmessage.Question) (dnsmessage.RCode, bool) {
	return dnsmessage.RCodeSuccess, true
}

// Drop never replies.
func Drop(dnsmessage.Question) (dnsmessage.RCode, bool) {
	return 0, false
}

// RCode returns a Handler that replies with rcode and no answers.
func RCode(rcode dnsmessage.RCode) Handler {
	return func(dnsmessage.Question) (dnsmessage.RCode, bool) {
		return rcode, true
	}
}

// Start runs a UDP DNS server on a random local port until the test ends and
// returns its address.
func Start(t testing.TB, h Handler) string {
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
func respond(msg []byte, h Handler) ([]byte, bool) {
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
