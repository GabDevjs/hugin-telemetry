package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

func cmdList(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "output as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	mgr := gpu.NewManager(ctx)
	devs, err := mgr.Discover(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hugin: discovery reported errors: %v\n", err)
	}

	if *asJSON {
		if err := writeJSON(os.Stdout, map[string]any{
			"backends": mgr.ActiveBackends(),
			"devices":  devs,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "hugin: %v\n", err)
			return 1
		}
		return 0
	}

	if len(devs) == 0 {
		fmt.Println("No GPUs detected.")
		fmt.Printf("Active backends: %v\n", mgr.ActiveBackends())
		return 0
	}
	renderDevices(os.Stdout, devs)
	return 0
}
