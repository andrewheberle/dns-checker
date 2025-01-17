package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

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
	viper.BindPFlags(pflag.CommandLine)

	// http server with some timeouts
	srv := &http.Server{
		Addr:         viper.GetString("listen"),
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
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

	// handler for health check
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, err := resolver.LookupHost(context.Background(), viper.GetString("lookup"))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	// start http service
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("Error with HTTP service", "error", err)
	}
}
