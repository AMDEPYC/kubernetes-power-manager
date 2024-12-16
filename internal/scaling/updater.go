package scaling

import (
	"time"

	"github.com/intel/kubernetes-power-manager/internal/metrics"
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
		_ = setCPUFrequency(opts.CPUID, uint(opts.FallbackFreq))
		opts.CurrentFrequency = opts.FallbackFreq
		return opts.SamplePeriod
	}

	if currentBusyness >= opts.TargetBusyness-opts.AllowedBusynessDifference &&
		currentBusyness <= opts.TargetBusyness+opts.AllowedBusynessDifference {
		return opts.SamplePeriod
	}

	nextFrequency := opts.HWMinFrequency + currentBusyness*(opts.HWMaxFrequency-opts.HWMinFrequency)/opts.TargetBusyness

	if nextFrequency > opts.HWMaxFrequency {
		nextFrequency = opts.HWMaxFrequency
	}

	if opts.CurrentFrequency != FrequencyNotYetSet &&
		nextFrequency >= opts.CurrentFrequency-opts.AllowedFrequencyDifference &&
		nextFrequency <= opts.CurrentFrequency+opts.AllowedFrequencyDifference {
		return opts.SamplePeriod
	}

	_ = setCPUFrequency(opts.CPUID, uint(nextFrequency))
	opts.CurrentFrequency = nextFrequency
	return opts.CooldownPeriod
}
