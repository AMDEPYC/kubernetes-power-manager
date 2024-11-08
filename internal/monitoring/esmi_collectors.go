package monitoring

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/prometheus"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func RegisterESMICollectors(esmiClient *metrics.ESMIClient, host power.Host, logger logr.Logger) {
	logger = logger.WithName(esmiSubsystem)

	ctrlMetrics.Registry.MustRegister(
		newPerCoreCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "core_energy_consumption_joules_total"),
			"Counter of total core energy consumption in joules.",
			prom.CounterValue,
			host,
			esmiClient.GetCoreEnergy,
			logger.WithValues(logNameKey, "core_energy_consumption_joules_total"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_energy_consumption_joules_total"),
			"Counter of total core energy consumption in joules.",
			prom.CounterValue,
			host,
			esmiClient.GetPackageEnergy,
			logger.WithValues(logNameKey, "core_energy_consumption_joules_total"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "data_fabric_clock_megahertz"),
			"Gauge of data fabric clock in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetDataFabricClock,
			logger.WithValues(logNameKey, "data_fabric_clock_megahertz"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "memory_clock_megahertz"),
			"Gauge of memory clock in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetMemoryClock,
			logger.WithValues(logNameKey, "memory_clock_megahertz"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "core_clock_throttle_limit_megahertz"),
			"Gauge of package core clock throttle limit in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetCoreClockThrottleLimit,
			logger.WithValues(logNameKey, "core_clock_throttle_limit_megahertz"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_frequency_limit_megahertz"),
			"Gauge of package frequency limit in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackageFreqLimit,
			logger.WithValues(logNameKey, "package_frequency_limit"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_min_frequency_megahertz"),
			"Gauge of package minimum frequency in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackageMinFreq,
			logger.WithValues(logNameKey, "package_min_frequency_megahertz"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_max_frequency_megahertz"),
			"Gauge of package maximum frequency in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackageMaxFreq,
			logger.WithValues(logNameKey, "package_max_frequency_megahertz"),
		),
		newPerCoreCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "core_frequency_limit_megahertz"),
			"Gauge of core frequency limit in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetCoreFreqLimit,
			logger.WithValues(logNameKey, "core_frequency_limit_megahertz"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "rail_frequency_limit"),
			"Gauge of package rail frequency limit policy. Values: 1 = all cores on both rails "+
				"have same frequency limit, 0 = each rail has different independent frequency limit.",
			prom.GaugeValue,
			host,
			esmiClient.GetRailFreqLimitPolicy,
			logger.WithValues(logNameKey, "rail_frequency_limit"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "df_cstate_enabling_control"),
			"Gauge of package DF C-state enabling control. Values: 1 = DFC enabled, 0 = DFC disabled.",
			prom.GaugeValue,
			host,
			esmiClient.GetDFCStateEnablingControl,
			logger.WithValues(logNameKey, "df_cstate_enabling_control"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_power_watts"),
			"Gauge of package power in watts.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackagePower,
			logger.WithValues(logNameKey, "package_power_watts"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_power_cap_watts"),
			"Gauge of package power cap in watts.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackagePowerCap,
			logger.WithValues(logNameKey, "package_power_cap_watts"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_power_max_cap_watts"),
			"Gauge of package power max cap in watts.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackagePowerMaxCap,
			logger.WithValues(logNameKey, "package_power_max_cap_watts"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "power_efficiency_mode"),
			"Gauge of package power efficiency mode. Values: 0 = high performance mode, "+
				"1 = power efficient mode, 2 = IO performance mode, 3 = balanced memory performance mode, "+
				"4 = balanced core performance mode, 5 = balanced core and memory performance mode.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackagePowerEfficiencyMode,
			logger.WithValues(logNameKey, "power_efficiency_mode"),
		),
		newPerCoreCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "core_boost_limit_megahertz"),
			"Gauge of core frequency boost limit in MHz.",
			prom.GaugeValue,
			host,
			esmiClient.GetCoreBoostLimit,
			logger.WithValues(logNameKey, "core_boost_limit_megahertz"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "c0_residency_percent"),
			"Gauge of package residency in C0 in percents.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackageC0Residency,
			logger.WithValues(logNameKey, "c0_residency_percent"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "package_temperature_celsius"),
			"Gauge of package temperature in degree Celsius.",
			prom.GaugeValue,
			host,
			esmiClient.GetPackageTemp,
			logger.WithValues(logNameKey, "package_temperature_celsius"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "ddr_bandwidth_utilization_gigabytes_per_second"),
			"Gauge of DDR bandwidth utilization in GB/s.",
			prom.GaugeValue,
			esmiClient.GetDDRBandwidthUtil,
			logger.WithValues(logNameKey, "ddr_bandwidth_utilization_gigabytes_per_second"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "ddr_bandwidth_utilization_percent"),
			"Gauge of DDR bandwidth utilization in percent of maximum bandwidth.",
			prom.GaugeValue,
			esmiClient.GetDDRBandwidthUtilPercent,
			logger.WithValues(logNameKey, "ddr_bandwidth_utilization_percent"),
		),
		newPackageDimmCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "dimm_power_watts"),
			"Gauge of DIMM power in watts.",
			prom.GaugeValue,
			host,
			esmiClient.GetDIMMPower,
			logger.WithValues(logNameKey, "dimm_power_watts"),
		),
		newPackageDimmCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "dimm_temperature_celsius"),
			"Gauge of DIMM temperature in degree Celsius.",
			prom.GaugeValue,
			host,
			esmiClient.GetDIMMTemp,
			logger.WithValues(logNameKey, "dimm_temperature_celsius"),
		),
		newPackageNBIOCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "lclk_dpm_min_level"),
			"Gauge of minimum LCLK DPM level. Values: either 0 or 1",
			prom.GaugeValue,
			host,
			esmiClient.GetLCLKDPMMinLevel,
			logger.WithValues(logNameKey, "lclk_dpm_min_level"),
		),
		newPackageNBIOCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "lclk_dpm_max_level"),
			"Gauge of maximum LCLK DPM level. Values: either 0 or 1",
			prom.GaugeValue,
			host,
			esmiClient.GetLCLKDPMMaxLevel,
			logger.WithValues(logNameKey, "lclk_dpm_max_level"),
		),
		newPackageIOLinkCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "io_link_bandwidth_utilization_megabits_per_second"),
			"Gauge of IO link bandwidth utilization in Mb/s.",
			prom.GaugeValue,
			host,
			esmiClient.GetIOLinkBandwidthUtil,
			logger.WithValues(logNameKey, "io_link_bandwidth_utilization_megabits_per_second"),
		),
		newPerIOLinkCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "xgmi_aggregate_bandwidth_utilization_megabits_per_second"),
			"Gauge of xGMI aggregate bandwidth utilization in Mb/s.",
			prom.GaugeValue,
			esmiClient.GetxGMIAggregateBandwidthUtil,
			logger.WithValues(logNameKey, "xgmi_aggregate_bandwidth_utilization_megabits_per_second"),
		),
		newPerIOLinkCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "xgmi_read_bandwidth_utilization_megabits_per_second"),
			"Gauge of xGMI read bandwidth utilization in Mb/s.",
			prom.GaugeValue,
			esmiClient.GetxGMIReadBandwidthUtil,
			logger.WithValues(logNameKey, "xgmi_read_bandwidth_utilization_megabits_per_second"),
		),
		newPerIOLinkCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "xgmi_write_bandwidth_utilization_megabits_per_second"),
			"Gauge of xGMI write bandwidth utilization in Mb/s.",
			prom.GaugeValue,
			esmiClient.GetxGMIWriteBandwidthUtil,
			logger.WithValues(logNameKey, "xgmi_write_bandwidth_utilization_megabits_per_second"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "processor_family"),
			"Gauge of processor family.",
			prom.GaugeValue,
			esmiClient.GetCPUFamily,
			logger.WithValues(logNameKey, "processor_family"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "processor_model"),
			"Gauge of processor model.",
			prom.GaugeValue,
			esmiClient.GetCPUModel,
			logger.WithValues(logNameKey, "processor_model"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "cpus_per_core"),
			"Gauge of CPUs per core.",
			prom.GaugeValue,
			esmiClient.GetNumberOfCPUsPerCore,
			logger.WithValues(logNameKey, "cpus_per_core"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "cpus"),
			"Gauge of total number of CPUs in the system.",
			prom.GaugeValue,
			esmiClient.GetNumberOfCPUs,
			logger.WithValues(logNameKey, "cpus"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "packages"),
			"Gauge of total number of packages in the system.",
			prom.GaugeValue,
			esmiClient.GetNumberOfPackages,
			logger.WithValues(logNameKey, "packages"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "smu_firmware_major_version"),
			"Gauge of SMU firmware major version.",
			prom.GaugeValue,
			esmiClient.GetSMUFWMajorVersion,
			logger.WithValues(logNameKey, "smu_firmware_major_version"),
		),
		newPerSystemCollector(
			prom.BuildFQName(promNamespace, esmiSubsystem, "smu_firmware_minor_version"),
			"Gauge of SMU firmware minor version.",
			prom.GaugeValue,
			esmiClient.GetSMUFWMinorVersion,
			logger.WithValues(logNameKey, "smu_firmware_minor_version"),
		),
	)
}

