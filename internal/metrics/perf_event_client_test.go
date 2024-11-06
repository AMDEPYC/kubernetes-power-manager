package metrics

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/intel/kubernetes-power-manager/pkg/testutils"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type testPerfEventReader struct {
	counter uint64
	mock.Mock
}

func newTestPerfEventReader(_, _, _ int) (perfEventReader, error) {
	p := &testPerfEventReader{}
	p.On("start").Return(nil)
	return p, nil
}

func (p *testPerfEventReader) start() error { return p.Called().Error(0) }
func (p *testPerfEventReader) close() error { return p.Called().Error(0) }
func (p *testPerfEventReader) read() (uint64, error) {
	p.counter += rand.Uint64N(10000)
	return p.counter, nil
}

func initializeHostMock() *testutils.MockHost {
	mockHost := new(testutils.MockHost)
	mockTop := new(testutils.MockTopology)
	mockPkg := new(testutils.MockPackage)
	mockDie := new(testutils.MockDie)
	mockCore := new(testutils.MockCore)

	pkgList := mockPkg.MakeList()
	dieList := mockDie.MakeList()
	coreList := mockCore.MakeList()
	cpuList := testutils.MakeCPUList(
		&testutils.MockCPU{},
		&testutils.MockCPU{},
	)

	mockHost.On("Topology").Return(mockTop)
	mockTop.On("Packages").Return(&pkgList)
	mockPkg.On("Dies").Return(&dieList)
	mockPkg.On("CPUs").Return(&cpuList)
	mockPkg.On("GetID").Return(uint(0))
	mockDie.On("Cores").Return(&coreList)
	mockDie.On("CPUs").Return(&cpuList)
	mockDie.On("GetID").Return(uint(0))
	mockCore.On("CPUs").Return(&cpuList)
	mockCore.On("GetID").Return(uint(0))
	for id, mockedCPU := range cpuList {
		mockedCPU.(*testutils.MockCPU).On("GetID").Return(uint(id))
	}

	return mockHost
}

func TestNewPerfEventClient(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newDefaultPerfEventReaderFunc
	defer func() {
		newDefaultPerfEventReaderFunc = origFunc
	}()

	newDefaultPerfEventReaderFunc = newTestPerfEventReader
	powerLibMock := initializeHostMock()

	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)

	// Assert all defined events were added properly
	for _, scope := range []int{perCPU, perCore, perPackage} {
		for _, event := range hwPerfEvents[scope] {
			key := client.getKey(unix.PERF_TYPE_HARDWARE, event)
			assert.Contains(t, client.readers, key)
		}
		for _, event := range swPerfEvents[scope] {
			key := client.getKey(unix.PERF_TYPE_SOFTWARE, event)
			assert.Contains(t, client.readers, key)
		}
		for _, event := range cachePerfEvents[scope] {
			key := client.getKey(unix.PERF_TYPE_HW_CACHE, event)
			assert.Contains(t, client.readers, key)
		}
	}

	// Assert all defined events were started
	for _, eventReaders := range client.readers {
		for _, reader := range eventReaders {
			testReader := reader.(*testPerfEventReader)
			testReader.AssertExpectations(t)
		}
	}
}

func TestNewPerfEventClientWithErrors(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newDefaultPerfEventReaderFunc
	defer func() {
		newDefaultPerfEventReaderFunc = origFunc
	}()

	newDefaultPerfEventReaderFunc = func(_, _, _ int) (perfEventReader, error) {
		return nil, fmt.Errorf("reader startup error")
	}
	powerLibMock := initializeHostMock()

	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)
	// Assert no events were added
	assert.Empty(t, client.readers)
}

func TestPerfEventClientGetMethods(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newDefaultPerfEventReaderFunc
	defer func() {
		newDefaultPerfEventReaderFunc = origFunc
	}()

	newDefaultPerfEventReaderFunc = newTestPerfEventReader
	powerLibMock := initializeHostMock()
	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)

	// Methods with repeated logic are not tested
	// CPU scoped
	for _, method := range []func(power.Cpu) (uint64, error){
		client.GetCycles, client.GetBPFOutput,
	} {
		util.IterateOverCPUs(powerLibMock, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
			res1, err := method(cpu)
			assert.Nil(t, err)
			assert.Greater(t, res1, uint64(0))

			res2, err := method(cpu)
			assert.Nil(t, err)
			assert.Greater(t, res2, res1)
		})
	}
}

func TestPerfEventClientClose(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	origFunc := newDefaultPerfEventReaderFunc
	defer func() {
		newDefaultPerfEventReaderFunc = origFunc
	}()

	newDefaultPerfEventReaderFunc = newTestPerfEventReader
	powerLibMock := initializeHostMock()
	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)

	for _, eventReaders := range client.readers {
		for _, reader := range eventReaders {
			testReader := reader.(*testPerfEventReader)
			testReader.On("close").Return(nil)
		}
	}

	client.Close()

	for _, eventReaders := range client.readers {
		for _, reader := range eventReaders {
			testReader := reader.(*testPerfEventReader)
			testReader.AssertExpectations(t)
		}
	}
}
