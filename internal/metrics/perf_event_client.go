package metrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/AMDEPYC/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"
	"golang.org/x/sys/unix"
)

// Func definitions for unit testing
var (
	newDefaultPerfEventReaderFunc func(int, int, int) (perfEventReader, error) = newDefaultPerfEventReader
)

// Helper map for iterations, new supported hardware measurements must be added here
var hwPerfEvents = map[int][]int{
	perCPU: {
		unix.PERF_COUNT_HW_CPU_CYCLES,
		unix.PERF_COUNT_HW_INSTRUCTIONS,
		unix.PERF_COUNT_HW_CACHE_REFERENCES,
		unix.PERF_COUNT_HW_CACHE_MISSES,
		unix.PERF_COUNT_HW_BRANCH_INSTRUCTIONS,
		unix.PERF_COUNT_HW_BRANCH_MISSES,
		// not verified, not supported by linux kernel as of 2024/11
		unix.PERF_COUNT_HW_BUS_CYCLES,
		unix.PERF_COUNT_HW_STALLED_CYCLES_FRONTEND,
		// not verified, not supported by linux kernel as of 2024/11
		unix.PERF_COUNT_HW_STALLED_CYCLES_BACKEND,
		// not verified, not supported by linux kernel as of 2024/11
		unix.PERF_COUNT_HW_REF_CPU_CYCLES,
	},
}

// Helper map for iterations, new supported software measurements must be added here.
var swPerfEvents = map[int][]int{
	perCPU: {
		unix.PERF_COUNT_SW_BPF_OUTPUT,
	},
}

// nolint: unused
// Cache perf events helper consts.
// See PERF_TYPE_HW_CACHE section in manpage https://man7.org/linux/man-pages/man2/perf_event_open.2.html for more info.
const (
	cacheL1DReadAccesses     = unix.PERF_COUNT_HW_CACHE_L1D | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheL1DReadMisses       = unix.PERF_COUNT_HW_CACHE_L1D | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheL1DWriteAccesses    = unix.PERF_COUNT_HW_CACHE_L1D | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheL1DWriteMisses      = unix.PERF_COUNT_HW_CACHE_L1D | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheL1DPrefetchAccesses = unix.PERF_COUNT_HW_CACHE_L1D | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheL1DPrefetchMisses   = unix.PERF_COUNT_HW_CACHE_L1D | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16

	cacheL1IReadAccesses     = unix.PERF_COUNT_HW_CACHE_L1I | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheL1IReadMisses       = unix.PERF_COUNT_HW_CACHE_L1I | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheL1IWriteAccesses    = unix.PERF_COUNT_HW_CACHE_L1I | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheL1IWriteMisses      = unix.PERF_COUNT_HW_CACHE_L1I | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheL1IPrefetchAccesses = unix.PERF_COUNT_HW_CACHE_L1I | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheL1IPrefetchMisses   = unix.PERF_COUNT_HW_CACHE_L1I | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16

	cacheBPUReadAccesses     = unix.PERF_COUNT_HW_CACHE_BPU | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheBPUReadMisses       = unix.PERF_COUNT_HW_CACHE_BPU | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheBPUWriteAccesses    = unix.PERF_COUNT_HW_CACHE_BPU | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheBPUWriteMisses      = unix.PERF_COUNT_HW_CACHE_BPU | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheBPUPrefetchAccesses = unix.PERF_COUNT_HW_CACHE_BPU | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheBPUPrefetchMisses   = unix.PERF_COUNT_HW_CACHE_BPU | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16

	cacheNodeReadAccesses     = unix.PERF_COUNT_HW_CACHE_NODE | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheNodeReadMisses       = unix.PERF_COUNT_HW_CACHE_NODE | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheNodeWriteAccesses    = unix.PERF_COUNT_HW_CACHE_NODE | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheNodeWriteMisses      = unix.PERF_COUNT_HW_CACHE_NODE | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheNodePrefetchAccesses = unix.PERF_COUNT_HW_CACHE_NODE | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheNodePrefetchMisses   = unix.PERF_COUNT_HW_CACHE_NODE | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16

	cacheLLReadAccesses     = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheLLReadMisses       = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_READ<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheLLWriteAccesses    = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheLLWriteMisses      = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_WRITE<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
	cacheLLPrefetchAccesses = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_ACCESS<<16
	cacheLLPrefetchMisses   = unix.PERF_COUNT_HW_CACHE_LL | unix.PERF_COUNT_HW_CACHE_OP_PREFETCH<<8 | unix.PERF_COUNT_HW_CACHE_RESULT_MISS<<16
)

