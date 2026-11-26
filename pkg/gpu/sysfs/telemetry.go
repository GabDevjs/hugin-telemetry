package sysfs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

// now is overridable in tests.
var now = time.Now

// Sample reads whatever telemetry the kernel exposes for the device under
// /sys/class/drm/<card>/device. This works without any vendor tooling and is
// most complete for AMD (amdgpu) cards; NVIDIA's proprietary driver exposes
// little here, so prefer the nvidia backend for those.
func Sample(d gpu.Device) (gpu.Telemetry, error) {
	t := gpu.Telemetry{DeviceID: d.ID, Timestamp: now()}
	if d.DRMCard == "" {
		return t, nil
	}
	devDir := filepath.Join(Root, d.DRMCard, "device")

	if v, ok := readFloat(filepath.Join(devDir, "gpu_busy_percent")); ok {
		t.UtilizationGPU = gpu.Float(v)
	}
	if v, ok := readUint(filepath.Join(devDir, "mem_info_vram_total")); ok {
		t.MemoryTotalBytes = gpu.Uint(v)
	}
	if v, ok := readUint(filepath.Join(devDir, "mem_info_vram_used")); ok {
		t.MemoryUsedBytes = gpu.Uint(v)
	}

	readHWMon(devDir, &t)
	return t, nil
}

// readHWMon pulls temperature, power and fan readings from the first hwmon node
// attached to the device.
func readHWMon(devDir string, t *gpu.Telemetry) {
	matches, _ := filepath.Glob(filepath.Join(devDir, "hwmon", "hwmon*"))
	if len(matches) == 0 {
		return
	}
	h := matches[0]

	// temp*_input is reported in milli-degrees Celsius.
	if v, ok := readFloat(filepath.Join(h, "temp1_input")); ok {
		t.TemperatureC = gpu.Float(v / 1000)
	}
	// power*_average / power*_input are in micro-watts.
	if v, ok := readFloat(filepath.Join(h, "power1_average")); ok {
		t.PowerWatts = gpu.Float(v / 1e6)
	} else if v, ok := readFloat(filepath.Join(h, "power1_input")); ok {
		t.PowerWatts = gpu.Float(v / 1e6)
	}
	// fan*_input is in RPM; convert to a percentage of fan*_max when available.
	if rpm, ok := readFloat(filepath.Join(h, "fan1_input")); ok {
		if max, ok := readFloat(filepath.Join(h, "fan1_max")); ok && max > 0 {
			t.FanPercent = gpu.Float(rpm / max * 100)
		}
	}
}

func readString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// readHexID parses a sysfs id file such as "0x10de" into its numeric value.
func readHexID(path string) (uint16, bool) {
	s := strings.TrimPrefix(readString(path), "0x")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}

func readUint(path string) (uint64, bool) {
	s := readString(path)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func readFloat(path string) (float64, bool) {
	s := readString(path)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
