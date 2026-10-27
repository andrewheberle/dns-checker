// Package probe checks the health of a DNS server by resolving a hostname
// against it.
package probe

import (
	"context"
	"fmt"
	"net"
	"time"
)

const defaultTimeout = 4 * time.Second

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
// which must be in host:port form.
func New(server, lookup string, opts ...Option) *Probe {
	p := &Probe{
		server:  server,
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

	return p
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
