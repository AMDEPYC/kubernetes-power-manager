package scaling

import (
	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"
)

type CPUScalingUpdater interface {
	Update(opts *CPUScalingOpts)
}

type cpuScalingUpdaterImpl struct {
	powerLibrary *power.Host
	dpdkClient   metrics.DPDKTelemetryClient
}

func NewCPUScalingUpdater(powerLib *power.Host, dpdkClient metrics.DPDKTelemetryClient) CPUScalingUpdater {
	updater := &cpuScalingUpdaterImpl{
		powerLibrary: powerLib,
		dpdkClient:   dpdkClient,
	}

	return updater
}

func (u *cpuScalingUpdaterImpl) Update(opts *CPUScalingOpts) {
	// TODO: implement perodic scaling actions
}
