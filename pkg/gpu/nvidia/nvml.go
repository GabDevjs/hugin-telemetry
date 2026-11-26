//go:build nvml

// This file is compiled only with `-tags nvml`. It links against NVIDIA's
// management library (libnvidia-ml) and therefore requires the NVML headers
// (nvml.h, shipped with the CUDA toolkit / datacenter driver) at build time and
// the shared library at run time.
//
//	go build -tags nvml ./...

package nvidia

/*
#cgo LDFLAGS: -lnvidia-ml
#include <stdlib.h>
#include <nvml.h>
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

// NVMLPriority outranks the nvidia-smi backend so NVML wins deduplication.
const NVMLPriority = 100

type nvmlBackend struct{}

// NewNVML returns the NVML-backed NVIDIA backend.
func NewNVML() gpu.Backend { return nvmlBackend{} }

func init() { gpu.Register("nvidia-nvml", NewNVML) }

var (
	nvmlOnce sync.Once
	nvmlOK   bool
)

func nvmlEnsure() bool {
	nvmlOnce.Do(func() {
		nvmlOK = C.nvmlInit_v2() == C.NVML_SUCCESS
	})
	return nvmlOK
}

func (nvmlBackend) Name() string  { return "nvidia-nvml" }
func (nvmlBackend) Priority() int { return NVMLPriority }

func (nvmlBackend) Available(context.Context) bool { return nvmlEnsure() }

func (nvmlBackend) Discover(context.Context) ([]gpu.Device, error) {
	if !nvmlEnsure() {
		return nil, fmt.Errorf("nvml: not initialized")
	}
	var count C.uint
	if r := C.nvmlDeviceGetCount_v2(&count); r != C.NVML_SUCCESS {
		return nil, nvmlErr("nvmlDeviceGetCount_v2", r)
	}
	out := make([]gpu.Device, 0, int(count))
	for i := 0; i < int(count); i++ {
		var dev C.nvmlDevice_t
		if C.nvmlDeviceGetHandleByIndex_v2(C.uint(i), &dev) != C.NVML_SUCCESS {
			continue
		}
		nameBuf := make([]C.char, C.NVML_DEVICE_NAME_BUFFER_SIZE)
		C.nvmlDeviceGetName(dev, &nameBuf[0], C.NVML_DEVICE_NAME_BUFFER_SIZE)

		var pci C.nvmlPciInfo_t
		C.nvmlDeviceGetPciInfo_v3(dev, &pci)

		out = append(out, gpu.Device{
			ID:         fmt.Sprintf("nvidia:%d", i),
			Index:      i,
			Vendor:     gpu.VendorNVIDIA,
			Name:       C.GoString(&nameBuf[0]),
			PCIAddress: normalizePCI(C.GoString(&pci.busId[0])),
		})
	}
	return out, nil
}

func (nvmlBackend) Telemetry(_ context.Context, d gpu.Device) (gpu.Telemetry, error) {
	if !nvmlEnsure() {
		return gpu.Telemetry{}, fmt.Errorf("nvml: not initialized")
	}
	cbus := C.CString(d.PCIAddress)
	defer C.free(unsafe.Pointer(cbus))

	var dev C.nvmlDevice_t
	if r := C.nvmlDeviceGetHandleByPciBusId_v2(cbus, &dev); r != C.NVML_SUCCESS {
		return gpu.Telemetry{}, nvmlErr("nvmlDeviceGetHandleByPciBusId_v2", r)
	}

	t := gpu.Telemetry{DeviceID: d.ID, Timestamp: time.Now()}

	var util C.nvmlUtilization_t
	if C.nvmlDeviceGetUtilizationRates(dev, &util) == C.NVML_SUCCESS {
		t.UtilizationGPU = gpu.Float(float64(util.gpu))
		t.UtilizationMem = gpu.Float(float64(util.memory))
	}
	var mem C.nvmlMemory_t
	if C.nvmlDeviceGetMemoryInfo(dev, &mem) == C.NVML_SUCCESS {
		t.MemoryTotalBytes = gpu.Uint(uint64(mem.total))
		t.MemoryUsedBytes = gpu.Uint(uint64(mem.used))
	}
	var temp C.uint
	if C.nvmlDeviceGetTemperature(dev, C.NVML_TEMPERATURE_GPU, &temp) == C.NVML_SUCCESS {
		t.TemperatureC = gpu.Float(float64(temp))
	}
	var power C.uint
	if C.nvmlDeviceGetPowerUsage(dev, &power) == C.NVML_SUCCESS {
		t.PowerWatts = gpu.Float(float64(power) / 1000) // mW -> W
	}
	var fan C.uint
	if C.nvmlDeviceGetFanSpeed(dev, &fan) == C.NVML_SUCCESS {
		t.FanPercent = gpu.Float(float64(fan))
	}
	var coreClk C.uint
	if C.nvmlDeviceGetClockInfo(dev, C.NVML_CLOCK_GRAPHICS, &coreClk) == C.NVML_SUCCESS {
		t.ClockCoreMHz = gpu.Uint(uint64(coreClk))
	}
	var memClk C.uint
	if C.nvmlDeviceGetClockInfo(dev, C.NVML_CLOCK_MEM, &memClk) == C.NVML_SUCCESS {
		t.ClockMemMHz = gpu.Uint(uint64(memClk))
	}
	return t, nil
}

func nvmlErr(op string, r C.nvmlReturn_t) error {
	return fmt.Errorf("nvml %s: %s", op, C.GoString(C.nvmlErrorString(r)))
}
