package scaling

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func createNewCPUScalingManager() cpuScalingManagerImpl {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	))

	return cpuScalingManagerImpl{
		logger: ctrl.Log.WithName("test-log"),
	}
}

func TestCPUScalingManager_Start(t *testing.T) {
	mgr := createNewCPUScalingManager()
	w := &workerMock{}
	w.On("Stop").Return()
	mgr.workers.Store(uint(0), w)

	ctx, cancel := context.WithCancel(context.TODO())

	cancel()
	mgr.Start(ctx)

	w.AssertCalled(t, "Stop")
}

func TestCPUScalingManager_UpdateConfig(t *testing.T) {
	origNewCPUScalingWorkerFunc := newCPUScalingWorkerFunc
	t.Cleanup(func() {
		newCPUScalingWorkerFunc = origNewCPUScalingWorkerFunc
	})
	newCPUScalingWorkerFunc = func(
		cpuID uint,
		_ *power.Host,
		dpdkClient metrics.DPDKTelemetryClient,
		opts *CPUScalingOpts,
	) CPUScalingWorker {
		w := CreateMockWorker(cpuID, opts)
		return w
	}

	tcases := []struct {
		testCase        string
		initialConfig   []CPUScalingOpts
		remainingCPUIDs []uint
		newConfig       []CPUScalingOpts
	}{
		{
			testCase:        "Test Case 1 - New Workers in an empty worker pool",
			initialConfig:   []CPUScalingOpts{},
			remainingCPUIDs: []uint{},
			newConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 10 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 100 * time.Millisecond,
				},
			},
		},
		{
			testCase: "Test Case 2 - New Workers extending already populated worker pool",
			initialConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 50 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 500 * time.Millisecond,
				},
			},
			remainingCPUIDs: []uint{0, 1},
			newConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 10 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 100 * time.Millisecond,
				},
				{
					CPUID:        2,
					SamplePeriod: 100 * time.Millisecond,
				},
				{
					CPUID:        3,
					SamplePeriod: 100 * time.Millisecond,
				},
			},
		},
		{
			testCase: "Test Case 3 - Updating existing workers",
			initialConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 50 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 500 * time.Millisecond,
				},
			},
			remainingCPUIDs: []uint{0, 1},
			newConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 10 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 100 * time.Millisecond,
				},
			},
		},
		{
			testCase: "Test Case 4 - Removing some workers from existing pool",
			initialConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 10 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 100 * time.Millisecond,
				},
				{
					CPUID:        2,
					SamplePeriod: 100 * time.Millisecond,
				},
				{
					CPUID:        3,
					SamplePeriod: 100 * time.Millisecond,
				},
			},
			remainingCPUIDs: []uint{0, 1},
			newConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 10 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 100 * time.Millisecond,
				},
			},
		},
		{
			testCase: "Test Case 5 - Removing all workers from existing pool",
			initialConfig: []CPUScalingOpts{
				{
					CPUID:        0,
					SamplePeriod: 10 * time.Millisecond,
				},
				{
					CPUID:        1,
					SamplePeriod: 100 * time.Millisecond,
				},
				{
					CPUID:        2,
					SamplePeriod: 100 * time.Millisecond,
				},
				{
					CPUID:        3,
					SamplePeriod: 100 * time.Millisecond,
				},
			},
			remainingCPUIDs: []uint{},
			newConfig:       []CPUScalingOpts{},
		},
	}

	for _, tc := range tcases {
		t.Log(tc.testCase)

		mgr := createNewCPUScalingManager()

		// create workers from initial configuration
		initialWorkers := make(map[uint]*workerMock, 0)
		for _, opt := range tc.initialConfig {
			w := CreateMockWorker(opt.CPUID, &opt)
			// set up Stap call for workers that should get removed
			if !slices.Contains(tc.remainingCPUIDs, opt.CPUID) {
				w.On("Stop").Return()
			}
			initialWorkers[opt.CPUID] = w
			mgr.workers.Store(opt.CPUID, w)
		}
		// set up UpdateOpts call for workers that should get updated
		for _, opt := range tc.newConfig {
			if slices.Contains(tc.remainingCPUIDs, opt.CPUID) {
				initialWorkers[opt.CPUID].On("UpdateOpts", &opt).Return()
			}
		}

		mgr.UpdateConfig(tc.newConfig)

		// assert all workers from options were created
		// and have correct values
		for _, opt := range tc.newConfig {
			w, found := mgr.getCPUScalingWorker(opt.CPUID)
			assert.True(t, found)
			typedW := w.(*workerMock)
			assert.Equal(t, &opt, typedW.opts)
		}
		// assert all initial workers were either stopped or updated
		for _, opt := range tc.initialConfig {
			w := initialWorkers[opt.CPUID]
			if !slices.Contains(tc.remainingCPUIDs, opt.CPUID) {
				w.AssertCalled(t, "Stop")
			}
		}
		for _, opt := range tc.newConfig {
			if slices.Contains(tc.remainingCPUIDs, opt.CPUID) {
				initialWorkers[opt.CPUID].AssertCalled(t, "UpdateOpts", &opt)
			}
		}
	}
}
