package scaling

import "time"

const FrequencyNotYetSet int = -1

type CPUScalingOpts struct {
	CPUID                      uint
	SamplePeriod               time.Duration
	CooldownPeriod             time.Duration
	TargetBusyness             int
	AllowedBusynessDifference  int
	AllowedFrequencyDifference int
	HWMaxFrequency             int
	HWMinFrequency             int
	CurrentFrequency           int
	FallbackFreq               int
}
