package testutils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"context"

	"github.com/go-logr/logr"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type MockHost struct {
	mock.Mock
	power.Host
}

func (m *MockHost) Topology() power.Topology {
	return m.Called().Get(0).(power.Topology)
}

func (m *MockHost) ValidateCStates(states power.CStates) error {
	return m.Called(states).Error(0)
}

func (m *MockHost) AvailableCStates() []string {
	return m.Called().Get(0).([]string)
}

func (m *MockHost) GetAllExclusivePools() *power.PoolList {
	return m.Called().Get(0).(*power.PoolList)
}

func (m *MockHost) SetName(name string) {
	m.Called(name)
}

func (m *MockHost) GetName() string {
	return m.Called().String(0)
}

func (m *MockHost) GetFreqRanges() power.CoreTypeList {
	return m.Called().Get(0).(power.CoreTypeList)
}

func (m *MockHost) GetFeaturesInfo() power.FeatureSet {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.FeatureSet)
	}
}

func (m *MockHost) GetReservedPool() power.Pool {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.Pool)
	}
}

func (m *MockHost) GetSharedPool() power.Pool {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.Pool)
	}
}
func (m *MockHost) AddExclusivePool(poolName string) (power.Pool, error) {
	args := m.Called(poolName)
	retPool := args.Get(0)
	if retPool == nil {
		return nil, args.Error(1)
	} else {
		return retPool.(power.Pool), args.Error(1)
	}
}

func (m *MockHost) GetExclusivePool(poolName string) power.Pool {
	ret := m.Called(poolName).Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(power.Pool)
	}
}

func (m *MockHost) GetAllCpus() *power.CpuList {
	ret := m.Called().Get(0)
	if ret == nil {
		return nil
	} else {
		return ret.(*power.CpuList)
	}
}

type MockPool struct {
	mock.Mock
	power.Pool
}

func (m *MockPool) SetCStates(states power.CStates) error {
	return m.Called(states).Error(0)
}

func (m *MockPool) Clear() error {
	return m.Called().Error(0)
}

func (m *MockPool) Name() string {
	return m.Called().String(0)
}

func (m *MockPool) Cpus() *power.CpuList {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*power.CpuList)
}

func (m *MockPool) SetCpus(cores power.CpuList) error {
	return m.Called(cores).Error(0)
}

func (m *MockPool) SetCpuIDs(cpuIDs []uint) error {
	return m.Called(cpuIDs).Error(0)
}

func (m *MockPool) Remove() error {
	return m.Called().Error(0)
}

func (m *MockPool) MoveCpuIDs(coreIDs []uint) error {
	return m.Called(coreIDs).Error(0)
}

func (m *MockPool) MoveCpus(cores power.CpuList) error {
	return m.Called(cores).Error(0)
}

func (m *MockPool) SetPowerProfile(profile power.Profile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *MockPool) GetPowerProfile() power.Profile {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(power.Profile)
}

type MockProf struct {
	mock.Mock
	power.Profile
}

func (m *MockProf) Name() string {
	return m.Called().String(0)
}

func (m *MockProf) Epp() string {
	return m.Called().String(0)
}

func (m *MockProf) MaxFreq() uint {
	return uint(m.Called().Int(0))
}

func (m *MockProf) MinFreq() uint {
	return uint(m.Called().Int(0))
}

func (m *MockProf) Governor() string {
	return m.Called().String(0)
}

type MockCPU struct {
	mock.Mock
	power.Cpu
}

func (m *MockCPU) SetCStates(cStates power.CStates) error {
	return m.Called(cStates).Error(0)
}

func (m *MockCPU) GetID() uint {
	return m.Called().Get(0).(uint)
}

func (m *MockCPU) SetPool(pool power.Pool) error {
	return m.Called(pool).Error(0)
}

func MakeCPUList(mockedCPUs ...*MockCPU) power.CpuList {
	cpuList := power.CpuList{}
	for _, mockedCPU := range mockedCPUs {
		cpuList = append(cpuList, mockedCPU)
	}

	return cpuList
}

type MockCore struct {
	mock.Mock
	power.Core
}

func (m *MockCore) MakeList() []power.Core {
	return []power.Core{m}
}

func (m *MockCore) CPUs() *power.CpuList {
	args := m.Called().Get(0)
	if args == nil {
		return nil
	}
	return args.(*power.CpuList)
}

func (m *MockCore) GetID() uint {
	args := m.Called()
	return args.Get(0).(uint)
}

