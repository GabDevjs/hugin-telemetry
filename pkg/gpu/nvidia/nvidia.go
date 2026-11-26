// Package nvidia implements GPU backends for NVIDIA hardware.
//
// By default it shells out to nvidia-smi, which keeps the build free of cgo and
// of the NVML headers. Building with -tags nvml additionally registers a
// higher-priority backend that links against libnvidia-ml directly (see
// nvml.go).
package nvidia

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

// Priority sits above sysfs but below the cgo NVML backend.
const Priority = 50

// smiTimeout bounds each nvidia-smi invocation.
var smiTimeout = 5 * time.Second

// lookPath / runCSV are indirected for testing.
var (
	lookPath = exec.LookPath
	runCSV   = defaultRunCSV
)

type smiBackend struct{}

// NewSMI returns the nvidia-smi based backend.
func NewSMI() gpu.Backend { return smiBackend{} }

func init() { gpu.Register("nvidia-smi", NewSMI) }

func (smiBackend) Name() string  { return "nvidia-smi" }
func (smiBackend) Priority() int { return Priority }

func (smiBackend) Available(context.Context) bool {
	_, err := lookPath("nvidia-smi")
	return err == nil
}

const discoverQuery = "index,name,pci.bus_id,uuid"

func (smiBackend) Discover(ctx context.Context) ([]gpu.Device, error) {
	rows, err := runCSV(ctx, discoverQuery)
	if err != nil {
		return nil, err
	}
	out := make([]gpu.Device, 0, len(rows))
	for _, r := range rows {
		if len(r) < 4 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(r[0]))
		out = append(out, gpu.Device{
			ID:         "nvidia:" + strings.TrimSpace(r[3]), // uuid
			Index:      idx,
			Vendor:     gpu.VendorNVIDIA,
			Name:       strings.TrimSpace(r[1]),
			PCIAddress: normalizePCI(r[2]),
		})
	}
	return out, nil
}

const telemetryQuery = "memory.total,memory.used,utilization.gpu,utilization.memory," +
	"temperature.gpu,power.draw,fan.speed,clocks.current.graphics,clocks.current.memory"

func (b smiBackend) Telemetry(ctx context.Context, d gpu.Device) (gpu.Telemetry, error) {
	rows, err := runCSV(ctx, telemetryQuery, "--id="+pciForQuery(d))
	if err != nil {
		return gpu.Telemetry{}, err
	}
	if len(rows) == 0 {
		return gpu.Telemetry{}, fmt.Errorf("nvidia-smi returned no data for %s", d.ID)
	}
	return parseTelemetry(d, rows[0]), nil
}

func parseTelemetry(d gpu.Device, r []string) gpu.Telemetry {
	t := gpu.Telemetry{DeviceID: d.ID, Timestamp: time.Now()}
	get := func(i int) (string, bool) {
		if i >= len(r) {
			return "", false
		}
		s := strings.TrimSpace(r[i])
		if s == "" || strings.HasPrefix(s, "[N/A") || strings.HasPrefix(s, "[Not") {
			return "", false
		}
		return s, true
	}
	if s, ok := get(0); ok {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			t.MemoryTotalBytes = gpu.Uint(uint64(v) * 1024 * 1024) // MiB -> bytes
		}
	}
	if s, ok := get(1); ok {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			t.MemoryUsedBytes = gpu.Uint(uint64(v) * 1024 * 1024)
		}
	}
	t.UtilizationGPU = parseFloatPtr(get(2))
	t.UtilizationMem = parseFloatPtr(get(3))
	t.TemperatureC = parseFloatPtr(get(4))
	t.PowerWatts = parseFloatPtr(get(5))
	t.FanPercent = parseFloatPtr(get(6))
	if s, ok := get(7); ok {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			t.ClockCoreMHz = gpu.Uint(v)
		}
	}
	if s, ok := get(8); ok {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			t.ClockMemMHz = gpu.Uint(v)
		}
	}
	return t
}

func parseFloatPtr(s string, ok bool) *float64 {
	if !ok {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return gpu.Float(v)
}

// defaultRunCSV runs nvidia-smi with a CSV query and returns the parsed rows.
func defaultRunCSV(ctx context.Context, query string, extra ...string) ([][]string, error) {
	ctx, cancel := context.WithTimeout(ctx, smiTimeout)
	defer cancel()

	args := append([]string{
		"--query-gpu=" + query,
		"--format=csv,noheader,nounits",
	}, extra...)

	cmd := exec.CommandContext(ctx, "nvidia-smi", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi: %w", err)
	}
	return parseCSV(string(out))
}

func parseCSV(s string) ([][]string, error) {
	r := csv.NewReader(bufio.NewReader(strings.NewReader(s)))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// normalizePCI converts nvidia-smi's "00000000:01:00.0" bus id into the sysfs
// "0000:01:00.0" form so devices deduplicate across backends.
func normalizePCI(busID string) string {
	s := strings.ToLower(strings.TrimSpace(busID))
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return s
	}
	domain := strings.TrimLeft(parts[0], "0")
	for len(domain) < 4 {
		domain = "0" + domain
	}
	return domain + ":" + parts[1]
}

// pciForQuery returns the identifier passed to nvidia-smi --id. The PCI address
// is the most stable selector; fall back to the UUID embedded in the device ID.
func pciForQuery(d gpu.Device) string {
	if d.PCIAddress != "" {
		return d.PCIAddress
	}
	return strings.TrimPrefix(d.ID, "nvidia:")
}
