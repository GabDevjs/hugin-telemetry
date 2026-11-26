// Package gpu provides a vendor-agnostic abstraction over GPUs.
//
// The core of the package is the [Backend] interface: each vendor (NVIDIA,
// AMD, Intel/integrated) is implemented as a backend that can discover devices
// and read telemetry. A [Manager] aggregates every registered backend,
// deduplicates the devices they report and routes telemetry requests to the
// backend that owns each device.
//
// Backends register themselves through [Register] from an init function, so a
// program enables them simply by importing the backend package for its side
// effects (see the pkg/gpu/all helper).
package gpu

import "time"

// Vendor identifies the manufacturer of a GPU.
type Vendor string

const (
	VendorNVIDIA  Vendor = "nvidia"
	VendorAMD     Vendor = "amd"
	VendorIntel   Vendor = "intel"
	VendorUnknown Vendor = "unknown"
)

// VendorFromPCIID maps a PCI vendor ID (e.g. 0x10de) to a [Vendor].
func VendorFromPCIID(id uint16) Vendor {
	switch id {
	case 0x10de:
		return VendorNVIDIA
	case 0x1002, 0x1022:
		return VendorAMD
	case 0x8086:
		return VendorIntel
	default:
		return VendorUnknown
	}
}

// Device describes a single GPU as reported by a backend.
//
// Fields that a backend cannot determine are left at their zero value.
type Device struct {
	// ID is a stable identifier for the device, unique within a discovery
	// result (for example "nvidia:0" or a PCI address).
	ID string `json:"id"`
	// Index is the position assigned by the [Manager] after deduplication.
	Index int `json:"index"`
	// Vendor is the manufacturer of the device.
	Vendor Vendor `json:"vendor"`
	// Name is the human-readable product name (e.g. "NVIDIA GeForce RTX 5060 Ti").
	Name string `json:"name"`
	// PCIAddress is the PCI bus address (e.g. "0000:01:00.0") when known.
	PCIAddress string `json:"pci_address,omitempty"`
	// DRMCard is the Linux DRM node (e.g. "card0") when known.
	DRMCard string `json:"drm_card,omitempty"`
	// Integrated reports whether the GPU is integrated (iGPU) rather than discrete.
	Integrated bool `json:"integrated"`
	// Backend is the name of the [Backend] that produced (and owns) this device.
	Backend string `json:"backend"`
}

// Telemetry holds a point-in-time sample of a device's runtime metrics.
//
// Optional metrics use pointers so that "unknown" is distinguishable from a
// genuine zero reading.
type Telemetry struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`

	UtilizationGPU *float64 `json:"utilization_gpu_percent,omitempty"`
	UtilizationMem *float64 `json:"utilization_mem_percent,omitempty"`

	MemoryTotalBytes *uint64 `json:"memory_total_bytes,omitempty"`
	MemoryUsedBytes  *uint64 `json:"memory_used_bytes,omitempty"`

	TemperatureC *float64 `json:"temperature_c,omitempty"`
	PowerWatts   *float64 `json:"power_watts,omitempty"`
	FanPercent   *float64 `json:"fan_percent,omitempty"`

	ClockCoreMHz *uint64 `json:"clock_core_mhz,omitempty"`
	ClockMemMHz  *uint64 `json:"clock_mem_mhz,omitempty"`
}

// Float returns a pointer to v. It is a helper for backends populating the
// optional fields of [Telemetry].
func Float(v float64) *float64 { return &v }

// Uint returns a pointer to v. It is a helper for backends populating the
// optional fields of [Telemetry].
func Uint(v uint64) *uint64 { return &v }