type MockTopology struct {
	mock.Mock
	power.Topology
}

func (m *MockTopology) GetID() uint {
	return m.Called().Get(0).(uint)
}

func (m *MockTopology) SetUncore(uncore power.Uncore) error {
	return m.Called(uncore).Error(0)
}

func (m *MockTopology) applyUncore() error {
	return m.Called().Error(0)
}

func (m *MockTopology) getEffectiveUncore() power.Uncore {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(power.Uncore)
	}
	return nil
}

func (m *MockTopology) addCpu(u uint) (power.Cpu, error) {
	ret := m.Called(u)

	var r0 power.Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *MockTopology) CPUs() *power.CpuList {
	ret := m.Called()

	var r0 *power.CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*power.CpuList)
	}

	return r0
}

func (m *MockTopology) Packages() *[]power.Package {
	ret := m.Called()

	var r0 *[]power.Package
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]power.Package)

	}
	return r0
}

func (m *MockTopology) Package(id uint) power.Package {
	ret := m.Called(id)

	var r0 power.Package
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Package)
	}

	return r0
}

type MockPackage struct {
	mock.Mock
	power.Package
}
type MockPackageList struct {
	mock.Mock
}

func (m *MockPackage) MakeList() []power.Package {
	return []power.Package{m}
}

func (m *MockPackage) GetID() uint {
	return m.Called().Get(0).(uint)
}

func (m *MockPackage) SetUncore(uncore power.Uncore) error {
	return m.Called(uncore).Error(0)
}

func (m *MockPackage) applyUncore() error {
	return m.Called().Error(0)
}

func (m *MockPackage) getEffectiveUncore() power.Uncore {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(power.Uncore)
	}
	return nil
}

func (m *MockPackage) addCpu(u uint) (power.Cpu, error) {
	ret := m.Called(u)

	var r0 power.Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *MockPackage) CPUs() *power.CpuList {
	ret := m.Called()

	var r0 *power.CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*power.CpuList)
	}

	return r0
}

func (m *MockPackage) Dies() *[]power.Die {
	ret := m.Called()

	var r0 *[]power.Die
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]power.Die)

	}
	return r0
}

func (m *MockPackage) Die(id uint) power.Die {
	ret := m.Called(id)

	var r0 power.Die
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Die)
	}

	return r0
}

type MockDie struct {
	mock.Mock
	power.Die
}

func (m *MockDie) MakeList() []power.Die {
	return []power.Die{m}
}

func (m *MockDie) GetID() uint {
	return m.Called().Get(0).(uint)
}

func (m *MockDie) SetUncore(uncore power.Uncore) error {
	return m.Called(uncore).Error(0)
}

func (m *MockDie) applyUncore() error {
	return m.Called().Error(0)
}

func (m *MockDie) getEffectiveUncore() power.Uncore {
	ret := m.Called()
	if ret.Get(0) != nil {
		return ret.Get(0).(power.Uncore)
	}
	return nil
}

func (m *MockDie) addCpu(u uint) (power.Cpu, error) {
	ret := m.Called(u)

	var r0 power.Cpu
	var r1 error

	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Cpu)
	}
	r1 = ret.Error(1)

	return r0, r1
}

func (m *MockDie) CPUs() *power.CpuList {
	ret := m.Called()

	var r0 *power.CpuList
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*power.CpuList)
	}

	return r0
}

func (m *MockDie) Cores() *[]power.Core {
	ret := m.Called()

	var r0 *[]power.Core
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*[]power.Core)
	}

	return r0
}

func (m *MockDie) Core(id uint) power.Core {
	ret := m.Called(id)

	var r0 power.Core
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(power.Core)
	}

	return r0
}

type FrequencySetMock struct {
	mock.Mock
	power.CpuFrequencySet
}

func (m *FrequencySetMock) GetMax() uint {
	return m.Called().Get(0).(uint)
}

func (m *FrequencySetMock) GetMin() uint {
	return m.Called().Get(0).(uint)
}