// Helpers with hardcoded e_smi lib functions parameters
var (
	supportedUMCNum  int = 12
	dimm0StartAddr   int = 0x80
	dimm1StartAddr   int = 0x90
	supportedNBIOIDs     = []int{0, 1, 2, 3}
	supportedIOLinks     = []string{
		"P0", "P1", "P2", "P3", "P4",
		"G0", "G1", "G2", "G3", "G4", "G5", "G6", "G7",
	}
)

// newPackageDimmCollector is generic factory of prometheus Collectors for metrics that are taking package
// and DIMM address as parameters.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration.
func newPackageDimmCollector[T, E number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Package, E) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(metricName,
		metricDesc,
		[]string{"package", "umc", "dimm"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverPackages(host, func(pkg power.Package) {
		for umc := range supportedUMCNum {
			for dimm, dimmAddr := range []int{dimm0StartAddr + umc, dimm1StartAddr + umc} {
				if _, err := readFunc(pkg, E(dimmAddr)); errors.Is(err, metrics.ErrMetricMissing) {
					log.Info("Not registering collection, client will not be able to read this metric", "error",
						err.Error(), "package", pkg.GetID(), "DIMM address", dimmAddr, "DIMM", dimm, "UMC", umc)
				} else {
					collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
						log.V(5).Info("Collecting metrics for reporting", "package", pkg.GetID(),
							"DIMM address", dimmAddr, "DIMM", dimm, "UMC", umc)
						if val, err := readFunc(pkg, E(dimmAddr)); err == nil {
							ch <- prom.MustNewConstMetric(
								desc,
								metricType,
								float64(val),
								strconv.Itoa(int(pkg.GetID())),
								strconv.Itoa(umc),
								strconv.Itoa(dimm),
							)
						} else {
							log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "package",
								pkg.GetID(), "DIMM address", dimmAddr, dimmAddr, "DIMM", dimm, "UMC", umc)
						}
					})
				}
			}
		}
	})
	log.V(4).Info("New Package & DIMM address prometheus Collector created")

	return collectorImpl{
		describeFunc: func(ch chan<- *prom.Desc) {
			ch <- desc
		},
		collectFunc: func(ch chan<- prom.Metric) {
			for _, collectFunc := range collectorFuncs {
				collectFunc(ch)
			}
		},
	}

}

