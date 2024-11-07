package metrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"
)

// Func definitions for unit testing
var (
	newMSRReaderFunc func(int) (msrReader, error) = newMSRReader
)

// MSR offsets definitions
const (
	tscOffset   uint64 = 0x00000010
	mperfOffset uint64 = 0x000000E7
	aperfOffset uint64 = 0x000000E8

	raplOffset                uint64 = 0xC0010299
	totalPkgEnergyTicksOfsset uint64 = 0xC001029B
	totalCoreEnergyOffset     uint64 = 0xC001029A
)

const (
	workersInterval time.Duration = 1 * time.Second
)

var (
	// ErrCStateResidencyInsufficientData is returned when msrReader.read() method returned error
	// during runtime. We assume that the issue is temporary since MSR file handle for that CPU
	// was created properly and was readable.
	ErrCStateResidencyInsufficientData error = errors.New("insufficient data to calculate C-state residency")

	// ErrCStateResidencyNotYetCalculated is returned when trying to read C0 and Cx residency values when they
	// were not yet calculated (meaning time period of 2 intervals has not yet passed since client creation).
	ErrCStateResidencyNotYetCalculated error = errors.New("not yet calculated C-state residency")
)

// MSRClient is thread safe client to MSR pseudofiles located by default in
// /dev/cpuX/ directories. Single instance should be created using constructor
// and passed as a pointer.
type MSRClient struct {
	readers map[uint]msrReader
	host    power.Host
	log     logr.Logger

	workersCancel       context.CancelFunc
	workersWaitGroup    sync.WaitGroup
	c0ResPercentResults sync.Map
}

// MSRClient opens handles to all CPUs found on the system during power-library
// initialization. Handles need to be closed after client is no longer in use.
// Client also starts worker goroutine per CPU to constantly calculate C-state
// residency of that core in specified interval (set as const in in metrics package).
// host is instance of power-library Host that exposes system topology.
// Returns pointer to MSRClient that should be the only instance created within binary.
func NewMSRClient(log logr.Logger, host power.Host) *MSRClient {
	ctx, cancel := context.WithCancel(context.Background())
	msr := MSRClient{
		readers:       make(map[uint]msrReader),
		host:          host,
		log:           log,
		workersCancel: cancel,
	}

	msr.addReaders()
	msr.startC0ResidencyWorkers(ctx)
	msr.log.V(4).Info("New MSRClient created")

	return &msr
}

type c0ResidencyResult struct {
	value uint8
	err   error
}

func (msr *MSRClient) Close() {
	msr.log.V(4).Info("Closing all registered readers")
	for cpuId, reader := range msr.readers {
		if err := reader.close(); err != nil {
			msr.log.V(5).Info(fmt.Sprintf("error while closing reader, err: %v", err), "cpu id", cpuId)
		}
	}

	msr.log.V(4).Info("Closing all C-state residency worker goroutines")
	msr.workersCancel()
	msr.workersWaitGroup.Wait()
}

func (msr *MSRClient) GetC0ResidencyPercent(cpu power.Cpu) (uint8, error) {
	logger := msr.log.WithValues("cpu", cpu.GetID())

	result, ok := msr.c0ResPercentResults.Load(cpu.GetID())
	if !ok {
		// Map keys are added on client creation so this branch should not happen at all.
		logger.V(5).Info(
			fmt.Sprintf("unexpected behavior, ensure MSRClient was created using constructor, err: %v",
				ErrMetricMissing),
		)
		return 0, ErrMetricMissing
	}
	r := result.(c0ResidencyResult)

	return r.value, r.err
}

func (msr *MSRClient) GetCxResidencyPercent(cpu power.Cpu) (uint8, error) {
	logger := msr.log.WithValues("cpu", cpu.GetID())

	result, ok := msr.c0ResPercentResults.Load(cpu.GetID())
	if !ok {
		// Map keys are added on client creation so this branch should not happen at all.
		logger.V(5).Info(
			fmt.Sprintf("unexpected behavior, ensure MSRClient was created using constructor, err: %v",
				ErrMetricMissing),
		)
		return 0, ErrMetricMissing
	}
	r := result.(c0ResidencyResult)

	return 100 - r.value, r.err
}

func (msr *MSRClient) GetPackageEnergyConsumption(pkg power.Package) (float64, error) {
	cpu := (*pkg.CPUs())[0]

	energyTicks, err := msr.readMetric(cpu.GetID(), totalPkgEnergyTicksOfsset, pkg.GetID(), packageLogKey)
	if err != nil {
		return 0.0, err
	}
	energyUnits, err := msr.getRAPLEnergyUnit(cpu)
	if err != nil {
		return 0.0, err
	}
	joulesConsumed := float64(energyTicks) * energyUnits

	return joulesConsumed, nil
}

func (msr *MSRClient) GetCoreEnergyConsumption(core power.Core) (float64, error) {
	cpu := (*core.CPUs())[0]

	energyTicks, err := msr.readMetric(cpu.GetID(), totalCoreEnergyOffset, core.GetID(), packageLogKey)
	if err != nil {
		return 0.0, err
	}
	energyUnits, err := msr.getRAPLEnergyUnit(cpu)
	if err != nil {
		return 0.0, err
	}
	joulesConsumed := float64(energyTicks) * energyUnits

	return joulesConsumed, nil
}

func (msr *MSRClient) getRAPLEnergyUnit(cpu power.Cpu) (float64, error) {
	raplMeta, err := msr.readMetric(cpu.GetID(), raplOffset, cpu.GetID(), cpuLogKey)
	if err != nil {
		return 0, err
	}
	const energyUnitBits uint64 = 0b0001111100000000
	exponent := (raplMeta & energyUnitBits) >> 8

	return 1 / math.Pow(2, float64(exponent)), nil
}

