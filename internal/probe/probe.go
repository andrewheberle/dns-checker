// Package probe checks the health of a DNS server by querying it directly
// for a hostname.
package probe

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	defaultTimeout = 4 * time.Second
	defaultPort    = "53"

	// resendInterval is how often an unanswered query is sent again
	resendInterval = time.Second
)

// Probe performs a DNS health check against a single target server.
type Probe struct {
	server  string
	lookup  string
	timeout time.Duration
}

// Option configures a Probe.
type Option func(*Probe)

// WithTimeout sets the time allowed for a whole check. The default is 4 seconds.
func WithTimeout(d time.Duration) Option {
	return func(p *Probe) {
		p.timeout = d
	}
}

// New returns a Probe that queries the DNS server at server for lookup,
// which is always treated as fully qualified. The server is given as
// host:port. The port defaults to 53 if omitted, and a bare IPv6
// address may be given with or without brackets. An error is returned if
// server is not a valid address.
func New(server, lookup string, opts ...Option) (*Probe, error) {
	addr, err := normaliseServer(server)
	if err != nil {
		return nil, fmt.Errorf("invalid target server address %q: %w", server, err)
	}

	p := &Probe{
		server:  addr,
		lookup:  fqdn(lookup),
		timeout: defaultTimeout,
	}

	for _, o := range opts {
		o(p)
	}

	return p, nil
}

// fqdn returns name with a trailing dot, so it is never expanded with search
// domains.
func fqdn(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}

	return name + "."
}

// normaliseServer returns server in host:port form, adding the default port
// if none is given.
func normaliseServer(server string) (string, error) {
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		// no port, so server is a hostname or an IP address, with IPv6
		// addresses optionally in brackets
		host = server
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
			if _, perr := netip.ParseAddr(host); perr != nil {
				return "", err
			}
		} else if strings.Contains(host, ":") {
			if _, perr := netip.ParseAddr(host); perr != nil {
				return "", err
			}
		}
		port = defaultPort
	}

	if host == "" {
		return "", errors.New("missing host")
	}

	if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
		return "", fmt.Errorf("port %q must be a number from 1 to 65535", port)
	}

	return net.JoinHostPort(host, port), nil
}

// Check queries the target server for the lookup host's A records, then its
// AAAA records if there are no A records. It returns an error unless a
// NOERROR reply answers the lookup host with an address or a CNAME within the
// timeout. CNAMEs are not followed, as answering with one shows the target
// server is working.
func (p *Probe) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	answered, err := p.query(ctx, dnsmessage.TypeA)
	if err == nil && !answered {
		answered, err = p.query(ctx, dnsmessage.TypeAAAA)
	}
	if err != nil {
		return fmt.Errorf("probe %s via %s: %w", p.lookup, p.server, err)
	}
	if !answered {
		return fmt.Errorf("probe %s via %s: no A, AAAA or CNAME records", p.lookup, p.server)
	}

	return nil
}

// query sends a question of type t for the lookup host over UDP, resending
// it until a reply arrives or ctx is done. It reports whether the reply
// answered the lookup host with a type t record or a CNAME. Replies with a
// different ID or question are ignored.
func (p *Probe) query(ctx context.Context, t dnsmessage.Type) (bool, error) {
	name, err := dnsmessage.NewName(p.lookup)
	if err != nil {
		return false, fmt.Errorf("invalid lookup host: %w", err)
	}

	q := dnsmessage.Question{Name: name, Type: t, Class: dnsmessage.ClassINET}
	id := uint16(rand.Uint32())
	msg := dnsmessage.Message{
		Header:    dnsmessage.Header{ID: id, RecursionDesired: true},
		Questions: []dnsmessage.Question{q},
	}
	b, err := msg.Pack()
	if err != nil {
		return false, fmt.Errorf("building query: %w", err)
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", p.server)
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close() }()

	// unblock reads once ctx is done
	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stop()

	if _, err := conn.Write(b); err != nil {
		return false, err
	}

	// resend until a reply arrives, in case a packet is lost
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(resendInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = conn.Write(b)
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}

			return false, err
		}

		answered, ok, err := parseReply(buf[:n], id, q)
		if !ok {
			continue
		}

		return answered, err
	}
}

// parseReply checks a reply to the query with the given id and question. ok
// is false if msg is not a reply to that query. Otherwise answered reports
// whether it answers the question's name with a record of the question's
// type or a CNAME, and err is set if the server returned an error.
func parseReply(msg []byte, id uint16, q dnsmessage.Question) (answered, ok bool, err error) {
	var pr dnsmessage.Parser
	h, err := pr.Start(msg)
	if err != nil || h.ID != id || !h.Response {
		return false, false, nil
	}

	// some servers leave out the question when returning an error
	rq, err := pr.Question()
	switch {
	case errors.Is(err, dnsmessage.ErrSectionDone):
	case err != nil:
		return false, false, nil
	case rq.Type != q.Type || rq.Class != q.Class || !strings.EqualFold(rq.Name.String(), q.Name.String()):
		return false, false, nil
	}

	if h.RCode != dnsmessage.RCodeSuccess {
		return false, true, fmt.Errorf("server returned %s", rcodeName(h.RCode))
	}

	// the server answered, even if the answers did not fit
	if h.Truncated {
		return true, true, nil
	}

	if err := pr.SkipAllQuestions(); err != nil {
		return false, true, fmt.Errorf("parsing reply: %w", err)
	}

	for {
		ah, err := pr.AnswerHeader()
		if errors.Is(err, dnsmessage.ErrSectionDone) {
			return false, true, nil
		}
		if err != nil {
			return false, true, fmt.Errorf("parsing reply: %w", err)
		}

		if (ah.Type == q.Type || ah.Type == dnsmessage.TypeCNAME) && strings.EqualFold(ah.Name.String(), q.Name.String()) {
			return true, true, nil
		}

		if err := pr.SkipAnswer(); err != nil {
			return false, true, fmt.Errorf("parsing reply: %w", err)
		}
	}
}

// rcodeName returns the conventional name for a DNS response code.
func rcodeName(rcode dnsmessage.RCode) string {
	switch rcode {
	case dnsmessage.RCodeFormatError:
		return "FORMERR"
	case dnsmessage.RCodeServerFailure:
		return "SERVFAIL"
	case dnsmessage.RCodeNameError:
		return "NXDOMAIN"
	case dnsmessage.RCodeNotImplemented:
		return "NOTIMP"
	case dnsmessage.RCodeRefused:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE %d", rcode)
	}
}
