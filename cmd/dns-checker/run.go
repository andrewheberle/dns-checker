package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/andrewheberle/configger"
	"github.com/andrewheberle/dns-checker/internal/probe"
	"github.com/spf13/pflag"
)

// run parses args and DNS_* environment variables, then serves the health
// check until ctx is cancelled and the HTTP server has shut down.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	logger := slog.New(slog.NewTextHandler(stderr, nil))

	// command line flags
	f := pflag.NewFlagSet("dns-checker", pflag.ContinueOnError)
	f.SetOutput(stderr)
	f.String("listen", "localhost:8080", "Service listen address")
	f.String("server", "localhost:5353", "DNS service to probe")
	f.String("lookup", "google.com", "Hostname to lookup")
	f.Bool("version", false, "Display version and exit")

	// parse command line
	if err := f.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}

		return fmt.Errorf("error parsing command line flags: %w", err)
	}

	// handle if version was requested
	if version, err := f.GetBool("version"); err == nil && version {
		_, err := fmt.Fprintf(stdout, "dns-checker %s\n", Version)
		return err
	}

	// set up config
	c, err := configger.LoadConfig(f, configger.WithoutConfigurationFile(), configger.WithEnvPrefix("DNS"))
	if err != nil {
		return fmt.Errorf("error setting up config: %w", err)
	}

	// set up probe of specified DNS server
	p, err := probe.New(c.String("server"), c.String("lookup"))
	if err != nil {
		return fmt.Errorf("error setting up probe: %w", err)
	}

	mux := http.NewServeMux()

	// handler for health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := p.Check(r.Context()); err != nil {
			logger.Error("DNS check failed", "server", c.String("server"), "lookup", c.String("lookup"), "error", err)
			http.Error(w, "DNS check failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// http server with some timeouts
	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// listen before serving so listen errors are returned straight away
	ln, err := net.Listen("tcp", c.String("listen"))
	if err != nil {
		return fmt.Errorf("error listening: %w", err)
	}

	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(ln)
	}()

	select {
	case err := <-errc:
		return fmt.Errorf("error with HTTP service: %w", err)
	case <-ctx.Done():
	}

	// wait for in-flight requests to finish before returning
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}

	return nil
}