// scopeid and scopeName are passed for user friendly logs
func (msr *MSRClient) readMetric(cpu uint, offset uint64, scopeId uint, scopeName string) (uint64, error) {
	logger := msr.log.WithValues("cpu", cpu, "scope id", scopeId, "scope name", scopeName)

	reader, ok := msr.readers[cpu]
	if !ok {
		logger.V(5).Info(fmt.Sprintf("err: %v", ErrMetricMissing))
		return 0, ErrMetricMissing
	}
	val, err := reader.read(offset)
	if err != nil {
		return 0, err
	}

	return val, nil
}

func (msr *MSRClient) addReaders() {
	util.IterateOverCPUs(msr.host, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		logger := msr.log.WithValues("cpu", cpu.GetID())

		reader, err := newMSRReaderFunc(int(cpu.GetID()))
		if err != nil {
			logger.Error(err, "error while creating event reader")
			return
		}
		msr.readers[cpu.GetID()] = reader

		logger.V(5).Info("Initialized reader")
	})
}

// startC0ResidencyWorkers first checks if MSR readers needed for workers calculations are readable.
// If yes, it starts goroutine for each CPU, if not, it logs an error.
func (msr *MSRClient) startC0ResidencyWorkers(ctx context.Context) {
	util.IterateOverCPUs(msr.host, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		logger := msr.log.WithValues("cpu", cpu.GetID())

		// Test if MSR readers are active before starting worker goroutine
		_, errTsc := msr.readMetric(cpu.GetID(), tscOffset, cpu.GetID(), cpuLogKey)
		_, errMperf := msr.readMetric(cpu.GetID(), mperfOffset, cpu.GetID(), cpuLogKey)
		if errMperf == ErrMetricMissing || errTsc == ErrMetricMissing {
			logger.Error(ErrMetricMissing, "not starting C-state residency worker goroutine")
			msr.c0ResPercentResults.Store(cpu.GetID(), c0ResidencyResult{err: ErrMetricMissing})
			return
		}

		msr.workersWaitGroup.Add(1)
		msr.c0ResPercentResults.Store(cpu.GetID(), c0ResidencyResult{err: ErrCStateResidencyNotYetCalculated})
		go msr.c0ResidencyWorker(cpu, ctx)
		logger.V(5).Info("Started C-state residency worker goroutine")
	})
}

// c0ResidencyWorker calculates percent of CPU C0-state residency in last time interval (specified by
// const variable). It takes as arguments CPU that will be monitored and context - used only for cancellation.
// At the end of each calculation loop it updates the value stored in sync.Map (client field) for that CPU.
// It pushes first value after 2 intervals, and then updates it every interval. Worker supports error handling.
func (msr *MSRClient) c0ResidencyWorker(cpu power.Cpu, ctx context.Context) {
	defer msr.workersWaitGroup.Done()
	logger := msr.log.WithValues("cpu", cpu.GetID(), "worker", "C-state residency")

	var (
		tsc      uint64
		tscErr   error = ErrCStateResidencyNotYetCalculated
		mperf    uint64
		mperfErr = ErrCStateResidencyNotYetCalculated

		prevTSC      uint64
		prevTSCErr   error
		prevMPERF    uint64
		prevMPERFErr error
	)

	for {
		select {
		case <-ctx.Done():
			logger.V(5).Info("cancellation signal received, exiting work")
			return
		case <-time.After(workersInterval):
			prevTSC, prevTSCErr = tsc, tscErr
			if tsc, tscErr = msr.readMetric(cpu.GetID(), tscOffset, cpu.GetID(), cpuLogKey); tscErr != nil {
				msr.c0ResPercentResults.Store(cpu.GetID(), c0ResidencyResult{err: tscErr})
				logger.V(5).Info(
					fmt.Sprintf("error retrieving TSC value, continuing to next iteration, err: %v", tscErr),
				)
				continue
			}

			prevMPERF, prevMPERFErr = mperf, mperfErr
			if mperf, mperfErr = msr.readMetric(cpu.GetID(), mperfOffset, cpu.GetID(), cpuLogKey); mperfErr != nil {
				msr.c0ResPercentResults.Store(cpu.GetID(), c0ResidencyResult{err: mperfErr})
				logger.V(5).Info(
					fmt.Sprintf("error retrieving MPERF value, continuing to next iteration, err: %v", mperfErr),
				)
				continue
			}

			// tsc <= prevTSC check is there to prevent dividing by 0 that would happen
			// when 2 consecutive TSC reads will have the same value. We also handle
			// situation in which counters were reset and prev is bigger than current.
			// Both situation very unlikely to happen.
			if prevTSCErr != nil || prevMPERFErr != nil || tsc <= prevTSC || mperf < prevMPERF {
				continue
			}

			c0ResPercentFloat := (float64(mperf-prevMPERF) / float64(tsc-prevTSC)) * 100
			// Both TSC and MPERF counters are going up by a lot every millisecond. MPERF is retrieved after TSC so
			// in situation when C0-state residency is close to 100% those nanoseconds between retrieval of both values
			// might cause MPERF > TSC which will yield value >100%. The same could happen when C0-state residency
			// is close to 0%, then the calculated value will be a little larger than actual. Since we want to return %,
			// and the calcuations cannot be fully accurate, we round to integers.
			c0ResPercent := uint8(math.Round(c0ResPercentFloat))

			msr.c0ResPercentResults.Store(cpu.GetID(), c0ResidencyResult{value: c0ResPercent})
		}
	}
}
