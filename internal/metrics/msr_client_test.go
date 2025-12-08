package metrics

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/AMDEPYC/kubernetes-power-manager/pkg/util"
	"github.com/AMDEPYC/power-optimization-library/pkg/power"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type testMSRReader struct {
	tscCounter uint64
	counter    uint64
	mock.Mock
}

func newTestMSRReader(_ int) (msrReader, error) {
	msr := &testMSRReader{}
	msr.On("close").Return(nil)
	return msr, nil
}

func (msr *testMSRReader) close() error { return msr.Called().Error(0) }

func (msr *testMSRReader) read(offset uint64) (uint64, error) {
	// There is counter specifically for tsc as it has to be greater than mperf
	if offset == tscOffset {
		msr.tscCounter += (rand.Uint64N(1000) + 1000)
		return msr.tscCounter, nil
	}
	if offset == raplOffset {
		// Default raw RAPL unit value for AMD processors
		return 659459, nil
	}
	// Regular testing counter for everything else
	msr.counter += rand.Uint64N(1000)
	return msr.counter, nil
}

func TestNewMSRClient(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newMSRReaderFunc
	defer func() {
		newMSRReaderFunc = origFunc
	}()

	newMSRReaderFunc = newTestMSRReader
	powerLibMock := initializeHostMock()

	client := NewMSRClient(ctrl.Log.WithName("testing"), powerLibMock)
	defer client.Close()
	// Assert all readers were added properly and are readable
	util.IterateOverCPUs(powerLibMock, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		reader, ok := client.readers[cpu.GetID()]
		assert.True(t, ok)
		val, err := reader.read(0x0)
		assert.Nil(t, err)
		assert.Greater(t, val, uint64(0))
	})

	// Assert all workers were started properly and are updating values
	wg := sync.WaitGroup{}
	util.IterateOverCores(powerLibMock, func(core power.Core, _ power.Die, _ power.Package) {
		wg.Add(1)
		cpu := (*core.CPUs())[0]
		go func() {
			defer wg.Done()
			// Wait for the completion of the first two iterations of the worker loop, add 50ms for calculations.
			time.Sleep(2*workersInterval + 50*time.Millisecond)

			result, ok := client.c0ResPercentResults.Load(cpu.GetID())
			assert.True(t, ok)
			r := result.(c0ResidencyResult)
			assert.Nil(t, r.err)
		}()
	})
	wg.Wait()
}

func TestNewMSRClientWithErrors(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newMSRReaderFunc
	defer func() {
		newMSRReaderFunc = origFunc
	}()

	newMSRReaderFunc = func(i int) (msrReader, error) { return nil, fmt.Errorf("reader startup error") }
	powerLibMock := initializeHostMock()

	client := NewMSRClient(ctrl.Log.WithName("testing"), powerLibMock)
	defer client.Close()
	// Assert no readers were started
	util.IterateOverCPUs(powerLibMock, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		_, ok := client.readers[cpu.GetID()]
		assert.False(t, ok)
	})
	// Assert no worker goroutines were started
	util.IterateOverCores(powerLibMock, func(core power.Core, _ power.Die, _ power.Package) {
		cpu := (*core.CPUs())[0]
		result, ok := client.c0ResPercentResults.Load(cpu.GetID())
		assert.True(t, ok)
		r := result.(c0ResidencyResult)
		assert.ErrorIs(t, r.err, ErrMetricMissing)
	})
}

func TestGetC0ResidencyPercent(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newMSRReaderFunc
	defer func() {
		newMSRReaderFunc = origFunc
	}()

	newMSRReaderFunc = newTestMSRReader
	powerLibMock := initializeHostMock()

	client := NewMSRClient(ctrl.Log.WithName("testing"), powerLibMock)
	defer client.Close()
	// Ensure that reads occurring before the first two iterations of the worker loop are properly handled
	util.IterateOverCPUs(powerLibMock, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		_, err := client.GetC0ResidencyPercent(cpu)
		assert.ErrorIs(t, err, ErrCStateResidencyNotYetCalculated)
	})

	// Wait for the completion of the first two iterations of the worker loop, add 50ms for calculations
	time.Sleep(2*workersInterval + 50*time.Millisecond)

	// Check if two reads within the same interval are the same
	// (there is a wait time of two intervals before first metrics are pushed from goroutine)
	util.IterateOverCPUs(powerLibMock, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		val1, err := client.GetC0ResidencyPercent(cpu)
		assert.Nil(t, err)
		assert.GreaterOrEqual(t, val1, uint8(0))
		assert.LessOrEqual(t, val1, uint8(100))

		val2, err := client.GetC0ResidencyPercent(cpu)
		assert.Nil(t, err)
		assert.GreaterOrEqual(t, val2, uint8(0))
		assert.LessOrEqual(t, val2, uint8(100))

		assert.Equal(t, val1, val2)
	})

	// Check if two reads with gap longer than interval are different
	util.IterateOverCPUs(powerLibMock, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
		val1, err := client.GetC0ResidencyPercent(cpu)
		assert.Nil(t, err)
		assert.GreaterOrEqual(t, val1, uint8(0))
		assert.LessOrEqual(t, val1, uint8(100))

		// 50ms for calculations
		time.Sleep(workersInterval + 50*time.Millisecond)

		val2, err := client.GetC0ResidencyPercent(cpu)
		assert.Nil(t, err)
		assert.GreaterOrEqual(t, val2, uint8(0))
		assert.LessOrEqual(t, val2, uint8(100))

		assert.NotEqual(t, val1, val2)
	})
}

func TestGetRAPLEnergyUnit(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newMSRReaderFunc
	defer func() {
		newMSRReaderFunc = origFunc
	}()

	newMSRReaderFunc = newTestMSRReader
	powerLibMock := initializeHostMock()
	client := NewMSRClient(ctrl.Log.WithName("testing"), powerLibMock)
	defer client.Close()

	// Correctness is checked with default AMD RAPL values
	energyUnit, err := client.getRAPLEnergyUnit((*(*client.host.Topology().Packages())[0].CPUs())[0])
	assert.Nil(t, err)

	// Truncated to 6 decimal places after dot to easily compare with turbostat value
	shift := math.Pow(10, 6)
	truncated := math.Trunc(energyUnit*shift) / shift
	assert.Equal(t, float64(0.000015), truncated)
}

func TestMSRClientClose(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newMSRReaderFunc
	defer func() {
		newMSRReaderFunc = origFunc
	}()

	newMSRReaderFunc = newTestMSRReader
	powerLibMock := initializeHostMock()
	client := NewMSRClient(ctrl.Log.WithName("testing"), powerLibMock)
	defer client.Close()

	for _, reader := range client.readers {
		reader.(*testMSRReader).On("close").Return(nil)
	}

	client.Close()

	for _, reader := range client.readers {
		reader.(*testMSRReader).AssertExpectations(t)
	}
}
