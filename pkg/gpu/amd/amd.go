// Package amd implements GPU backends for AMD hardware.
//
// The default backend is pure Go: it discovers cards through sysfs and reads
// telemetry from the amdgpu sysfs/hwmon interfaces, which are rich enough for
// utilization, VRAM, temperature, power and fan without any tooling. Building
// with -tags rocm registers a higher-priority backend backed by the ROCm SMI
// library (see rocm.go).
package amd

import (
	"context"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
	"github.com/GabDevjs/hugin-telemetry/pkg/gpu/sysfs"
)

// Priority sits above the generic sysfs backend but below the cgo ROCm backend.
const Priority = 50

type backend struct{}

// New returns the sysfs-based AMD backend.
func New() gpu.Backend { return backend{} }

func init() { gpu.Register("amd", New) }

func (backend) Name() string  { return "amd" }
func (backend) Priority() int { return Priority }

func (backend) Available(context.Context) bool {
	devs, err := sysfs.List()
	if err != nil {
		return false
	}
	for _, d := range devs {
		if d.Vendor == gpu.VendorAMD {
			return true
		}
	}
	return false
}

func (backend) Discover(context.Context) ([]gpu.Device, error) {
	all, err := sysfs.List()
	if err != nil {
		return nil, err
	}
	out := make([]gpu.Device, 0, len(all))
	for _, d := range all {
		if d.Vendor != gpu.VendorAMD {
			continue
		}
		d.ID = "amd:" + d.DRMCard
		out = append(out, d)
	}
	return out, nil
}

// Telemetry reads the amdgpu sysfs/hwmon metrics for the device. The richer
// ROCm SMI path is enabled with -tags rocm.
func (backend) Telemetry(_ context.Context, d gpu.Device) (gpu.Telemetry, error) {
	return sysfs.Sample(d)
}