// Helper map for iterations, new supported cache measurements must be added here.
var cachePerfEvents = map[int][]int{
	perCPU: {
		cacheL1DReadAccesses,
		cacheL1DReadMisses,
		cacheL1DWriteAccesses, // not verified, not supported by linux kernel as of 2024/11
		cacheL1DWriteMisses,   // not verified, not supported by linux kernel as of 2024/11
		cacheL1DPrefetchAccesses,
		cacheL1DPrefetchMisses, // not verified, not supported by linux kernel as of 2024/11

		cacheL1IReadAccesses,
		cacheL1IReadMisses,
		cacheL1IWriteAccesses,    // not verified, not supported by linux kernel as of 2024/11
		cacheL1IWriteMisses,      // not verified, not supported by linux kernel as of 2024/11
		cacheL1IPrefetchAccesses, // not verified, not supported by linux kernel as of 2024/11
		cacheL1IPrefetchMisses,   // not verified, not supported by linux kernel as of 2024/11

		cacheBPUReadAccesses,
		cacheBPUReadMisses,
		cacheBPUWriteAccesses,    // not verified, not supported by linux kernel as of 2024/11
		cacheBPUWriteMisses,      // not verified, not supported by linux kernel as of 2024/11
		cacheBPUPrefetchAccesses, // not verified, not supported by linux kernel as of 2024/11
		cacheBPUPrefetchMisses,   // not verified, not supported by linux kernel as of 2024/11

		cacheNodeReadAccesses,     // not verified, not supported by linux kernel as of 2024/11
		cacheNodeReadMisses,       // not verified, not supported by linux kernel as of 2024/11
		cacheNodeWriteAccesses,    // not verified, not supported by linux kernel as of 2024/11
		cacheNodeWriteMisses,      // not verified, not supported by linux kernel as of 2024/11
		cacheNodePrefetchAccesses, // not verified, not supported by linux kernel as of 2024/11
		cacheNodePrefetchMisses,   // not verified, not supported by linux kernel as of 2024/11
	},
	perDie: {
		cacheLLReadAccesses,     // not verified, not supported by linux kernel as of 2024/11
		cacheLLReadMisses,       // not verified, not supported by linux kernel as of 2024/11
		cacheLLWriteAccesses,    // not verified, not supported by linux kernel as of 2024/11
		cacheLLWriteMisses,      // not verified, not supported by linux kernel as of 2024/11
		cacheLLPrefetchAccesses, // not verified, not supported by linux kernel as of 2024/11
		cacheLLPrefetchMisses,   // not verified, not supported by linux kernel as of 2024/11
	},
}

// Helper maps defined below are holding strings instead of ints as event id needs to be queried from filename.
// See 'dynamic PMU' section for more info: man7.org/linux/man-pages/man2/perf_event_open.2.html.

const (
	typeFileName  string = "type"
	eventsDirName string = "events"

	powerPMUName      string = "power"
	powerPMUEnergyPkg string = "energy-pkg"
)

var dynamicPMUPath string = "/sys/bus/event_source/devices/"

// Helper map for iterations, new supported dynamic-power measurements must be added here.
var powerPerfEvents = map[int][]string{
	perPackage: {
		powerPMUEnergyPkg,
	},
}

