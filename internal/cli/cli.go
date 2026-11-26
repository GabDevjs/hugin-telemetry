// Package cli implements the hugin command-line interface. It is a thin layer
// over the gpu package: a small subcommand router built on the standard
// library's flag package, with no external dependencies.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/GabDevjs/hugin-telemetry/internal/buildinfo"
)

// Run dispatches argv (excluding the program name) and returns a process exit
// code. It is the single entry point used by cmd/hugin.
func Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "list", "ls":
		return cmdList(ctx, rest)
	case "watch", "top":
		return cmdWatch(ctx, rest)
	case "serve":
		return cmdServe(ctx, rest)
	case "version", "--version", "-v":
		fmt.Println(buildinfo.String())
		return 0
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "hugin: unknown command %q\n\n", cmd)
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `hugin — vendor-agnostic GPU inventory & telemetry

Usage:
  hugin <command> [flags]

Commands:
  list      List the GPUs detected on this host
  watch     Continuously print live telemetry
  serve     Run the HTTP API (devices & telemetry as JSON)
  version   Print version information
  help      Show this help

Run "hugin <command> -h" for command-specific flags.
`)
}
