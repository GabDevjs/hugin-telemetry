//go:build rocm

// This file is compiled only with `-tags rocm`. It links against AMD's ROCm
// System Management Interface library (librocm_smi64) and requires the ROCm SMI
// headers at build time and the shared library at run time.
//
//	go build -tags rocm ./...
//
// The bindings below are intentionally minimal; extend them as your telemetry
// needs grow. The pure-Go sysfs backend in amd.go remains the portable default.

package amd

/*
#cgo LDFLAGS: -lrocm_smi64
#include <stdlib.h>
#include <rocm_smi/rocm_smi.h>
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
	"github.com/GabDevjs/hugin-telemetry/pkg/gpu/sysfs"
)

// ROCmPriority outranks the sysfs-based AMD backend.
const ROCmPriority = 100

type rocmBackend struct{}

// NewROCm returns the ROCm SMI backed AMD backend.
func NewROCm() gpu.Backend { return rocmBackend{} }

func init() { gpu.Register("amd-rocm", NewROCm) }

var (
	rocmOnce sync.Once
	rocmOK   bool
)

func rocmEnsure() bool {
	rocmOnce.Do(func() {
		rocmOK = C.rsmi_init(0) == C.RSMI_STATUS_SUCCESS
	})
	return rocmOK
}

func (rocmBackend) Name() string  { return "amd-rocm" }
func (rocmBackend) Priority() int { return ROCmPriority }

func (rocmBackend) Available(context.Context) bool { return rocmEnsure() }

// Discover reuses sysfs enumeration so device identity stays consistent with
// the pure-Go backend; ROCm SMI is used for telemetry only.
func (rocmBackend) Discover(ctx context.Context) ([]gpu.Device, error) {
	all, err := sysfs.List()
	if err != nil {
		return nil, err
	}
	out := make([]gpu.Device, 0, len(all))
	for i, d := range all {
		if d.Vendor != gpu.VendorAMD {
			continue
		}
		d.ID = fmt.Sprintf("amd:%d", i)
		out = append(out, d)
	}
	return out, nil
}

func (rocmBackend) Telemetry(_ context.Context, d gpu.Device) (gpu.Telemetry, error) {
	if !rocmEnsure() {
		return gpu.Telemetry{}, fmt.Errorf("rocm: not initialized")
	}
	// Map the device to a ROCm SMI index via its DRM card number.
	idx := C.uint32_t(drmIndex(d.DRMCard))
	t := gpu.Telemetry{DeviceID: d.ID, Timestamp: time.Now()}

	var busy C.uint32_t
	if C.rsmi_dev_busy_percent_get(idx, &busy) == C.RSMI_STATUS_SUCCESS {
		t.UtilizationGPU = gpu.Float(float64(busy))
	}
	var total, used C.uint64_t
	if C.rsmi_dev_memory_total_get(idx, C.RSMI_MEM_TYPE_VRAM, &total) == C.RSMI_STATUS_SUCCESS {
		t.MemoryTotalBytes = gpu.Uint(uint64(total))
	}
	if C.rsmi_dev_memory_usage_get(idx, C.RSMI_MEM_TYPE_VRAM, &used) == C.RSMI_STATUS_SUCCESS {
		t.MemoryUsedBytes = gpu.Uint(uint64(used))
	}
	var tempMilliC C.int64_t
	if C.rsmi_dev_temp_metric_get(idx, 0, C.RSMI_TEMP_CURRENT, &tempMilliC) == C.RSMI_STATUS_SUCCESS {
		t.TemperatureC = gpu.Float(float64(tempMilliC) / 1000)
	}
	var powerMicroW C.uint64_t
	if C.rsmi_dev_power_ave_get(idx, 0, &powerMicroW) == C.RSMI_STATUS_SUCCESS {
		t.PowerWatts = gpu.Float(float64(powerMicroW) / 1e6)
	}
	return t, nil
}

func drmIndex(card string) int {
	var n int
	_, _ = fmt.Sscanf(card, "card%d", &n)
	return n
}
