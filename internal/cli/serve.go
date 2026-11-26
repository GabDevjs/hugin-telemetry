package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/GabDevjs/hugin-telemetry/internal/api"
	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

func cmdServe(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address (development; loopback recommended)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	mgr := gpu.NewManager(ctx)
	srv := api.NewServer(mgr)

	fmt.Printf("hugin: serving on %s (backends: %v)\n", *addr, mgr.ActiveBackends())
	fmt.Println("  GET /healthz")
	fmt.Println("  GET /v1/version")
	fmt.Println("  GET /v1/devices")
	fmt.Println("  GET /v1/telemetry[?id=<device-id>]")

	if err := srv.ListenAndServe(ctx, *addr); err != nil {
		fmt.Fprintf(os.Stderr, "hugin: server error: %v\n", err)
		return 1
	}
	return 0
}
