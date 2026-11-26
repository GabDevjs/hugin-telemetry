package gpu

import (
	"context"
	"testing"
)

type fakeBackend struct {
	name     string
	priority int
	devices  []Device
}

func (f fakeBackend) Name() string                   { return f.name }
func (f fakeBackend) Priority() int                  { return f.priority }
func (f fakeBackend) Available(context.Context) bool { return true }
func (f fakeBackend) Discover(context.Context) ([]Device, error) {
	return f.devices, nil
}
func (f fakeBackend) Telemetry(_ context.Context, d Device) (Telemetry, error) {
	return Telemetry{DeviceID: d.ID, UtilizationGPU: Float(42)}, nil
}

// newTestManager wires fake backends without touching the global registry.
func newTestManager(bs ...Backend) *Manager {
	m := &Manager{backends: map[string]Backend{}}
	for _, b := range bs {
		m.backends[b.Name()] = b
	}
	return m
}

func TestDiscoverDeduplicatesByPriority(t *testing.T) {
	low := fakeBackend{
		name: "sysfs", priority: 10,
		devices: []Device{{ID: "sysfs:card0", PCIAddress: "0000:01:00.0", Name: "generic"}},
	}
	high := fakeBackend{
		name: "nvidia", priority: 50,
		devices: []Device{{ID: "nvidia:0", PCIAddress: "0000:01:00.0", Name: "RTX"}},
	}

	mgr := newTestManager(low, high)
	devs, err := mgr.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("expected 1 deduplicated device, got %d", len(devs))
	}
	if got := devs[0].Backend; got != "nvidia" {
		t.Errorf("expected higher-priority backend to win, got %q", got)
	}
	if devs[0].Index != 0 {
		t.Errorf("expected reindexed device 0, got %d", devs[0].Index)
	}
}

func TestDiscoverBackfillsFieldsFromDuplicate(t *testing.T) {
	// nvidia-smi knows the PCI address but not the DRM card; sysfs knows both.
	high := fakeBackend{
		name: "nvidia", priority: 50,
		devices: []Device{{ID: "nvidia:0", PCIAddress: "0000:01:00.0", Name: "RTX"}},
	}
	low := fakeBackend{
		name: "sysfs", priority: 10,
		devices: []Device{{ID: "sysfs:card0", PCIAddress: "0000:01:00.0", DRMCard: "card0"}},
	}

	mgr := newTestManager(high, low)
	devs, err := mgr.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devs))
	}
	if devs[0].Backend != "nvidia" {
		t.Errorf("backend = %q, want nvidia", devs[0].Backend)
	}
	if devs[0].DRMCard != "card0" {
		t.Errorf("expected DRM card backfilled from sysfs, got %q", devs[0].DRMCard)
	}
}

func TestDiscoverKeepsDistinctDevices(t *testing.T) {
	b := fakeBackend{
		name: "sysfs", priority: 10,
		devices: []Device{
			{ID: "a", PCIAddress: "0000:01:00.0"},
			{ID: "b", PCIAddress: "0000:02:00.0"},
		},
	}
	mgr := newTestManager(b)
	devs, err := mgr.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(devs) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devs))
	}
	// Stable ordering by PCI address.
	if devs[0].PCIAddress != "0000:01:00.0" || devs[1].PCIAddress != "0000:02:00.0" {
		t.Errorf("devices not ordered by PCI address: %+v", devs)
	}
}

func TestTelemetryRoutesToOwningBackend(t *testing.T) {
	b := fakeBackend{name: "nvidia", priority: 50}
	mgr := newTestManager(b)

	tlm, err := mgr.Telemetry(context.Background(), Device{ID: "x", Backend: "nvidia"})
	if err != nil {
		t.Fatalf("Telemetry: %v", err)
	}
	if tlm.UtilizationGPU == nil || *tlm.UtilizationGPU != 42 {
		t.Errorf("unexpected telemetry: %+v", tlm)
	}
}

func TestTelemetryUnknownBackend(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.Telemetry(context.Background(), Device{ID: "x", Backend: "ghost"})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestVendorFromPCIID(t *testing.T) {
	cases := map[uint16]Vendor{
		0x10de: VendorNVIDIA,
		0x1002: VendorAMD,
		0x8086: VendorIntel,
		0x1234: VendorUnknown,
	}
	for id, want := range cases {
		if got := VendorFromPCIID(id); got != want {
			t.Errorf("VendorFromPCIID(%#x) = %q, want %q", id, got, want)
		}
	}
}