// dynamicEvent holds data about dynamic PMU event discovered from sysfs.
type dynamicEvent struct {
	// Sourced from the /sys/bus/event_sources/devices/<pmu>/type file.
	kindID int
	// Sourced from the /sys/bus/event_sources/devices/<pmu>/events/<event> file.
	eventID int
	// Sourced from the /sys/bus/event_sources/devices/<pmu>/events/<event>.scale file.
	scale float64
}

// PerfEventClient is thread safe client to perf_event_open counters.
// Single instance should be created using constructor and passed as
// a pointer.
type PerfEventClient struct {
	readers       map[string]map[uint]perfEventReader
	dynamicEvents map[string]dynamicEvent
	host          power.Host
	log           logr.Logger
}

// By default, all supported perf measurements are enabled.
// host is instance of power optimization library Host that exposes system topology.
// Readers are automatically started on client creation, but need to be closed
// after no longer in use.
// Return pointer to PerfEventClient that should be the only instance created within binary.
func NewPerfEventClient(log logr.Logger, host power.Host) *PerfEventClient {
	pc := PerfEventClient{
		readers:       make(map[string]map[uint]perfEventReader),
		dynamicEvents: make(map[string]dynamicEvent),
		host:          host,
		log:           log,
	}

	// Static events
	pc.addEventGroup(hwPerfEvents, unix.PERF_TYPE_HARDWARE, pc.addDefaultEvent)
	pc.addEventGroup(swPerfEvents, unix.PERF_TYPE_SOFTWARE, pc.addDefaultEvent)
	pc.addEventGroup(cachePerfEvents, unix.PERF_TYPE_HW_CACHE, pc.addDefaultEvent)
	// Dynamic events
	pc.addDynamicEventGroup(powerPerfEvents, powerPMUName, pc.addDefaultEvent)

	pc.log.V(4).Info("New PerfEventClient created")

	return &pc
}

func (pc *PerfEventClient) Close() {
	pc.log.V(4).Info("Closing all registered readers")

	for eventID, eventReaders := range pc.readers {
		for scopeID, reader := range eventReaders {
			if err := reader.close(); err != nil {
				pc.log.V(5).Info(fmt.Sprintf("error while closing reader, err: %v", err),
					"event ID", eventID, "scope ID", scopeID)
			}
		}
	}
}

func (pc *PerfEventClient) GetCycles(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES),
		cpuLogKey, "cpu cycles",
	)
}

func (pc *PerfEventClient) GetInstructions(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS),
		cpuLogKey, "retired instructions",
	)
}

func (pc *PerfEventClient) GetCacheAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_REFERENCES),
		cpuLogKey, "cache accesses",
	)
}

func (pc *PerfEventClient) GetCacheMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES),
		cpuLogKey, "cpu misses",
	)
}

func (pc *PerfEventClient) GetBranchInstructions(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_BRANCH_INSTRUCTIONS),
		cpuLogKey, "branch instructions",
	)
}

func (pc *PerfEventClient) GetBranchMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_BRANCH_MISSES),
		cpuLogKey, "mispredicted branch instructions",
	)
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetBusCycles(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_BUS_CYCLES),
		cpuLogKey, "bus cycles",
	)
}

func (pc *PerfEventClient) GetStalledCyclesFrontend(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_STALLED_CYCLES_FRONTEND),
		cpuLogKey, "stalled cycles frontend",
	)
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetStalledCyclesBackend(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_STALLED_CYCLES_BACKEND),
		cpuLogKey, "stalled cycles backend",
	)
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetRefCycles(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES),
		cpuLogKey, "ref cycles",
	)
}

func (pc *PerfEventClient) GetBPFOutput(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_BPF_OUTPUT),
		cpuLogKey, "BPF output",
	)
}

func (pc *PerfEventClient) GetL1DCacheReadAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1DReadAccesses),
		cpuLogKey, "l1 data cache read accesses")
}

