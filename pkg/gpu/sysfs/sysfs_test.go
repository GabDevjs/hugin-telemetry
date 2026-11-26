package sysfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

// writeFile is a test helper that creates parent dirs and writes content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureRoot builds a fake /sys/class/drm tree with one AMD card and points
// Root at it for the duration of the test.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	card := filepath.Join(root, "card0", "device")

	writeFile(t, filepath.Join(card, "vendor"), "0x1002\n")
	writeFile(t, filepath.Join(card, "device"), "0x744c\n")
	writeFile(t, filepath.Join(card, "gpu_busy_percent"), "73\n")
	writeFile(t, filepath.Join(card, "mem_info_vram_total"), "25757220864\n")
	writeFile(t, filepath.Join(card, "mem_info_vram_used"), "1073741824\n")

	hwmon := filepath.Join(card, "hwmon", "hwmon3")
	writeFile(t, filepath.Join(hwmon, "temp1_input"), "61000\n")      // 61°C
	writeFile(t, filepath.Join(hwmon, "power1_average"), "180000000") // 180 W
	writeFile(t, filepath.Join(hwmon, "fan1_input"), "1500\n")
	writeFile(t, filepath.Join(hwmon, "fan1_max"), "3000\n")

	// A connector node and render node that must be ignored by discovery.
	writeFile(t, filepath.Join(root, "card0-DP-1", "status"), "connected\n")
	writeFile(t, filepath.Join(root, "renderD128", "dev"), "226:128\n")

	old := Root
	Root = root
	t.Cleanup(func() { Root = old })
	return root
}

func TestListDiscoversCards(t *testing.T) {
	fixtureRoot(t)

	devs, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(devs) != 1 {
		t.Fatalf("expected 1 card, got %d: %+v", len(devs), devs)
	}
	d := devs[0]
	if d.Vendor != gpu.VendorAMD {
		t.Errorf("vendor = %q, want amd", d.Vendor)
	}
	if d.DRMCard != "card0" {
		t.Errorf("drm card = %q, want card0", d.DRMCard)
	}
}

func TestSampleReadsTelemetry(t *testing.T) {
	fixtureRoot(t)
	now = func() time.Time { return time.Unix(0, 0) }
	t.Cleanup(func() { now = time.Now })

	tlm, err := Sample(gpu.Device{ID: "amd:card0", DRMCard: "card0"})
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}

	if tlm.UtilizationGPU == nil || *tlm.UtilizationGPU != 73 {
		t.Errorf("utilization = %v, want 73", tlm.UtilizationGPU)
	}
	if tlm.MemoryTotalBytes == nil || *tlm.MemoryTotalBytes != 25757220864 {
		t.Errorf("vram total = %v", tlm.MemoryTotalBytes)
	}
	if tlm.TemperatureC == nil || *tlm.TemperatureC != 61 {
		t.Errorf("temp = %v, want 61", tlm.TemperatureC)
	}
	if tlm.PowerWatts == nil || *tlm.PowerWatts != 180 {
		t.Errorf("power = %v, want 180", tlm.PowerWatts)
	}
	if tlm.FanPercent == nil || *tlm.FanPercent != 50 {
		t.Errorf("fan = %v, want 50", tlm.FanPercent)
	}
}
