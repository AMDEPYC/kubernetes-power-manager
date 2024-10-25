package scaling

import "time"

type CPUScalingOpts struct {
	CPUID        uint
	SamplePeriod time.Duration
}