func (pc *PerfEventClient) GetL1DCacheReadMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1DReadMisses),
		cpuLogKey, "l1 data cache read misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1DCacheWriteAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1DWriteAccesses),
		cpuLogKey, "l1 data cache write accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1DCacheWriteMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1DWriteMisses),
		cpuLogKey, "l1 data cache write misses")
}

func (pc *PerfEventClient) GetL1DCachePrefetchAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1DPrefetchAccesses),
		cpuLogKey, "l1 data cache prefetch accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1DCachePrefetchMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1DPrefetchMisses),
		cpuLogKey, "l1 data cache prefetch misses")
}

func (pc *PerfEventClient) GetL1ICacheReadAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1IReadAccesses),
		cpuLogKey, "l1 instruction cache read accesses")
}

func (pc *PerfEventClient) GetL1ICacheReadMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1IReadMisses),
		cpuLogKey, "l1 instruction cache read misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1ICacheWriteAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1IWriteAccesses),
		cpuLogKey, "l1 instruction cache write accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1ICacheWriteMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1IWriteMisses),
		cpuLogKey, "l1 instruction cache write misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1ICachePrefetchAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1IPrefetchAccesses),
		cpuLogKey, "l1 instruction cache prefetch accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetL1ICachePrefetchMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheL1IPrefetchMisses),
		cpuLogKey, "l1 instruction cache prefetch misses")
}

func (pc *PerfEventClient) GetBPUCacheReadAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheBPUReadAccesses),
		cpuLogKey, "branch prediction unit cache read accesses")
}

func (pc *PerfEventClient) GetBPUCacheReadMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheBPUReadMisses),
		cpuLogKey, "branch prediction unit cache read misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetBPUCacheWriteAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheBPUWriteAccesses),
		cpuLogKey, "branch prediction unit cache write accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetBPUCacheWriteMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheBPUWriteMisses),
		cpuLogKey, "branch prediction unit cache write misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetBPUCachePrefetchAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheBPUPrefetchAccesses),
		cpuLogKey, "branch prediction unit cache prefetch accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetBPUCachePrefetchMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheBPUPrefetchMisses),
		cpuLogKey, "branch prediction unit cache prefetch misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetNodeCacheReadAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheNodeReadAccesses),
		cpuLogKey, "node cache read accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetNodeCacheReadMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheNodeReadMisses),
		cpuLogKey, "node cache read misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetNodeCacheWriteAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheNodeWriteAccesses),
		cpuLogKey, "node cache write accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetNodeCacheWriteMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheNodeWriteMisses),
		cpuLogKey, "node cache write misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetNodeCachePrefetchAccesses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheNodePrefetchAccesses),
		cpuLogKey, "node cache prefetch accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetNodeCachePrefetchMisses(cpu power.Cpu) (uint64, error) {
	return pc.readEvent(
		cpu.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheNodePrefetchMisses),
		cpuLogKey, "node cache prefetch misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetLLCacheReadAccesses(die power.Die) (uint64, error) {
	return pc.readEvent(
		die.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheLLReadAccesses),
		dieLogKey, "last level cache read accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetLLCacheReadMisses(die power.Die) (uint64, error) {
	return pc.readEvent(
		die.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheLLReadMisses),
		dieLogKey, "last level cache read misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetLLCacheWriteAccesses(die power.Die) (uint64, error) {
	return pc.readEvent(
		die.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheLLWriteAccesses),
		dieLogKey, "last level cache write accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetLLCacheWriteMisses(die power.Die) (uint64, error) {
	return pc.readEvent(
		die.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheLLWriteMisses),
		dieLogKey, "last level cache write misses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetLLCachePrefetchAccesses(die power.Die) (uint64, error) {
	return pc.readEvent(
		die.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheLLPrefetchAccesses),
		dieLogKey, "last level cache prefetch accesses")
}

// not verified, not supported by linux kernel as of 2024/11
func (pc *PerfEventClient) GetLLCachePrefetchMisses(die power.Die) (uint64, error) {
	return pc.readEvent(
		die.GetID(), pc.getKey(unix.PERF_TYPE_HW_CACHE, cacheLLPrefetchMisses),
		dieLogKey, "last level cache prefetch misses")
}