// newPackageNBIOCollector is generic factory of prometheus Collectors for metrics that are taking package
// and NBIO as parameters.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration.
func newPackageNBIOCollector[T, E number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Package, E) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(metricName,
		metricDesc,
		[]string{"package", "nbio"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverPackages(host, func(pkg power.Package) {
		for _, nbioID := range supportedNBIOIDs {
			if _, err := readFunc(pkg, E(nbioID)); errors.Is(err, metrics.ErrMetricMissing) {
				log.Info("Not registering collection, client will not be able to read this metric",
					"error", err.Error(), "package", pkg.GetID(), "NBIO ID", nbioID)
			} else {
				collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
					log.V(5).Info("Collecting metrics for reporting", "package", pkg.GetID(), "NBIO ID", nbioID)
					if val, err := readFunc(pkg, E(nbioID)); err == nil {
						ch <- prom.MustNewConstMetric(
							desc,
							metricType,
							float64(val),
							strconv.Itoa(int(pkg.GetID())),
							strconv.Itoa(nbioID),
						)
					} else {
						log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "package", pkg.GetID(),
							"NBIO ID", nbioID)
					}
				})
			}
		}
	})
	log.V(4).Info("New Package & NBIO prometheus Collector created")

	return collectorImpl{
		describeFunc: func(ch chan<- *prom.Desc) {
			ch <- desc
		},
		collectFunc: func(ch chan<- prom.Metric) {
			for _, collectFunc := range collectorFuncs {
				collectFunc(ch)
			}
		},
	}
}

