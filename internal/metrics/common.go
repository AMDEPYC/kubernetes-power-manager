package metrics

import "errors"

// ErrMetricMissing is returned when metric reader (lower level component)
// is missing and metric won't be available during process lifetime.
var ErrMetricMissing error = errors.New("metric is missing")

// Internal helper constants for logging
const (
	cpuLogKey     = "cpu"
	coreLogKey    = "core"
	dieLogKey     = "die"
	packageLogKey = "package"
)

// Enum for identifying scope of standard measurements
const (
	perCPU = iota
	perCore
	perDie
	perPackage
)
