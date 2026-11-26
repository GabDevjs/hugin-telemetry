// Package buildinfo exposes version metadata injected at build time via
// -ldflags. See the Makefile for the linker flags.
package buildinfo

import (
	"fmt"
	"runtime"
)

// These variables are overridden at build time:
//
//	go build -ldflags "-X github.com/GabDevjs/hugin-telemetry/internal/buildinfo.Version=v1.2.3"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable, single-line version summary.
func String() string {
	return fmt.Sprintf("hugin %s (commit %s, built %s, %s)",
		Version, Commit, Date, runtime.Version())
}
