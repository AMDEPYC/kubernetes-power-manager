package scaling

import (
	"github.com/intel/power-optimization-library/pkg/power"
)

type CPUScalingUpdater interface {
	Update(opts *CPUScalingOpts)
}

type cpuScalingUpdaterImpl struct {
	powerLibrary *power.Host
}

func NewCPUScalingUpdater(powerLib *power.Host) CPUScalingUpdater {
	updater := &cpuScalingUpdaterImpl{
		powerLibrary: powerLib,
	}

	return updater
}

func (u *cpuScalingUpdaterImpl) Update(opts *CPUScalingOpts) {
	// TODO: implement perodic scaling actions
}
