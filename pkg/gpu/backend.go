package gpu

import (
	"context"
	"sort"
	"sync"
)

// Backend is implemented by each vendor integration. Implementations must be
// safe for concurrent use.
type Backend interface {
	// Name is a short, unique identifier for the backend (e.g. "nvidia-smi").
	Name() string
	// Priority decides which backend wins when several report the same
	// physical device. Higher priority wins; richer integrations (cgo/NVML)
	// should outrank generic ones (sysfs).
	Priority() int
	// Available reports whether the backend can run on the current host
	// (drivers present, tools installed, …). It should be cheap.
	Available(ctx context.Context) bool
	// Discover enumerates the devices visible to this backend.
	Discover(ctx context.Context) ([]Device, error)
	// Telemetry samples runtime metrics for a device previously returned by
	// Discover on the same backend.
	Telemetry(ctx context.Context, d Device) (Telemetry, error)
}

// Factory constructs a [Backend]. Registering a factory rather than an instance
// lets the [Manager] decide when (and whether) to instantiate a backend.
type Factory func() Backend

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register adds a backend factory to the global registry. It is intended to be
// called from a backend package's init function. Registering the same name
// twice panics, which surfaces duplicate wiring at startup.
func Register(name string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic("gpu: backend already registered: " + name)
	}
	registry[name] = f
}

// Backends instantiates every registered backend, sorted by descending
// priority (ties broken by name for determinism).
func Backends() []Backend {
	registryMu.RLock()
	defer registryMu.RUnlock()

	out := make([]Backend, 0, len(registry))
	for _, f := range registry {
		out = append(out, f())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority() != out[j].Priority() {
			return out[i].Priority() > out[j].Priority()
		}
		return out[i].Name() < out[j].Name()
	})
	return out
}

// RegisteredNames returns the names of all registered backend factories.
func RegisteredNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
