package metrics

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/AMDEPYC/kubernetes-power-manager/pkg/testutils"
	"github.com/AMDEPYC/kubernetes-power-manager/pkg/util"
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

func setupDummySysfs(t *testing.T) string {
	testPMUPath := t.TempDir()
	origDynamicPMUPath := dynamicPMUPath
	dynamicPMUPath = testPMUPath
	t.Cleanup(func() {
		dynamicPMUPath = origDynamicPMUPath
	})

	// Setup mocked energy-pkg in power PMU
	os.MkdirAll(filepath.Join(testPMUPath, powerPMUName, eventsDirName), 0777)
	os.WriteFile(filepath.Join(testPMUPath, powerPMUName, "type"), []byte("999"), 0777)
	os.WriteFile(filepath.Join(testPMUPath, powerPMUName, eventsDirName, powerPMUEnergyPkg),
		[]byte("event=0x999"), 0777,
	)
	os.WriteFile(filepath.Join(testPMUPath, powerPMUName, eventsDirName, powerPMUEnergyPkg+".scale"),
		[]byte("1.23456789e-10"), 0777,
	)

	return testPMUPath
}

func setupNewTestReaderFunc(t *testing.T, f func(int, int, int) (perfEventReader, error)) {
	origNewDefaultPerfEventReaderFunc := newDefaultPerfEventReaderFunc
	newDefaultPerfEventReaderFunc = f
	t.Cleanup(func() {
		newDefaultPerfEventReaderFunc = origNewDefaultPerfEventReaderFunc
	})
}

func TestNewPerfEventClient(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	setupNewTestReaderFunc(t, newTestPerfEventReader)
	setupDummySysfs(t)

	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)

	// Assert all defined static events were added properly
	for _, scope := range []int{perDie, perPackage} {
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

	// Assert dynamic events were discovered properly
	key := client.getDynamicKey(powerPMUName, powerPMUEnergyPkg)
	assert.Contains(t, client.dynamicEvents, key)
	assert.Equal(t, 999, client.dynamicEvents[key].kindID)
	assert.Equal(t, 0x999, client.dynamicEvents[key].eventID)
	assert.Equal(t, 1.23456789e-10, client.dynamicEvents[key].scale)

	// Assert all defined dynamic events were added properly
	for _, scope := range []int{perPackage} {
		for _, eventName := range powerPerfEvents[scope] {
			key := client.getDynamicKey(powerPMUName, eventName)
			readersKey := client.getKey(
				client.dynamicEvents[key].kindID,
				client.dynamicEvents[key].eventID,
			)
			assert.Contains(t, client.readers, readersKey)
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

func TestNewPerfEventClientWithReadersErrors(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	setupNewTestReaderFunc(t, func(_, _, _ int) (perfEventReader, error) {
		return nil, fmt.Errorf("reader startup error")
	})
	setupDummySysfs(t)

	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)
	// Assert no events were added
	assert.Empty(t, client.readers)
}

func TestNewPerfEventClientWithPowerDiscoveryErrors(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	setupNewTestReaderFunc(t, newTestPerfEventReader)

	// Damage 'type' dynamic power PMU file
	testPMUPath := setupDummySysfs(t)
	os.WriteFile(filepath.Join(testPMUPath, powerPMUName, "type"), []byte("nan"), 0777)
	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)
	dynamicKey := client.getDynamicKey(powerPMUName, powerPMUEnergyPkg)
	// Assert power PMU events were not added at all - when more events of the same PMU are added, check it here
	assert.NotContains(t, client.dynamicEvents, dynamicKey)

	// Damage 'energy-pkg' dynamic power PMU file
	testPMUPath = setupDummySysfs(t)
	os.WriteFile(filepath.Join(testPMUPath, powerPMUName, eventsDirName, powerPMUEnergyPkg),
		[]byte("event=0x99,somethingelse=0x21"), 0777,
	)
	client = NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)
	// Assert energy-pkg event was not added at all
	assert.NotContains(t, client.dynamicEvents, dynamicKey)

	// Damage 'energy-pkg.scale' dynamic power PMU file
	testPMUPath = setupDummySysfs(t)
	os.WriteFile(filepath.Join(testPMUPath, powerPMUName, eventsDirName, powerPMUEnergyPkg+".scale"),
		[]byte("nan"), 0777,
	)
	client = NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)
	// Assert energy-pkg event was not added at all
	assert.NotContains(t, client.dynamicEvents, dynamicKey)
}

func TestPerfEventClientGetMethods(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	setupNewTestReaderFunc(t, newTestPerfEventReader)
	setupDummySysfs(t)

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
	// Die scoped
	for _, method := range []func(power.Die) (uint64, error){
		client.GetLLCacheReadAccesses,
	} {
		util.IterateOverDies(powerLibMock, func(die power.Die, _ power.Package) {
			res1, err := method(die)
			assert.Nil(t, err)
			assert.Greater(t, res1, uint64(0))

			res2, err := method(die)
			assert.Nil(t, err)
			assert.Greater(t, res2, res1)
		})
	}
}

func TestGetPackageEnergyConsumption(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	setupNewTestReaderFunc(t, newTestPerfEventReader)
	setupDummySysfs(t)

	client := NewPerfEventClient(ctrl.Log.WithName("testing"), powerLibMock)

	util.IterateOverPackages(powerLibMock, func(pkg power.Package) {
		res, err := client.GetPackageEnergyConsumption(pkg)
		assert.Nil(t, err)
		assert.NotEqual(t, 0, res)
	})
}

func TestPerfEventClientClose(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	setupNewTestReaderFunc(t, newTestPerfEventReader)
	setupDummySysfs(t)

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