// newPackageIOLinkCollector is generic factory of prometheus Collectors for metrics
// that are taking package and IO link as parameters.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration.
func newPackageIOLinkCollector[T number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Package, string) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(metricName,
		metricDesc,
		[]string{"package", "io_link"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverPackages(host, func(pkg power.Package) {
		for _, ioLink := range supportedIOLinks {
			if _, err := readFunc(pkg, ioLink); errors.Is(err, metrics.ErrMetricMissing) {
				log.Info("Not registering collection, client will not be able to read this metric",
					"error", err.Error(), "package", pkg.GetID(), "IO link", ioLink)
			} else {
				collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
					log.V(5).Info("Collecting metrics for reporting", "package", pkg.GetID(), "IO link", ioLink)
					if val, err := readFunc(pkg, ioLink); err == nil {
						ch <- prom.MustNewConstMetric(
							desc,
							metricType,
							float64(val),
							strconv.Itoa(int(pkg.GetID())),
							ioLink,
						)
					} else {
						log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err),
							"package", pkg.GetID(), "IO link", ioLink)
					}
				})
			}
		}
	})
	log.V(4).Info("New Package & IO Link prometheus Collector created")

	return collectorImpl{
		describeFunc: func(ch chan<- *prom.Desc) {
			ch <- desc
		},
		collectFunc: func(ch chan<- prom.Metric) {
			for _, collectFunc := range collectorFuncs {
				collectFunc(ch)
			}
		},
	}
}

// newPerIOLinkCollector is generic factory of prometheus Collectors for metrics that are IO link bound.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration.
func newPerIOLinkCollector[T number](metricName, metricDesc string, metricType prom.ValueType,
	readFunc func(string) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(
		metricName,
		metricDesc,
		[]string{"io_link"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	for _, ioLink := range supportedIOLinks {
		if _, err := readFunc(ioLink); errors.Is(err, metrics.ErrMetricMissing) {
			log.Info("Not registering collection, client will not be able to read this metric",
				"error", err.Error(), "IO link", ioLink)
		} else {
			collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
				log.V(5).Info("Collecting metrics for reporting", "IO link", ioLink)
				if val, err := readFunc(ioLink); err == nil {
					ch <- prom.MustNewConstMetric(
						desc,
						metricType,
						float64(val),
						ioLink,
					)
				} else {
					log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "IO link", ioLink)
				}
			})
		}
	}
	log.V(4).Info("New perIOLink prometheus Collector created")

	return collectorImpl{
		describeFunc: func(ch chan<- *prom.Desc) {
			ch <- desc
		},
		collectFunc: func(ch chan<- prom.Metric) {
			for _, collectFunc := range collectorFuncs {
				collectFunc(ch)
			}
		},
	}
}