func SetupDummyFiles(cores int, packages int, diesPerPackage int, cpufiles map[string]string) (power.Host, func(), error) {
	//variables for various files
	path := "testing/cpus"
	pStatesDrvFile := "cpufreq/scaling_driver"

	cpuMaxFreqFile := "cpufreq/cpuinfo_max_freq"
	cpuMinFreqFile := "cpufreq/cpuinfo_min_freq"
	scalingMaxFile := "cpufreq/scaling_max_freq"
	scalingMinFile := "cpufreq/scaling_min_freq"
	scalingGovFile := "cpufreq/scaling_governor"
	availGovFile := "cpufreq/scaling_available_governors"
	eppFile := "cpufreq/energy_performance_preference"
	cpuTopologyDir := "topology/"
	packageIdFile := cpuTopologyDir + "physical_package_id"
	dieIdFile := cpuTopologyDir + "die_id"
	coreIdFile := cpuTopologyDir + "core_id"
	uncoreDir := path + "/intel_uncore_frequency/"
	uncoreInitMaxFreqFile := "initial_max_freq_khz"
	uncoreInitMinFreqFile := "initial_min_freq_khz"
	uncoreMaxFreqFile := "max_freq_khz"
	uncoreMinFreqFile := "min_freq_khz"
	cstates := []string{"C0", "C1", "C1E", "C2", "C3", "6"}
	// if we're setting uncore we need to spoof the module being loaded
	_, ok := cpufiles["uncore_max"]
	if ok {
		os.Mkdir("testing", os.ModePerm)
		os.WriteFile("testing/proc.modules", []byte("intel_uncore_frequency"+"\n"), 0644)
		os.MkdirAll(filepath.Join(uncoreDir, "package_00_die_00"), os.ModePerm)
	}
	die := 0
	pkg := 0
	strPkg := "00"
	strDie := "00"
	pkgDir := "package_00_die_00/"
	increment := diesPerPackage * packages
	coresPerDie := cores / increment
	for i := 0; i < cores; i++ {
		cpuName := "cpu" + fmt.Sprint(i)
		cpudir := filepath.Join(path, cpuName)
		os.MkdirAll(filepath.Join(cpudir, "cpufreq"), os.ModePerm)
		os.MkdirAll(filepath.Join(cpudir, "topology"), os.ModePerm)
		//used to divide cores between packages and dies
		if i%coresPerDie == 0 && i != 0 && packages != 0 {
			if die == diesPerPackage-1 && pkg != (packages-1) {
				die = 0
				pkg++
			} else if die != (diesPerPackage - 1) {
				die++
			}

			if pkg > 10 {
				strPkg = fmt.Sprint(pkg)
			} else {
				strPkg = "0" + fmt.Sprint(pkg)
			}
			if die > 10 {
				strDie = fmt.Sprint(die)
			} else {
				strDie = "0" + fmt.Sprint(die)
			}
			pkgDir = "package_" + strPkg + "_die_" + strDie + "/"
			os.MkdirAll(filepath.Join(uncoreDir, pkgDir), os.ModePerm)
		}
		if packages != 0 {
			os.WriteFile(filepath.Join(cpudir, packageIdFile), []byte(fmt.Sprint(pkg)+"\n"), 0664)
			os.WriteFile(filepath.Join(cpudir, dieIdFile), []byte(fmt.Sprint(die)+"\n"), 0664)
			os.WriteFile(filepath.Join(cpudir, coreIdFile), []byte(fmt.Sprint(i)+"\n"), 0664)
		}
		for prop, value := range cpufiles {
			switch prop {
			case "driver":
				os.WriteFile(filepath.Join(cpudir, pStatesDrvFile), []byte(value+"\n"), 0664)
			case "max":
				os.WriteFile(filepath.Join(cpudir, scalingMaxFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMaxFreqFile), []byte(value+"\n"), 0644)
			case "min":
				os.WriteFile(filepath.Join(cpudir, scalingMinFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(cpudir, cpuMinFreqFile), []byte(value+"\n"), 0644)
			case "epp":
				os.WriteFile(filepath.Join(cpudir, eppFile), []byte(value+"\n"), 0644)
			case "governor":
				os.WriteFile(filepath.Join(cpudir, scalingGovFile), []byte(value+"\n"), 0644)
			case "available_governors":
				os.WriteFile(filepath.Join(cpudir, availGovFile), []byte(value+"\n"), 0644)
			case "uncore_max":
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreInitMaxFreqFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreMaxFreqFile), []byte(value+"\n"), 0644)
			case "uncore_min":
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreInitMinFreqFile), []byte(value+"\n"), 0644)
				os.WriteFile(filepath.Join(uncoreDir, pkgDir, uncoreMinFreqFile), []byte(value+"\n"), 0644)
			case "cstates":
				for i, state := range cstates {
					statedir := "cpuidle/state" + fmt.Sprint(i)
					os.MkdirAll(filepath.Join(cpudir, statedir), os.ModePerm)
					os.MkdirAll(filepath.Join(path, "cpuidle"), os.ModePerm)
					os.WriteFile(filepath.Join(path, "cpuidle", "current_driver"), []byte(value+"\n"), 0644)
					os.WriteFile(filepath.Join(cpudir, statedir, "name"), []byte(state+"\n"), 0644)
					os.WriteFile(filepath.Join(cpudir, statedir, "disable"), []byte("0\n"), 0644)
				}
			}

		}
	}
	host, err := power.CreateInstanceWithConf("test-node", power.LibConfig{CpuPath: "testing/cpus", ModulePath: "testing/proc.modules", Cores: uint(cores)})
	// Ignore if error comes from ESMI initialization - we want to run unit tests on any hardware, not only AMD EPYCs
	if strings.Contains(err.Error(), "ESMI") {
		err = nil
	}
	return host, func() {
		os.RemoveAll(strings.Split(path, "/")[0])
	}, err
}

