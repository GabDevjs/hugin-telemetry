package gpu

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// ErrUnknownBackend is returned when telemetry is requested for a device whose
// owning backend is not available in the manager.
var ErrUnknownBackend = errors.New("gpu: unknown or unavailable backend for device")

// Manager aggregates the registered backends into a single view of the host's
// GPUs. The zero value is not usable; create one with [NewManager].
type Manager struct {
	backends map[string]Backend
}

// NewManager instantiates every registered backend and keeps the ones that
// report themselves available on the current host.
func NewManager(ctx context.Context) *Manager {
	m := &Manager{backends: map[string]Backend{}}
	for _, b := range Backends() {
		if b.Available(ctx) {
			m.backends[b.Name()] = b
		}
	}
	return m
}

// ActiveBackends returns the names of the available backends, ordered by
// descending priority.
func (m *Manager) ActiveBackends() []string {
	bs := make([]Backend, 0, len(m.backends))
	for _, b := range m.backends {
		bs = append(bs, b)
	}
	sort.Slice(bs, func(i, j int) bool {
		if bs[i].Priority() != bs[j].Priority() {
			return bs[i].Priority() > bs[j].Priority()
		}
		return bs[i].Name() < bs[j].Name()
	})
	names := make([]string, len(bs))
	for i, b := range bs {
		names[i] = b.Name()
	}
	return names
}

// Discover enumerates every GPU on the host. When several backends report the
// same physical device (matched by PCI address, then DRM node), the one from
// the highest-priority backend is kept. Errors from individual backends are
// collected and returned alongside whatever devices were discovered.
func (m *Manager) Discover(ctx context.Context) ([]Device, error) {
	// Iterate backends from highest to lowest priority so the first device we
	// see for a given key is the one we keep.
	ordered := make([]Backend, 0, len(m.backends))
	for _, b := range m.backends {
		ordered = append(ordered, b)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Priority() != ordered[j].Priority() {
			return ordered[i].Priority() > ordered[j].Priority()
		}
		return ordered[i].Name() < ordered[j].Name()
	})

	seen := map[string]Device{}
	var order []string
	var errs []error

	for _, b := range ordered {
		devs, err := b.Discover(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("backend %s: %w", b.Name(), err))
			continue
		}
		for _, d := range devs {
			d.Backend = b.Name()
			key := dedupKey(d)
			if existing, dup := seen[key]; dup {
				// The higher-priority backend already owns this device; just
				// backfill any identifying fields it could not determine.
				seen[key] = mergeDevice(existing, d)
				continue
			}
			seen[key] = d
			order = append(order, key)
		}
	}

	// Stable ordering: PCI address, then DRM card, then ID.
	sort.Slice(order, func(i, j int) bool {
		return deviceLess(seen[order[i]], seen[order[j]])
	})

	out := make([]Device, len(order))
	for i, key := range order {
		d := seen[key]
		d.Index = i
		out[i] = d
	}
	return out, errors.Join(errs...)
}

// Telemetry samples runtime metrics for d using the backend that owns it.
func (m *Manager) Telemetry(ctx context.Context, d Device) (Telemetry, error) {
	b, ok := m.backends[d.Backend]
	if !ok {
		return Telemetry{}, fmt.Errorf("%w: %q", ErrUnknownBackend, d.Backend)
	}
	t, err := b.Telemetry(ctx, d)
	if err != nil {
		return Telemetry{}, err
	}
	if t.DeviceID == "" {
		t.DeviceID = d.ID
	}
	return t, nil
}

// mergeDevice fills empty identifying fields of keep (the higher-priority
// device that wins ownership) from drop (a lower-priority duplicate). It never
// overrides ownership-defining fields such as Backend, ID, Name or Vendor.
func mergeDevice(keep, drop Device) Device {
	if keep.PCIAddress == "" {
		keep.PCIAddress = drop.PCIAddress
	}
	if keep.DRMCard == "" {
		keep.DRMCard = drop.DRMCard
	}
	if !keep.Integrated && drop.Integrated {
		keep.Integrated = drop.Integrated
	}
	return keep
}

func dedupKey(d Device) string {
	switch {
	case d.PCIAddress != "":
		return "pci:" + d.PCIAddress
	case d.DRMCard != "":
		return "drm:" + d.DRMCard
	default:
		return "id:" + d.ID
	}
}

func deviceLess(a, b Device) bool {
	if a.PCIAddress != b.PCIAddress {
		return a.PCIAddress < b.PCIAddress
	}
	if a.DRMCard != b.DRMCard {
		return a.DRMCard < b.DRMCard
	}
	return a.ID < b.ID
}
