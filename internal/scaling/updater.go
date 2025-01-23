package scaling

import (
	"time"

	"github.com/AMDEPYC/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"
)

type CPUScalingUpdater interface {
	Update(opts *CPUScalingOpts) time.Duration
}

type cpuScalingUpdaterImpl struct {
	dpdkClient metrics.DPDKTelemetryClient
}

func NewCPUScalingUpdater(powerLib *power.Host, dpdkClient metrics.DPDKTelemetryClient) CPUScalingUpdater {
	updater := &cpuScalingUpdaterImpl{
		dpdkClient: dpdkClient,
	}

	return updater
}

func (u *cpuScalingUpdaterImpl) Update(opts *CPUScalingOpts) time.Duration {
	currentBusyness, err := u.dpdkClient.GetBusynessPercent(opts.CPUID)
	if err != nil {
		if frequencyInAllowedDifference(opts.FallbackFreq, opts) {
			return opts.SamplePeriod
		}

		_ = setCPUFrequency(opts.CPUID, uint(opts.FallbackFreq))
		opts.CurrentTargetFrequency = opts.FallbackFreq
		return opts.SamplePeriod
	}

	if currentBusyness >= opts.TargetBusyness-opts.AllowedBusynessDifference &&
		currentBusyness <= opts.TargetBusyness+opts.AllowedBusynessDifference {
		return opts.SamplePeriod
	}

	currentFrequencyUint, err := getCPUFrequency(opts.CPUID)
	if err != nil {
		if frequencyInAllowedDifference(opts.FallbackFreq, opts) {
			return opts.SamplePeriod
		}

		_ = setCPUFrequency(opts.CPUID, uint(opts.FallbackFreq))
		opts.CurrentTargetFrequency = opts.FallbackFreq
		return opts.SamplePeriod
	}
	currentFrequency := int(currentFrequencyUint)

	nextFrequencyFloat :=
		float64(currentFrequency) * (1.0 + (float64(currentBusyness)/float64(opts.TargetBusyness)-1.0)*opts.ScaleFactor)
	nextFrequency := int(nextFrequencyFloat)

	if nextFrequency < opts.HWMinFrequency {
		nextFrequency = opts.HWMinFrequency
	}
	if nextFrequency > opts.HWMaxFrequency {
		nextFrequency = opts.HWMaxFrequency
	}

	if frequencyInAllowedDifference(nextFrequency, opts) {
		return opts.SamplePeriod
	}

	_ = setCPUFrequency(opts.CPUID, uint(nextFrequency))
	opts.CurrentTargetFrequency = nextFrequency

	return opts.CooldownPeriod
}

func frequencyInAllowedDifference(frequency int, opts *CPUScalingOpts) bool {
	if opts.CurrentTargetFrequency != FrequencyNotYetSet &&
		frequency >= opts.CurrentTargetFrequency-opts.AllowedFrequencyDifference &&
		frequency <= opts.CurrentTargetFrequency+opts.AllowedFrequencyDifference {
		return true
	}

	return false
}
