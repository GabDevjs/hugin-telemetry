package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

func cmdWatch(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 2*time.Second, "sampling interval")
	count := fs.Int("count", 0, "number of samples (0 = run until interrupted)")
	asJSON := fs.Bool("json", false, "emit one JSON object per sample")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	mgr := gpu.NewManager(ctx)
	devs, err := mgr.Discover(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hugin: discovery reported errors: %v\n", err)
	}
	if len(devs) == 0 {
		fmt.Println("No GPUs detected.")
		return 0
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for i := 0; *count == 0 || i < *count; i++ {
		samples := sampleAll(ctx, mgr, devs)
		if *asJSON {
			if err := writeJSON(os.Stdout, samples); err != nil {
				fmt.Fprintf(os.Stderr, "hugin: %v\n", err)
				return 1
			}
		} else {
			fmt.Printf("\n%s\n", time.Now().Format(time.RFC3339))
			byID := make(map[string]gpu.Telemetry, len(samples))
			for _, t := range samples {
				byID[t.DeviceID] = t
			}
			renderTelemetry(os.Stdout, devs, byID)
		}

		if *count != 0 && i == *count-1 {
			break
		}
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		}
	}
	return 0
}

func sampleAll(ctx context.Context, mgr *gpu.Manager, devs []gpu.Device) []gpu.Telemetry {
	out := make([]gpu.Telemetry, 0, len(devs))
	for _, d := range devs {
		t, err := mgr.Telemetry(ctx, d)
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	return out
}