func (pc *PerfEventClient) GetPackageEnergyConsumption(pkg power.Package) (float64, error) {
	return pc.readDynamicEvent(
		pkg.GetID(),
		pc.getDynamicKey(powerPMUName, powerPMUEnergyPkg),
		packageLogKey, "package energy consumption")
}

// getDynamicKey creates string identifier for dynamic perf event.
// Key is created by combining pmuName with eventName seperated by delimeter.
func (pc *PerfEventClient) getDynamicKey(pmuName, eventName string) string {
	return fmt.Sprintf("%s:%s", pmuName, eventName)
}

// getKey creates string identifier for specific perf event reader.
// Key is created by combining kindID with eventID seperated by delimeter.
func (pc *PerfEventClient) getKey(kindID, eventID int) string {
	return fmt.Sprintf("%d:%d", kindID, eventID)
}

// scopeName and eventName are passed for user friendly logs
func (pc *PerfEventClient) readDynamicEvent(scopeID uint, eventKey string, scopeName, eventName string,
) (float64, error) {
	logger := pc.log.WithValues("event key", eventKey, "event name", eventName, "scope", scopeName, "scope ID", scopeID)

	event, ok := pc.dynamicEvents[eventKey]
	if !ok {
		logger.V(5).Info(fmt.Sprintf("err: %v", ErrMetricMissing))
		return 0, ErrMetricMissing
	}

	counterVal, err := pc.readEvent(scopeID, pc.getKey(event.kindID, event.eventID), scopeName, eventName)
	if err != nil {
		return 0, err
	}

	return float64(counterVal) * event.scale, nil
}

// scopeName and eventName are passed for user friendly logs
func (pc *PerfEventClient) readEvent(scopeID uint, eventKey string, scopeName, eventName string) (uint64, error) {
	logger := pc.log.WithValues("event key", eventKey, "event name", eventName, "scope", scopeName, "scope ID", scopeID)

	reader, ok := pc.readers[eventKey][scopeID]
	if !ok {
		logger.V(5).Info(fmt.Sprintf("err: %v", ErrMetricMissing))
		return 0, ErrMetricMissing
	}
	val, err := reader.read()
	if err != nil {
		return 0, err
	}

	return val, nil
}

func (pc *PerfEventClient) addDynamicEventGroup(kindMap map[int][]string, pmuName string,
	addEvent func(scopeID uint, cpuID uint, eventID, kind int, scopeName string),
) {
	logger := pc.log.WithValues("dynamic PMU name", pmuName)

	kindID, err := pc.getDynamicPMUTypeID(pmuName)
	if err != nil {
		logger.Error(err, "failed to use dynamic PMU")
		return
	}

	if pkgScoped, ok := kindMap[perPackage]; ok {
		for _, eventName := range pkgScoped {
			util.IterateOverPackages(pc.host, func(pkg power.Package) {
				logger = logger.WithValues("event name", eventName)

				eventID, err := pc.getDynamicPMUEventConfig(pmuName, eventName)
				if err != nil {
					logger.Error(err, "error while adding event")
					return
				}
				scale, err := pc.getDynamicEventScale(pmuName, eventName)
				if err != nil {
					logger.Error(err, "error while adding event")
					return
				}

				pc.dynamicEvents[pc.getDynamicKey(pmuName, eventName)] = dynamicEvent{kindID, eventID, scale}
				addEvent(pkg.GetID(), pkg.CPUs().IDs()[0], eventID, kindID, packageLogKey)
			})
		}
	}
}

