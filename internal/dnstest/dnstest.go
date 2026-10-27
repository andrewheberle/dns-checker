// Package dnstest provides a local DNS server for tests.
package dnstest

import (
	"net"
	"net/netip"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// Reply describes one response the server sends to a query.
type Reply struct {
	// WrongID sends the reply with an ID that does not match the query.
	WrongID   bool
	RCode     dnsmessage.RCode
	Truncated bool
	Answers   []dnsmessage.Resource
}

// Handler decides how the server responds to a query. Each Reply is sent in
// order, and returning none drops the query.
type Handler func(q dnsmessage.Question) []Reply

// Answer replies successfully, answering A questions with 192.0.2.1.
func Answer(q dnsmessage.Question) []Reply {
	r := Reply{RCode: dnsmessage.RCodeSuccess}
	if q.Type == dnsmessage.TypeA {
		r.Answers = []dnsmessage.Resource{A(q.Name.String(), "192.0.2.1")}
	}

	return []Reply{r}
}

// Drop never replies.
func Drop(dnsmessage.Question) []Reply {
	return nil
}

// RCode returns a Handler that replies with rcode and no answers.
func RCode(rcode dnsmessage.RCode) Handler {
	return func(dnsmessage.Question) []Reply {
		return []Reply{{RCode: rcode}}
	}
}

// A returns an A record for name.
func A(name, addr string) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: header(name),
		Body:   &dnsmessage.AResource{A: netip.MustParseAddr(addr).As4()},
	}
}

// AAAA returns an AAAA record for name.
func AAAA(name, addr string) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: header(name),
		Body:   &dnsmessage.AAAAResource{AAAA: netip.MustParseAddr(addr).As16()},
	}
}

// CNAME returns a CNAME record pointing name at target.
func CNAME(name, target string) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: header(name),
		Body:   &dnsmessage.CNAMEResource{CNAME: dnsmessage.MustNewName(target)},
	}
}

func header(name string) dnsmessage.ResourceHeader {
	return dnsmessage.ResourceHeader{
		Name:  dnsmessage.MustNewName(name),
		Class: dnsmessage.ClassINET,
		TTL:   60,
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

			for _, resp := range respond(buf[:n], h) {
				_, _ = conn.WriteTo(resp, addr)
			}
		}
	}()

	return conn.LocalAddr().String()
}

// respond builds the replies to the query in msg.
func respond(msg []byte, h Handler) [][]byte {
	var p dnsmessage.Parser
	hdr, err := p.Start(msg)
	if err != nil {
		return nil
	}

	q, err := p.Question()
	if err != nil {
		return nil
	}

	var resps [][]byte
	for _, r := range h(q) {
		m := dnsmessage.Message{
			Header: dnsmessage.Header{
				ID:                 hdr.ID,
				Response:           true,
				Authoritative:      true,
				Truncated:          r.Truncated,
				RecursionDesired:   hdr.RecursionDesired,
				RecursionAvailable: true,
				RCode:              r.RCode,
			},
			Questions: []dnsmessage.Question{q},
			Answers:   r.Answers,
		}
		if r.WrongID {
			m.ID = ^hdr.ID
		}

		resp, err := m.Pack()
		if err != nil {
			continue
		}
		resps = append(resps, resp)
	}

	return resps
}
