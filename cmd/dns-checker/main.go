package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/andrewheberle/configger"
	"github.com/oklog/run"
	"github.com/spf13/pflag"
)

var Version = "dev"

func main() {
	// command line flags
	f := pflag.NewFlagSet("dns-checker", pflag.ContinueOnError)
	f.String("listen", "localhost:8080", "Service listen address")
	f.String("server", "localhost:5353", "DNS service to probe")
	f.String("lookup", "google.com", "Hostname to lookup")
	f.Bool("version", false, "Display version and exit")

	// parse command line
	if err := f.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing command line flags: %s\n", err)
		os.Exit(1)
	}

	// handle if version was requested
	if version, err := f.GetBool("version"); err == nil && version {
		fmt.Printf("dns-checker %s\n", Version)
		os.Exit(0)
	}

	// set up config
	c, err := configger.LoadConfig(f, configger.WithoutConfigurationFile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting up config: %s\n", err)
		os.Exit(1)
	}

	// set up custom resolver to use specified DNS server
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 4,
			}

			return d.DialContext(ctx, network, c.String("server"))
		},
	}

	mux := http.NewServeMux()

	// handler for health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		_, err := resolver.LookupHost(ctx, c.String("lookup"))
		if err != nil {
			slog.Error("DNS check failed", "server", c.String("server"), "lookup", c.String("lookup"), "error", err)
			http.Error(w, "DNS check failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// http server with some timeouts
	srv := &http.Server{
		Addr:         c.String("listen"),
		Handler:      mux,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
	}

	g := run.Group{}

	// add http server
	{
		g.Add(func() error {
			return srv.ListenAndServe()
		}, func(err error) {
			if err != nil {
				slog.Error("Error with HTTP service", "error", err)
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				srv.Shutdown(ctx)
				cancel()
			}()
		})
	}

	// add signal handler for SIGINT and SIGTERM
	{
		g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))
	}

	// start run group
	if err := g.Run(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, run.ErrSignal) {
			slog.Error("Error from service", "error", err)
		}
	}
}
