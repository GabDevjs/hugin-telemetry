package nvidia

import (
	"context"
	"testing"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

func TestNormalizePCI(t *testing.T) {
	cases := map[string]string{
		"00000000:01:00.0": "0000:01:00.0",
		"0000:01:00.0":     "0000:01:00.0",
		"00000000:65:00.0": "0000:65:00.0",
	}
	for in, want := range cases {
		if got := normalizePCI(in); got != want {
			t.Errorf("normalizePCI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTelemetry(t *testing.T) {
	// Order: mem.total, mem.used, util.gpu, util.mem, temp, power, fan, coreclk, memclk
	row := []string{"16384", "2048", "55", "30", "64", "120.5", "40", "2550", "10501"}
	tlm := parseTelemetry(gpu.Device{ID: "nvidia:abc"}, row)

	if tlm.MemoryTotalBytes == nil || *tlm.MemoryTotalBytes != 16384*1024*1024 {
		t.Errorf("mem total = %v", tlm.MemoryTotalBytes)
	}
	if tlm.UtilizationGPU == nil || *tlm.UtilizationGPU != 55 {
		t.Errorf("util gpu = %v", tlm.UtilizationGPU)
	}
	if tlm.PowerWatts == nil || *tlm.PowerWatts != 120.5 {
		t.Errorf("power = %v", tlm.PowerWatts)
	}
	if tlm.ClockMemMHz == nil || *tlm.ClockMemMHz != 10501 {
		t.Errorf("mem clock = %v", tlm.ClockMemMHz)
	}
}

func TestParseTelemetryHandlesNA(t *testing.T) {
	row := []string{"16384", "2048", "[N/A]", "30", "[Not Supported]", "120", "40", "2550", "10501"}
	tlm := parseTelemetry(gpu.Device{ID: "nvidia:abc"}, row)

	if tlm.UtilizationGPU != nil {
		t.Errorf("expected nil utilization for [N/A], got %v", *tlm.UtilizationGPU)
	}
	if tlm.TemperatureC != nil {
		t.Errorf("expected nil temperature for [Not Supported], got %v", *tlm.TemperatureC)
	}
}

func TestDiscoverParsesCSV(t *testing.T) {
	orig := runCSV
	t.Cleanup(func() { runCSV = orig })
	runCSV = func(_ context.Context, _ string, _ ...string) ([][]string, error) {
		return [][]string{
			{"0", "NVIDIA GeForce RTX 5060 Ti", "00000000:01:00.0", "GPU-abc123"},
		}, nil
	}

	devs, err := smiBackend{}.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devs))
	}
	d := devs[0]
	if d.PCIAddress != "0000:01:00.0" {
		t.Errorf("pci = %q", d.PCIAddress)
	}
	if d.ID != "nvidia:GPU-abc123" {
		t.Errorf("id = %q", d.ID)
	}
	if d.Vendor != gpu.VendorNVIDIA {
		t.Errorf("vendor = %q", d.Vendor)
	}
}
