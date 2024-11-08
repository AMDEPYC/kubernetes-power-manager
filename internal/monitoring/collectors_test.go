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

const testSubsystem = "test"

type testMetricClient struct {
	mock.Mock
}

func (p *testMetricClient) GetPerCPUMetric(id power.Cpu) (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testMetricClient) GetNotStartedPerCPUMetric(power.Cpu) (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetPerCoreMetric(id power.Core) (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testMetricClient) GetNotStartedPerCoreMetric(power.Core) (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetPerPackageMetric(id power.Package) (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testMetricClient) GetNotStartedPerPackageMetric(power.Package) (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetPerSystemMetric() (uint64, error) {
	return p.Called().Get(0).(uint64), nil
}

func (p *testMetricClient) GetNotStartedPerSystemMetric() (uint64, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetPackageDIMMMetric(power.Package, uint8) (float32, error) {
	return p.Called().Get(0).(float32), nil
}

func (p *testMetricClient) GetNotStartedPackageDIMMMetric(power.Package, uint8) (float32, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetPackageNBIOMetric(power.Package, uint8) (uint8, error) {
	return p.Called().Get(0).(uint8), nil
}

func (p *testMetricClient) GetNotStartedPackageNBIOMetric(power.Package, uint8) (uint8, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetPackageIOLinkMetric(power.Package, string) (uint32, error) {
	return p.Called().Get(0).(uint32), nil
}

func (p *testMetricClient) GetNotStartedPackageIOLinkMetric(power.Package, string) (uint32, error) {
	p.Called()
	return 0, metrics.ErrMetricMissing
}

func (p *testMetricClient) GetIOLinkMetric(string) (uint32, error) {
	return p.Called().Get(0).(uint32), nil
}

func (p *testMetricClient) GetNotStartedIOLinkMetric(string) (uint32, error) {
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

func initializeMetricClientMock() *testMetricClient {
	client := &testMetricClient{}
	client.On("GetPerCPUMetric").Return(uint64(1))
	client.On("GetNotStartedPerCPUMetric").Return()
	client.On("GetPerCoreMetric").Return(uint64(2))
	client.On("GetNotStartedPerCoreMetric").Return()
	client.On("GetPerPackageMetric").Return(uint64(3))
	client.On("GetNotStartedPerPackageMetric").Return()
	client.On("GetPerSystemMetric").Return(uint64(5))
	client.On("GetNotStartedPerSystemMetric").Return()
	client.On("GetPackageDIMMMetric").Return(float32(6))
	client.On("GetNotStartedPackageDIMMMetric").Return()
	client.On("GetPackageNBIOMetric").Return(uint8(7))
	client.On("GetNotStartedPackageNBIOMetric").Return()
	client.On("GetPackageIOLinkMetric").Return(uint32(8))
	client.On("GetNotStartedPackageIOLinkMetric").Return()
	client.On("GetIOLinkMetric").Return(uint32(9))
	client.On("GetNotStartedIOLinkMetric").Return()

	return client
}

func TestNewPerCPUCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_cpu_metric1_total")
	workingPerCPUCollector := newPerCPUCollector(
		metricName,
		"test cpu metric1",
		prom.CounterValue,
		powerLibMock,
		client.GetPerCPUMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_cpu_metric1_total test cpu metric1
		# TYPE power_test_test_cpu_metric1_total counter
		power_test_test_cpu_metric1_total{core="0",cpu="0",die="0",package="0"} 1
		power_test_test_cpu_metric1_total{core="0",cpu="1",die="0",package="0"} 1
	`
	err := promtestutil.CollectAndCompare(workingPerCPUCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_cpu_metric2_total")
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
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_core_metric1_total")
	workingPerCoreCollector := newPerCoreCollector(
		metricName,
		"test core metric1",
		prom.CounterValue,
		powerLibMock,
		client.GetPerCoreMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_core_metric1_total test core metric1
		# TYPE power_test_test_core_metric1_total counter
		power_test_test_core_metric1_total{core="0",die="0",package="0"} 2
	`
	err := promtestutil.CollectAndCompare(workingPerCoreCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_core_metric2_total")
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
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_package_metric1_total")
	workingPerPackageCollector := newPerPackageCollector(
		metricName,
		"test package metric1",
		prom.CounterValue,
		powerLibMock,
		client.GetPerPackageMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_package_metric1_total test package metric1
		# TYPE power_test_test_package_metric1_total counter
		power_test_test_package_metric1_total{package="0"} 3
	`
	err := promtestutil.CollectAndCompare(workingPerPackageCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_package_metric2_total")
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

func TestNewPerSystemCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_system_metric1_total")
	workingPerSystemCollector := newPerSystemCollector(
		metricName,
		"test system metric1",
		prom.CounterValue,
		client.GetPerSystemMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_system_metric1_total test system metric1
		# TYPE power_test_test_system_metric1_total counter
		power_test_test_system_metric1_total{} 5
	`
	err := promtestutil.CollectAndCompare(workingPerSystemCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_system_metric2_total")
	notWorkingPerSystemCollector := newPerSystemCollector(
		metricName,
		"test system metric2",
		prom.CounterValue,
		client.GetNotStartedPerSystemMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPerSystemCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}

func TestNewPackageDimmCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializeMetricClientMock()
	origSupportedUMCNum := supportedUMCNum
	defer func() {
		supportedUMCNum = origSupportedUMCNum
	}()
	// Lower supported UMCs number to not have wall of expected metrics
	supportedUMCNum = 5

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_package_dimm_metric1")
	workingPackageDIMMCollector := newPackageDimmCollector(
		metricName,
		"test package & dimm metric1",
		prom.GaugeValue,
		powerLibMock,
		client.GetPackageDIMMMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_package_dimm_metric1 test package & dimm metric1
		# TYPE power_test_test_package_dimm_metric1 gauge
		power_test_test_package_dimm_metric1{dimm="0",package="0",umc="0"} 6
		power_test_test_package_dimm_metric1{dimm="1",package="0",umc="0"} 6
		power_test_test_package_dimm_metric1{dimm="0",package="0",umc="1"} 6
		power_test_test_package_dimm_metric1{dimm="1",package="0",umc="1"} 6
		power_test_test_package_dimm_metric1{dimm="0",package="0",umc="2"} 6
		power_test_test_package_dimm_metric1{dimm="1",package="0",umc="2"} 6
		power_test_test_package_dimm_metric1{dimm="0",package="0",umc="3"} 6
		power_test_test_package_dimm_metric1{dimm="1",package="0",umc="3"} 6
		power_test_test_package_dimm_metric1{dimm="0",package="0",umc="4"} 6
		power_test_test_package_dimm_metric1{dimm="1",package="0",umc="4"} 6
	`
	err := promtestutil.CollectAndCompare(workingPackageDIMMCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_package_dimm_metric2")
	notWorkingPackageDIMMCollector := newPackageDimmCollector(
		metricName,
		"test package & dimm metric2",
		prom.GaugeValue,
		powerLibMock,
		client.GetNotStartedPackageDIMMMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPackageDIMMCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}

func TestNewPackageNBIOCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_package_nbio_metric1")
	workingPackageNBIOCollector := newPackageNBIOCollector(
		metricName,
		"test package & nbio metric1",
		prom.GaugeValue,
		powerLibMock,
		client.GetPackageNBIOMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_package_nbio_metric1 test package & nbio metric1
		# TYPE power_test_test_package_nbio_metric1 gauge
		power_test_test_package_nbio_metric1{nbio="0",package="0"} 7
		power_test_test_package_nbio_metric1{nbio="1",package="0"} 7
		power_test_test_package_nbio_metric1{nbio="2",package="0"} 7
		power_test_test_package_nbio_metric1{nbio="3",package="0"} 7
	`
	err := promtestutil.CollectAndCompare(workingPackageNBIOCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_package_nbio_metric2")
	notWorkingPackageNBIOCollector := newPackageNBIOCollector(
		metricName,
		"test package & nbio metric2",
		prom.GaugeValue,
		powerLibMock,
		client.GetNotStartedPackageNBIOMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPackageNBIOCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}

func TestNewPackageIOLinkCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	powerLibMock := initializeHostMock()
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_package_io_link_metric1")
	workingPackageIOLinkCollector := newPackageIOLinkCollector(
		metricName,
		"test package & io_link metric1",
		prom.GaugeValue,
		powerLibMock,
		client.GetPackageIOLinkMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_package_io_link_metric1 test package & io_link metric1
		# TYPE power_test_test_package_io_link_metric1 gauge
		power_test_test_package_io_link_metric1{io_link="P0",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="P1",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="P2",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="P3",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="P4",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G0",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G1",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G2",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G3",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G4",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G5",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G6",package="0"} 8
		power_test_test_package_io_link_metric1{io_link="G7",package="0"} 8
	`
	err := promtestutil.CollectAndCompare(workingPackageIOLinkCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_package_io_link_metric2")
	notWorkingPackageIOLinkCollector := newPackageIOLinkCollector(
		metricName,
		"test package & io_link metric2",
		prom.GaugeValue,
		powerLibMock,
		client.GetNotStartedPackageIOLinkMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPackageIOLinkCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}

func TestNewIOLinkCollector(t *testing.T) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))
	client := initializeMetricClientMock()

	metricName := prom.BuildFQName(promNamespace, testSubsystem, "test_io_link_metric1")
	workingPerIOLinkCollector := newPerIOLinkCollector(
		metricName,
		"test io_link metric1",
		prom.GaugeValue,
		client.GetIOLinkMetric,
		ctrl.Log.WithName("testing"),
	)
	expected := `
		# HELP power_test_test_io_link_metric1 test io_link metric1
		# TYPE power_test_test_io_link_metric1 gauge
		power_test_test_io_link_metric1{io_link="P0"} 9
		power_test_test_io_link_metric1{io_link="P1"} 9
		power_test_test_io_link_metric1{io_link="P2"} 9
		power_test_test_io_link_metric1{io_link="P3"} 9
		power_test_test_io_link_metric1{io_link="P4"} 9
		power_test_test_io_link_metric1{io_link="G0"} 9
		power_test_test_io_link_metric1{io_link="G1"} 9
		power_test_test_io_link_metric1{io_link="G2"} 9
		power_test_test_io_link_metric1{io_link="G3"} 9
		power_test_test_io_link_metric1{io_link="G4"} 9
		power_test_test_io_link_metric1{io_link="G5"} 9
		power_test_test_io_link_metric1{io_link="G6"} 9
		power_test_test_io_link_metric1{io_link="G7"} 9
	`
	err := promtestutil.CollectAndCompare(workingPerIOLinkCollector, strings.NewReader(expected), metricName)
	assert.Nil(t, err)

	metricName = prom.BuildFQName(promNamespace, testSubsystem, "test_io_link_metric2")
	notWorkingPerIOLinkCollector := newPerIOLinkCollector(
		metricName,
		"test io_link metric2",
		prom.GaugeValue,
		client.GetNotStartedIOLinkMetric,
		ctrl.Log.WithName("testing"),
	)
	// We expect the Collector to return nothing - that also means no errors
	err = promtestutil.CollectAndCompare(notWorkingPerIOLinkCollector, strings.NewReader(``), metricName)
	assert.Nil(t, err)
}
