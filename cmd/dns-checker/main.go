package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var Version = "dev"

func main() {
	// stop gracefully on SIGINT and SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		stop()
		os.Exit(1)
	}
}
