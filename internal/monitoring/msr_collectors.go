package monitoring

import (
	"github.com/AMDEPYC/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/prometheus"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func RegisterMSRCollectors(msrClient *metrics.MSRClient, host power.Host, logger logr.Logger) {
	logger = logger.WithName(msrSubsystem)

	ctrlMetrics.Registry.MustRegister(
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, msrSubsystem, "c0_residency_percent"),
			"Gauge of CPU residency in C0",
			prom.GaugeValue,
			host,
			msrClient.GetC0ResidencyPercent,
			logger.WithValues(logNameKey, "c0_residency_percent"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, msrSubsystem, "cx_residency_percent"),
			"Gauge of CPU residency in C-states other than C0",
			prom.GaugeValue,
			host,
			msrClient.GetCxResidencyPercent,
			logger.WithValues(logNameKey, "cx_residency_percent"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, msrSubsystem, "package_energy_consumption_joules_total"),
			"Counter of total package energy consumption in joules",
			prom.CounterValue,
			host,
			msrClient.GetPackageEnergyConsumption,
			logger.WithValues(logNameKey, "package_energy_consumption_joules_total"),
		),
		newPerCoreCollector(
			prom.BuildFQName(promNamespace, msrSubsystem, "core_energy_consumption_joules_total"),
			"Counter of total core energy consumption in joules",
			prom.CounterValue,
			host,
			msrClient.GetCoreEnergyConsumption,
			logger.WithValues(logNameKey, "core_energy_consumption_joules_total"),
		),
	)
}
