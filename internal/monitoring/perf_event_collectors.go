package monitoring

import (
	"github.com/AMDEPYC/kubernetes-power-manager/internal/metrics"
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
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "cycles_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "instructions_total"),
			"Counter of retired instructions on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetInstructions,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "instructions_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_accesses_total"),
			"Counter of cache accesses on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetCacheAccesses,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "cache_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_misses_total"),
			"Counter of cache misses on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetCacheMisses,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "cache_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "branch_instructions_total"),
			"Counter of retired branch instructions on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBranchInstructions,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "branch_instructions_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "branch_misses_total"),
			"Counter of mispredicted branch instructions on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBranchMisses,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "branch_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "bus_cycles_total"),
			"Counter of bus cycles on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBusCycles,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "bus_cycles_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "stalled_cycles_frontend_total"),
			"Counter of stalled cycles during issue on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetStalledCyclesFrontend,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "stalled_cycles_frontend_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "stalled_cycles_backend_total"),
			"Counter of stalled cycles during retirement on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetStalledCyclesBackend,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "stalled_cycles_backend_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "ref_cycles_total"),
			"Counter of cycles not affected by frequency scaling on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetRefCycles,
			logger.WithValues(logTypeKey, logTypeHardware, logNameKey, "ref_cycles_total"),
		),
		// Type software
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "bpf_output_total"),
			"Counter of BPF outputs on specific CPU",
			prom.CounterValue,
			host,
			perfEventClient.GetBPFOutput,
			logger.WithValues(logTypeKey, logTypeSoftware, logNameKey, "bpf_output_total"),
		),
		// Type cache
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_read_accesses_total"),
			"Counter of level 1 data cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1d_read_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_read_misses_total"),
			"Counter of level 1 data cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1d_read_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_write_accesses_total"),
			"Counter of level 1 data cache total write accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCacheWriteAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1d_write_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_write_misses_total"),
			"Counter of level 1 data cache total write misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCacheWriteMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1d_write_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_prefetch_accesses_total"),
			"Counter of level 1 data cache total prefetch accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCachePrefetchAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1d_prefetch_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1d_prefetch_misses_total"),
			"Counter of level 1 data cache total prefetch misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1DCachePrefetchMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1d_prefetch_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_read_accesses_total"),
			"Counter of level 1 instruction cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1i_read_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_read_misses_total"),
			"Counter of level 1 instruction cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1i_read_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_write_accesses_total"),
			"Counter of level 1 instruction cache total write accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICacheWriteAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1i_write_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_write_misses_total"),
			"Counter of level 1 instruction cache total write misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICacheWriteMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1i_write_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_prefetch_accesses_total"),
			"Counter of level 1 instruction cache total prefetch accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICachePrefetchAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1i_prefetch_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_l1i_prefetch_misses_total"),
			"Counter of level 1 instruction cache total prefetch misses",
			prom.CounterValue,
			host,
			perfEventClient.GetL1ICachePrefetchMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_l1i_prefetch_misses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_read_accesses_total"),
			"Counter of branch prediction unit cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_bpu_read_accesses_total"),
		),
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_read_misses_total"),
			"Counter of branch prediction unit cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_bpu_read_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_write_accesses_total"),
			"Counter of branch prediction unit cache total write accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCacheWriteAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_bpu_write_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_write_misses_total"),
			"Counter of branch prediction unit cache total write misses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCacheWriteMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_bpu_write_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_prefetch_accesses_total"),
			"Counter of branch prediction unit cache total prefetch accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCachePrefetchAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_bpu_prefetch_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_bpu_prefetch_misses_total"),
			"Counter of branch prediction unit cache total prefetch misses",
			prom.CounterValue,
			host,
			perfEventClient.GetBPUCachePrefetchMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_bpu_prefetch_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_node_read_accesses_total"),
			"Counter of node cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetNodeCacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_node_read_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_node_read_misses_total"),
			"Counter of node cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetNodeCacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_node_read_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_node_write_accesses_total"),
			"Counter of node cache total write accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetNodeCacheWriteAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_node_write_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_node_write_misses_total"),
			"Counter of node cache total write misses",
			prom.CounterValue,
			host,
			perfEventClient.GetNodeCacheWriteMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_node_write_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_node_prefetch_accesses_total"),
			"Counter of node cache total prefetch accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetNodeCachePrefetchAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_node_prefetch_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerCPUCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_node_prefetch_misses_total"),
			"Counter of node cache total prefetch misses",
			prom.CounterValue,
			host,
			perfEventClient.GetNodeCachePrefetchMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_node_prefetch_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerDieCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_ll_read_accesses_total"),
			"Counter of last level cache total read accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetLLCacheReadAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_ll_read_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerDieCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_ll_read_misses_total"),
			"Counter of last level cache total read misses",
			prom.CounterValue,
			host,
			perfEventClient.GetLLCacheReadMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_ll_read_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerDieCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_ll_write_accesses_total"),
			"Counter of last level cache total write accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetLLCacheWriteAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_ll_write_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerDieCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_ll_write_misses_total"),
			"Counter of last level cache total write misses",
			prom.CounterValue,
			host,
			perfEventClient.GetLLCacheWriteMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_ll_write_misses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerDieCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_ll_prefetch_accesses_total"),
			"Counter of last level cache total prefetch accesses",
			prom.CounterValue,
			host,
			perfEventClient.GetLLCachePrefetchAccesses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_ll_prefetch_accesses_total"),
		),
		// not verified, not supported by linux kernel as of 2024/11
		newPerDieCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "cache_ll_prefetch_misses_total"),
			"Counter of last level cache total prefetch misses",
			prom.CounterValue,
			host,
			perfEventClient.GetLLCachePrefetchMisses,
			logger.WithValues(logTypeKey, logTypeCache, logNameKey, "cache_ll_prefetch_misses_total"),
		),
		newPerPackageCollector(
			prom.BuildFQName(promNamespace, perfSubsystem, "package_energy_consumption_joules_total"),
			"Counter of total package energy consumption in joules",
			prom.CounterValue,
			host,
			perfEventClient.GetPackageEnergyConsumption,
			logger.WithValues(logTypeKey, logTypePower, logNameKey, "package_energy_consumption_joules_total"),
		),
	)
}
