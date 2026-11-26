package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/GabDevjs/hugin-telemetry/pkg/gpu"
)

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func renderDevices(w io.Writer, devs []gpu.Device) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "IDX\tVENDOR\tNAME\tPCI\tDRM\tTYPE\tBACKEND")
	for _, d := range devs {
		typ := "discrete"
		if d.Integrated {
			typ = "integrated"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Index, d.Vendor, d.Name, dash(d.PCIAddress), dash(d.DRMCard), typ, d.Backend)
	}
	_ = tw.Flush()
}

func renderTelemetry(w io.Writer, devs []gpu.Device, samples map[string]gpu.Telemetry) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "IDX\tNAME\tUTIL%\tMEM\tTEMP\tPOWER\tCORE\tMEMCLK")
	for _, d := range devs {
		t := samples[d.ID]
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Index, d.Name,
			pctStr(t.UtilizationGPU),
			memStr(t.MemoryUsedBytes, t.MemoryTotalBytes),
			tempStr(t.TemperatureC),
			powerStr(t.PowerWatts),
			clockStr(t.ClockCoreMHz),
			clockStr(t.ClockMemMHz),
		)
	}
	_ = tw.Flush()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func pctStr(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", *v)
}

func tempStr(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f°C", *v)
}

func powerStr(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0fW", *v)
}

func clockStr(v *uint64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%dMHz", *v)
}

func memStr(used, total *uint64) string {
	switch {
	case used != nil && total != nil:
		return fmt.Sprintf("%s/%s", humanBytes(*used), humanBytes(*total))
	case total != nil:
		return "?/" + humanBytes(*total)
	default:
		return "-"
	}
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
