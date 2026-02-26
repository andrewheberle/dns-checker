package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/oklog/run"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	// command line flags
	pflag.String("listen", "localhost:8080", "Service listen address")
	pflag.String("server", "localhost:5353", "DNS service to probe")
	pflag.String("lookup", "google.com", "Hostname to lookup")
	pflag.Parse()

	// viper setup
	viper.SetEnvPrefix("dns")
	viper.AutomaticEnv()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		slog.Error("Failed to bind flags", "error", err)
		os.Exit(1)
	}

	// set up custom resolver to use specified DNS server
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 4,
			}

			return d.DialContext(ctx, network, viper.GetString("server"))
		},
	}

	mux := http.NewServeMux()

	// handler for health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()

		_, err := resolver.LookupHost(ctx, viper.GetString("lookup"))
		if err != nil {
			slog.Error("DNS check failed", "server", viper.GetString("server"), "lookup", viper.GetString("lookup"), "error", err)
			http.Error(w, "DNS check failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	// http server with some timeouts
	srv := &http.Server{
		Addr:         viper.GetString("listen"),
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