func (pc *PerfEventClient) addEventGroup(kindMap map[int][]int, kind int,
	addEvent func(scopeID uint, cpuID uint, eventID, kind int, scopeName string),
) {
	if dieScoped, ok := kindMap[perDie]; ok {
		for _, eventID := range dieScoped {
			util.IterateOverDies(pc.host, func(die power.Die, _ power.Package) {
				addEvent(die.GetID(), die.CPUs().IDs()[0], eventID, kind, dieLogKey)
			})
		}
	}
	if cpuScoped, ok := kindMap[perCPU]; ok {
		for _, eventID := range cpuScoped {
			util.IterateOverCPUs(pc.host, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
				addEvent(cpu.GetID(), cpu.GetID(), eventID, kind, cpuLogKey)
			})
		}
	}
}

// scopeName is passed for user friendly logs
func (pc *PerfEventClient) addDefaultEvent(scopeID uint, cpuID uint, eventID, kind int, scopeName string) {
	logger := pc.log.WithValues("event ID", eventID, "scope", scopeName, "scope ID", scopeID, "type", kind)

	reader, err := newDefaultPerfEventReaderFunc(int(cpuID), kind, eventID)
	if err != nil {
		logger.Error(err, "error while creating event reader")
		return
	}

	key := pc.getKey(kind, eventID)
	if _, ok := pc.readers[key]; !ok {
		pc.readers[key] = make(map[uint]perfEventReader)
	}
	pc.readers[key][scopeID] = reader
	logger.V(5).Info("Initialized reader")

	if err := reader.start(); err != nil {
		logger.Error(err, "error while starting reader, closing it immediately")
		if err := reader.close(); err != nil {
			logger.Error(err, "error while closing reader, nothing else to do")
		}
		delete(pc.readers[key], scopeID)
	}
}

func (pc *PerfEventClient) getDynamicPMUTypeID(pmuName string) (int, error) {
	typeFilepath := filepath.Join(dynamicPMUPath, pmuName, typeFileName)
	typeBytes, err := os.ReadFile(typeFilepath)
	if err != nil {
		err = fmt.Errorf("failed to read content of dynamic PMU file %s: %w", typeFilepath, err)
		return 0, err
	}
	typeContent := strings.Trim(string(typeBytes), "\n ")
	kindID, err := strconv.Atoi(typeContent)
	if err != nil {
		err = fmt.Errorf("failed to convert content '%s' of dynamic PMU file %s to integer: %w",
			typeContent, typeFilepath, err)
		return 0, err
	}

	return kindID, nil
}

func (pc *PerfEventClient) getDynamicEventScale(pmuName string, eventName string) (float64, error) {
	scaleFilepath := filepath.Join(dynamicPMUPath, pmuName, eventsDirName, eventName+".scale")
	scaleBytes, err := os.ReadFile(scaleFilepath)
	if err != nil {
		err = fmt.Errorf("failed to read content of dynamic PMU file %s: %w", scaleFilepath, err)
		return 0.0, err
	}
	scaleContent := strings.Trim(string(scaleBytes), "\n ")
	scale, err := strconv.ParseFloat(scaleContent, 64)
	if err != nil || math.IsNaN(scale) || math.IsInf(scale, 0) {
		err = fmt.Errorf("failed to convert content '%s' of dynamic PMU file %s to float64: %w",
			scaleContent, scaleFilepath, err,
		)
		return 0.0, err
	}

	return scale, nil
}

func (pc *PerfEventClient) getDynamicPMUEventConfig(pmuName, eventName string) (int, error) {
	configFilepath := filepath.Join(dynamicPMUPath, pmuName, eventsDirName, eventName)
	configBytes, err := os.ReadFile(configFilepath)
	if err != nil {
		pc.log.Error(err, "failed to read content of dynamic PMU file", "file", configFilepath)
		return 0, err
	}
	configStr := strings.Trim(string(configBytes), "\n ")

	var eventID int
	for _, pair := range strings.Split(configStr, ",") {
		var val int
		if n, err := fmt.Sscanf(pair, "event=0x%x", &val); err == nil && n == 1 {
			eventID = val
			continue
		}
		return 0, fmt.Errorf("%s key-value pair found in %s not supported/recognized", pair, configFilepath)
	}

	return eventID, nil
}
