package monitoring

import (
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/exp/constraints"

	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/prometheus"
)

// Helper constants for prom Collectors
const (
	promNamespace string = "power"

	LogTopName    string = "monitoring"
	perfSubsystem string = "perf"
	msrSubsystem  string = "msr"

	logTypeKey      string = "type"
	logTypeHardware string = "hardware"
	logTypeSoftware string = "software"
	logTypeCache    string = "cache"
	logNameKey      string = "name"
)

type collectorImpl struct {
	collectFunc  func(ch chan<- prom.Metric)
	describeFunc func(ch chan<- *prom.Desc)
}

func (c collectorImpl) Collect(ch chan<- prom.Metric) {
	c.collectFunc(ch)
}

func (c collectorImpl) Describe(ch chan<- *prom.Desc) {
	c.describeFunc(ch)
}

type number interface {
	constraints.Integer | constraints.Float
}

// newPerCPUCollector is generic factory of prometheus Collectors for metrics that are CPU bound.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration
func newPerCPUCollector[T number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Cpu) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(
		metricName,
		metricDesc,
		[]string{"cpu", "core", "die", "package"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverCPUs(host, func(cpu power.Cpu, core power.Core, die power.Die, pkg power.Package) {
		if _, err := readFunc(cpu); errors.Is(err, metrics.ErrMetricMissing) {
			log.Info("Not registering collection, client will not be able to read this metric",
				"error", err.Error(), "cpu", cpu.GetID())
		} else {
			collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
				log.V(5).Info("Collecting metrics for prometheus", "cpu", cpu.GetID())
				if val, err := readFunc(cpu); err == nil {
					ch <- prom.MustNewConstMetric(
						desc,
						metricType,
						float64(val),
						strconv.Itoa(int(cpu.GetID())),
						strconv.Itoa(int(core.GetID())),
						strconv.Itoa(int(die.GetID())),
						strconv.Itoa(int(pkg.GetID())),
					)
				} else {
					log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "cpu", cpu.GetID())
				}
			})
		}
	})
	log.V(4).Info("New perCPU prometheus Collector created")

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

// newPerCoreCollector is generic factory of prometheus Collectors for metrics that are core bound.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration
func newPerCoreCollector[T number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Core) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(
		metricName,
		metricDesc,
		[]string{"core", "die", "package"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverCores(host, func(core power.Core, die power.Die, pkg power.Package) {
		if _, err := readFunc(core); errors.Is(err, metrics.ErrMetricMissing) {
			log.Info("Not registering collection, client will not be able to read this metric",
				"error", err.Error(), "core", core.GetID())
		} else {
			collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
				log.V(5).Info("Collecting metrics for prometheus", "core", core.GetID())
				if val, err := readFunc(core); err == nil {
					ch <- prom.MustNewConstMetric(
						desc,
						metricType,
						float64(val),
						strconv.Itoa(int(core.GetID())),
						strconv.Itoa(int(die.GetID())),
						strconv.Itoa(int(pkg.GetID())),
					)
				} else {
					log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "core", core.GetID())
				}
			})
		}
	})
	log.V(4).Info("New perCore prometheus Collector created")

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

// nolint: unused
// newPerDieCollector is generic factory of prometheus Collectors for metrics that are die bound.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration.
func newPerDieCollector[T number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Die) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(
		metricName,
		metricDesc,
		[]string{"die", "package"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverDies(host, func(die power.Die, pkg power.Package) {
		if _, err := readFunc(die); errors.Is(err, metrics.ErrMetricMissing) {
			log.Info("Not registering collection, client will not be able to read this metric",
				"error", err.Error(), "die", die.GetID())
		} else {
			collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
				log.V(5).Info("Collecting metrics for prometheus", "die", die.GetID())
				if val, err := readFunc(die); err == nil {
					ch <- prom.MustNewConstMetric(
						desc,
						metricType,
						float64(val),
						strconv.Itoa(int(die.GetID())),
						strconv.Itoa(int(pkg.GetID())),
					)
				} else {
					log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "die", die.GetID())
				}
			})
		}
	})
	log.V(4).Info("New perDie prometheus Collector created")

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

// newPerPackageCollector is generic factory of prometheus Collectors for metrics that are package bound.
// host is instance of power optimization library Host that exposes system topology.
// readFunc is generic function which signature corresponds to methods of southbound telemetry clients.
// log is Logger that should have all Names, KeysValues and other... already attached.
// return prometheus Collector that is ready for registration
func newPerPackageCollector[T number](metricName, metricDesc string, metricType prom.ValueType,
	host power.Host, readFunc func(power.Package) (T, error), log logr.Logger,
) prom.Collector {
	desc := prom.NewDesc(
		metricName,
		metricDesc,
		[]string{"package"},
		nil,
	)

	collectorFuncs := make([]func(ch chan<- prom.Metric), 0)
	util.IterateOverPackages(host, func(pkg power.Package) {
		if _, err := readFunc(pkg); errors.Is(err, metrics.ErrMetricMissing) {
			log.Info("Not registering collection, client will not be able to read this metric",
				"error", err.Error(), "package", pkg.GetID())
		} else {
			collectorFuncs = append(collectorFuncs, func(ch chan<- prom.Metric) {
				log.V(5).Info("Collecting metrics for prometheus", "package", pkg.GetID())
				if val, err := readFunc(pkg); err == nil {
					ch <- prom.MustNewConstMetric(
						desc,
						metricType,
						float64(val),
						strconv.Itoa(int(pkg.GetID())),
					)
				} else {
					log.V(5).Info(fmt.Sprintf("error reading metric value, err: %v", err), "package", pkg.GetID())
				}
			})
		}
	})
	log.V(4).Info("New perPackage prometheus Collector created")

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
