package monitoring

import (
	"strings"
	"testing"

	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/kubernetes-power-manager/pkg/testutils"
	"github.com/intel/power-optimization-library/pkg/power"

	prom "github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type testPerfEventClient struct {
	mock.Mock
}

func (p *testPerfEventClient) GetPerCPUMetric(id power.Cpu) (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testPerfEventClient) GetNotStartedPerCPUMetric(power.Cpu) (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testPerfEventClient) GetPerCoreMetric(id power.Core) (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testPerfEventClient) GetNotStartedPerCoreMetric(power.Core) (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testPerfEventClient) GetPerPackageMetric(id power.Package) (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testPerfEventClient) GetNotStartedPerPackageMetric(power.Package) (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
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
	mockDie.On("GetID").Return(uint(0))
	mockCore.On("CPUs").Return(&cpuList)
	mockCore.On("GetID").Return(uint(0))
	for id, mockedCPU := range cpuList {
		mockedCPU.(*testutils.MockCPU).On("GetID").Return(uint(id))
	}

	return mockHost
}

func initializePerfEventClientMock() *testPerfEventClient {
	client := &testPerfEventClient{}
	client.On("GetPerCPUMetric").Return(uint64(10))
	client.On("GetNotStartedPerCPUMetric").Return(uint64(10))
	client.On("GetPerCoreMetric").Return(uint64(10))
	client.On("GetNotStartedPerCoreMetric").Return(uint64(10))
	client.On("GetPerPackageMetric").Return(uint64(10))
	client.On("GetNotStartedPerPackageMetric").Return(uint64(10))

	return client
}

func TestNewPerCPUCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializePerfEventClientMock()

	metricName := prom.BuildFQName(promNamespace, perfSubsystem, "test_cpu_metric1_total")
	workingPerCPUCollector := newPerCPUCollector(
		metricName,
		"test cpu metric1",
		prom.CounterValue,
		powerLibMock,
		client.GetPerCPUMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_perf_test_cpu_metric1_total test cpu metric1
		# TYPE power_perf_test_cpu_metric1_total counter
		power_perf_test_cpu_metric1_total{core="0",cpu="0",die="0",package="0"} 10
		power_perf_test_cpu_metric1_total{core="0",cpu="1",die="0",package="0"} 10
	`
	err := promtestutil.CollectAndCompare(workingPerCPUCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, perfSubsystem, "test_cpu_metric2_total")
	notWorkingPerCPUCollector := newPerCPUCollector(
		metricName,
		"test cpu metric2",
		prom.CounterValue,
		powerLibMock,
		client.GetNotStartedPerCPUMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPerCPUCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}

func TestNewPerCoreCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializePerfEventClientMock()

	metricName := prom.BuildFQName(promNamespace, perfSubsystem, "test_core_metric1_total")
	workingPerCoreCollector := newPerCoreCollector(
		metricName,
		"test core metric1",
		prom.CounterValue,
		powerLibMock,
		client.GetPerCoreMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_perf_test_core_metric1_total test core metric1
		# TYPE power_perf_test_core_metric1_total counter
		power_perf_test_core_metric1_total{core="0",die="0",package="0"} 10
	`
	err := promtestutil.CollectAndCompare(workingPerCoreCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, perfSubsystem, "test_core_metric2_total")
	notWorkingPerCoreCollector := newPerCoreCollector(
		metricName,
		"test core metric2",
		prom.CounterValue,
		powerLibMock,
		client.GetNotStartedPerCoreMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPerCoreCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}

func TestNewPerPackageCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializePerfEventClientMock()

	metricName := prom.BuildFQName(promNamespace, perfSubsystem, "test_package_metric1_total")
	workingPerPackageCollector := newPerPackageCollector(
		metricName,
		"test package metric1",
		prom.CounterValue,
		powerLibMock,
		client.GetPerPackageMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_perf_test_package_metric1_total test package metric1
		# TYPE power_perf_test_package_metric1_total counter
		power_perf_test_package_metric1_total{package="0"} 10
	`
	err := promtestutil.CollectAndCompare(workingPerPackageCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, perfSubsystem, "test_package_metric2_total")
	notWorkingPerPackageCollector := newPerPackageCollector(
		metricName,
		"test package metric2",
		prom.CounterValue,
		powerLibMock,
		client.GetNotStartedPerPackageMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPerPackageCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}
