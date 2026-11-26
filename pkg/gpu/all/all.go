// Package all registers every built-in GPU backend as a side effect of being
// imported. Import it for its side effects to enable the default set:
//
//	import _ "github.com/GabDevjs/hugin-telemetry/pkg/gpu/all"
//
// The cgo backends (nvidia-nvml, amd-rocm) register themselves only when their
// build tags are active (-tags nvml, -tags rocm).
package all

import (
	_ "github.com/GabDevjs/hugin-telemetry/pkg/gpu/amd"
	_ "github.com/GabDevjs/hugin-telemetry/pkg/gpu/intel"
	_ "github.com/GabDevjs/hugin-telemetry/pkg/gpu/nvidia"
	_ "github.com/GabDevjs/hugin-telemetry/pkg/gpu/sysfs"
)