// default dummy file system to be used in standard tests
func FullDummySystem() (power.Host, func(), error) {
	return SetupDummyFiles(86, 1, 2, map[string]string{
		"driver": "intel_pstate", "max": "3700000", "min": "1000000",
		"epp": "performance", "governor": "performance",
		"available_governors": "powersave performance",
		"uncore_max":          "2400000", "uncore_min": "1200000",
		"cstates": "intel_idle"})
}

// mock required for testing setupwithmanager
type ClientMock struct {
	mock.Mock
	client.Client
}

// mock required for testing client errs
type ErrClient struct {
	client.Client
	mock.Mock
}

func (e *ErrClient) Get(ctx context.Context, NamespacedName types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, NamespacedName, obj, opts).Error(0)
	}
	return e.Called(ctx, NamespacedName, obj).Error(0)
}
func (e *ErrClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, list, opts).Error(0)

	}
	return e.Called(ctx, list).Error(0)
}

func (e *ErrClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *ErrClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *ErrClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *ErrClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, opts).Error(0)
	}
	return e.Called(ctx, obj).Error(0)
}

func (e *ErrClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if len(opts) != 0 {
		return e.Called(ctx, obj, patch, opts).Error(0)
	}
	return e.Called(ctx, obj, patch).Error(0)
}

func (e *ErrClient) Status() client.SubResourceWriter {
	return e.Called().Get(0).(client.SubResourceWriter)
}

type MockResourceWriter struct {
	mock.Mock
	client.SubResourceWriter
}

func (m *MockResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if len(opts) != 0 {
		return m.Called(ctx, obj, opts).Error(0)
	}
	return m.Called(ctx, obj).Error(0)
}

type MgrMock struct {
	mock.Mock
	manager.Manager
}

func (m *MgrMock) Add(r manager.Runnable) error {
	return m.Called(r).Error(0)
}

func (m *MgrMock) Elected() <-chan struct{} {
	return m.Called().Get(0).(<-chan struct{})
}

func (m *MgrMock) AddMetricsExtraHandler(path string, handler http.Handler) error {
	return m.Called(path, handler).Get(0).(error)
}

func (m *MgrMock) AddHealthzCheck(name string, check healthz.Checker) error {
	return m.Called(name, check).Get(0).(error)
}

func (m *MgrMock) AddReadyzCheck(name string, check healthz.Checker) error {
	return m.Called(name, check).Get(0).(error)
}

func (m *MgrMock) Start(ctx context.Context) error {
	return m.Called(ctx).Get(0).(error)
}

func (m *MgrMock) GetWebhookServer() webhook.Server {
	return m.Called().Get(0).(webhook.Server)
}

func (m *MgrMock) GetLogger() logr.Logger {
	return m.Called().Get(0).(logr.Logger)

}

func (m *MgrMock) GetControllerOptions() config.Controller {
	return m.Called().Get(0).(config.Controller)
}

func (m *MgrMock) GetScheme() *runtime.Scheme {
	return m.Called().Get(0).(*runtime.Scheme)
}
func (m *MgrMock) SetFields(i interface{}) error {
	return m.Called(i).Error(0)
}

func (m *MgrMock) GetCache() cache.Cache {
	return m.Called().Get(0).(cache.Cache)
}

type CacheMk struct {
	cache.Cache
	mock.Mock
}
