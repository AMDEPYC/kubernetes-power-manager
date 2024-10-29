package monitoring

import (
	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"

	"github.com/go-logr/logr"
	prom "github.com/prometheus/client_golang/prometheus"
	ctrlMetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func RegisterPerfEventCollectors(perfEventClient *metrics.PerfEventClient, host power.Host, logger logr.Logger) {
	logger = logger.WithName(perfSubsystem)

	// Type hardware
	ctrlMetrics.Registry.MustRegister(
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cycles_total"),
			"Counter of cycles on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetCycles,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "cycles_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "instructions_total"),
			"Counter of retired instructions on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetInstructions,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "instructions_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_accesses_total"),
			"Counter of cache accesses on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetCacheAccesses,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "cache_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_misses_total"),
			"Counter of cache misses on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetCacheMisses,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "cache_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "branch_instructions_total"),
			"Counter of retired branch instructions on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBranchInstructions,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "branch_instructions_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "branch_misses_total"),
			"Counter of mispredicted branch instructions on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBranchMisses,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "branch_misses_total"),
		),
		// TODO: not working in the lab, to be clarified
		// newPerCPUCollector(
		// 	prom.BuildFQName(promNamespace, perfSubsystem, "bus_cycles_total"),
		// 	"Counter of bus cycles on specific CPU",
		// 	prom.CounterValue,
		// 	host,
		// 	perfEventClient.GetBusCycles,
		// 	logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "bus_cycles_total"),
		// ),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "stalled_cycles_frontend_total"),
			"Counter of stalled cycles during issue on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetStalledCyclesFrontend,
			logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "stalled_cycles_frontend_total"),
		),
		// TODO: not working in the lab, to be clarified
		// newPerCPUCollector(
		// 	prom.BuildFQName(promNamespace, perfSubsystem, "stalled_cycles_backend_total"),
		// 	"Counter of stalled cycles during retirement on specific CPU",
		// 	prom.CounterValue,
		// 	host,
		// 	perfEventClient.GetStalledCyclesBackend,
		// 	logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "stalled_cycles_backend_total"),
		// ),
		// newPerCPUCollector(
		// 	prom.BuildFQName(promNamespace, perfSubsystem, "ref_cycles_total"),
		// 	"Counter of cycles not affected by frequency scaling on specific CPU",
		// 	prom.CounterValue,
		// 	host,
		// 	perfEventClient.GetRefCycles,
		// 	logger.WithValues(logTypeKey, logTypeHardware, logTypeName, "ref_cycles_total"),
		// ),
		// Type software
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "bpf_output_total"),
			"Counter of BPF outputs on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBPFOutput,
			logger.WithValues(logTypeKey, logTypeSoftware, logTypeName, "bpf_output_total"),
		),
		// Type cache
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_read_accesses_total"),
			"Counter of level 1 data cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_l1d_read_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_read_misses_total"),
			"Counter of level 1 data cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_l1d_read_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_prefetch_accesses_total"),
			"Counter of level 1 data cache total prefetch accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCachePrefetchAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_l1d_prefetch_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_read_accesses_total"),
			"Counter of level 1 instruction cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_l1i_read_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_read_misses_total"),
			"Counter of level 1 instruction cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_l1i_read_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_read_accesses_total"),
			"Counter of branch prediction unit cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_bpu_read_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_read_misses_total"),
			"Counter of branch prediction unit cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logTypeName, "cache_bpu_read_misses_total"),
		),
	)
}
