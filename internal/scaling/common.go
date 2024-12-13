package scaling

import "time"

type CPUScalingOpts struct {
	CPUID                      uint
	SamplePeriod               time.Duration
	CooldownPeriod             time.Duration
	TargetBusyness             int
	AllowedBusynessDifference  int
	AllowedFrequencyDifference int
	FallbackFreq               int
}
