// Command hugin is a vendor-agnostic GPU inventory and telemetry tool. It
// detects NVIDIA, AMD and integrated (Intel) GPUs and reports them through a
// CLI or an HTTP API.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/GabDevjs/hugin-telemetry/internal/cli"

	// Register the built-in GPU telemetry backends. cgo backends (nvml, rocm)
	// join in automatically when built with their tags.
	_ "github.com/GabDevjs/hugin-telemetry/pkg/gpu/all"
)

func main() {
	// os.Exit skips deferred calls, so the signal cleanup lives in run().
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return cli.Run(ctx, os.Args[1:])
}
