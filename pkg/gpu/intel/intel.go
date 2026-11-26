// Package intel implements a GPU backend for Intel graphics, which are usually
// integrated (iGPU). Discovery and the limited telemetry the kernel exposes
// come from sysfs; richer metrics would require the i915/Xe perf interfaces,
// which are left as an extension point.
package intel

import (
	"context"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
	"github.com/GabDevjs/hugin-telemetry/pkg/gpu/sysfs"
)

// Priority sits above the generic sysfs backend.
const Priority = 50

type backend struct{}

// New returns the sysfs-based Intel backend.
func New() gpu.Backend { return backend{} }

func init() { gpu.Register("intel", New) }

func (backend) Name() string  { return "intel" }
func (backend) Priority() int { return Priority }

func (backend) Available(context.Context) bool {
	devs, err := sysfs.List()
	if err != nil {
		return false
	}
	for _, d := range devs {
		if d.Vendor == gpu.VendorIntel {
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
		if d.Vendor != gpu.VendorIntel {
			continue
		}
		d.ID = "intel:" + d.DRMCard
		d.Integrated = true
		out = append(out, d)
	}
	return out, nil
}

func (backend) Telemetry(_ context.Context, d gpu.Device) (gpu.Telemetry, error) {
	return sysfs.Sample(d)
}
