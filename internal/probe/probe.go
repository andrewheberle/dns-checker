// Package probe checks the health of a DNS server by resolving a hostname
// against it.
package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 4 * time.Second
	defaultPort    = "53"
)

// Probe performs a DNS health check against a single target server.
type Probe struct {
	server   string
	lookup   string
	timeout  time.Duration
	resolver *net.Resolver
}

// Option configures a Probe.
type Option func(*Probe)

// WithTimeout sets the time allowed for a whole check. The default is 4 seconds.
func WithTimeout(d time.Duration) Option {
	return func(p *Probe) {
		p.timeout = d
	}
}

// New returns a Probe that resolves lookup using the DNS server at server,
// given as host:port. The port defaults to 53 if omitted, and a bare IPv6
// address may be given with or without brackets. An error is returned if
// server is not a valid address.
func New(server, lookup string, opts ...Option) (*Probe, error) {
	addr, err := normaliseServer(server)
	if err != nil {
		return nil, fmt.Errorf("invalid target server address %q: %w", server, err)
	}

	p := &Probe{
		server:  addr,
		lookup:  lookup,
		timeout: defaultTimeout,
	}

	for _, o := range opts {
		o(p)
	}

	// custom resolver to send all queries to the target server
	p.resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer

			return d.DialContext(ctx, network, p.server)
		},
	}

	return p, nil
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

// Check resolves the lookup host against the target server, returning an
// error if the lookup fails or does not complete within the timeout.
func (p *Probe) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	if _, err := p.resolver.LookupHost(ctx, p.lookup); err != nil {
		return fmt.Errorf("probe %s via %s: %w", p.lookup, p.server, err)
	}

	return nil
}
