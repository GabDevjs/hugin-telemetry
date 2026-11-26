// Package sysfs implements a pure-Go GPU backend built on the Linux DRM and
// hwmon sysfs interfaces. It needs no vendor tooling and therefore acts as the
// universal discovery layer and the lowest-priority telemetry source.
package sysfs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

// Root is the sysfs DRM directory. It is a variable so tests can point it at a
// fixture tree.
var Root = "/sys/class/drm"

// Priority is the lowest among the built-in backends: vendor-specific backends
// should win deduplication.
const Priority = 10

type backend struct{}

// New returns the sysfs backend.
func New() gpu.Backend { return backend{} }

func init() { gpu.Register("sysfs", New) }

func (backend) Name() string  { return "sysfs" }
func (backend) Priority() int { return Priority }

func (backend) Available(context.Context) bool {
	_, err := os.Stat(Root)
	return err == nil
}

func (backend) Discover(context.Context) ([]gpu.Device, error) { return List() }

// List enumerates every DRM card exposed under [Root]. It is exported so that
// vendor backends can reuse the pure-Go discovery and filter by vendor.
func List() ([]gpu.Device, error) {
	cards, err := listCards()
	if err != nil {
		return nil, err
	}
	out := make([]gpu.Device, 0, len(cards))
	for _, card := range cards {
		d, ok := describeCard(card)
		if !ok {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func (backend) Telemetry(_ context.Context, d gpu.Device) (gpu.Telemetry, error) {
	return Sample(d)
}

// listCards returns the DRM card directory names ("card0", "card1", …) sorted
// numerically. Connector and render nodes (card0-DP-1, renderD128) are skipped.
func listCards() ([]string, error) {
	entries, err := os.ReadDir(Root)
	if err != nil {
		return nil, err
	}
	var cards []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "card") {
			continue
		}
		if strings.ContainsRune(name, '-') {
			continue // connector node, e.g. card0-DP-1
		}
		cards = append(cards, name)
	}
	sort.Slice(cards, func(i, j int) bool {
		return cardIndex(cards[i]) < cardIndex(cards[j])
	})
	return cards, nil
}

func cardIndex(card string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(card, "card"))
	return n
}

func describeCard(card string) (gpu.Device, bool) {
	devDir := filepath.Join(Root, card, "device")
	vendorID, ok := readHexID(filepath.Join(devDir, "vendor"))
	if !ok {
		return gpu.Device{}, false
	}
	vendor := gpu.VendorFromPCIID(vendorID)
	pci := pciAddress(devDir)

	d := gpu.Device{
		ID:         "sysfs:" + card,
		Vendor:     vendor,
		Name:       deviceName(devDir, vendor),
		PCIAddress: pci,
		DRMCard:    card,
		Integrated: vendor == gpu.VendorIntel,
	}
	return d, true
}

// pciAddress resolves the PCI bus address from the device symlink target,
// e.g. .../0000:01:00.0 -> "0000:01:00.0".
func pciAddress(devDir string) string {
	target, err := os.Readlink(devDir)
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

func deviceName(devDir string, vendor gpu.Vendor) string {
	if label := readString(filepath.Join(devDir, "label")); label != "" {
		return label
	}
	devID, ok := readHexID(filepath.Join(devDir, "device"))
	if !ok {
		return string(vendor)
	}
	return string(vendor) + " device " + strconv.FormatInt(int64(devID), 16)
}
