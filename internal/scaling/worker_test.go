package scaling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type workerMock struct {
	mock.Mock
	cpuID uint
	opts  *CPUScalingOpts
}

func (w *workerMock) UpdateOpts(opts *CPUScalingOpts) {
	w.Called(opts)
	w.opts = opts

}

func (w *workerMock) Stop() {
	w.Called()
}

func CreateMockWorker(cpuID uint, opts *CPUScalingOpts) *workerMock {
	w := &workerMock{
		cpuID: cpuID,
		opts:  opts,
	}
	return w
}

func TestCPUScalingWorker_UpdateOpts(t *testing.T) {
	expectedOpts := &CPUScalingOpts{
		SamplePeriod: 10 * time.Millisecond,
	}
	wrk := &cpuScalingWorkerImpl{}

	wrk.UpdateOpts(expectedOpts)
	assert.Equal(t, expectedOpts, wrk.opts.Load())
}

func TestCPUScalingWorker_Stop(t *testing.T) {
	cancelFuncCalled := false
	wrk := &cpuScalingWorkerImpl{
		waitGroup:  sync.WaitGroup{},
		cancelFunc: func() { cancelFuncCalled = true },
	}

	wrk.Stop()

	assert.True(t, cancelFuncCalled)
}

func TestCPUScalingWorker_runLoop(t *testing.T) {
	loopCounter := 0
	t.Cleanup(func() {
		testHookStopLoop = nil
	})
	testHookStopLoop = func() bool {
		loopCounter++
		return loopCounter > 1
	}

	wrk := &cpuScalingWorkerImpl{
		waitGroup: sync.WaitGroup{},
	}

	opts := &CPUScalingOpts{
		SamplePeriod: time.Millisecond,
	}
	wrk.opts.Store(opts)

	upd := &updaterMock{}
	upd.On("Update", opts).Return(opts.SamplePeriod)
	wrk.updater = upd
	wrk.waitGroup.Add(1)

	wrk.runLoop(context.TODO())

	upd.AssertCalled(t, "Update", opts)
	assert.Panics(t, wrk.waitGroup.Done)
}
