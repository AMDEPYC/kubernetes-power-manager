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
	wrk.waitGroup.Add(1)
	doneCh := make(chan struct{})

	go func() {
		wrk.Stop()
		close(doneCh)
	}()

	// give goroutine time to start up
	time.Sleep(50 * time.Millisecond)

	select {
	case <-doneCh:
		t.Fatal("Function returned early - expected to be blocking")
	default:
	}

	wrk.waitGroup.Done()

	select {
	case <-doneCh:
		// function unblocked properly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Function did not unblock properly after context was canceled.")
	}

	assert.True(t, cancelFuncCalled)
}

func TestCPUScalingWorker_runLoop(t *testing.T) {
	wrk := &cpuScalingWorkerImpl{
		waitGroup: sync.WaitGroup{},
	}
	opts := &CPUScalingOpts{
		SamplePeriod: 10 * time.Millisecond,
	}
	wrk.opts.Store(opts)
	upd := &updaterMock{}
	upd.On("Update", opts).Return()
	wrk.updater = upd

	wrk.waitGroup.Add(1)
	ctx, cancel := context.WithCancel(context.TODO())
	doneCh := make(chan struct{})

	go func() {
		wrk.runLoop(ctx)
		close(doneCh)
	}()

	// give goroutine time to start up
	time.Sleep(50 * time.Millisecond)

	select {
	case <-doneCh:
		t.Fatal("Function returned early - expected to be blocking")
	default:
	}

	cancel()

	select {
	case <-doneCh:
		// function unblocked properly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Function did not unblock properly after context was canceled.")
	}

	upd.AssertCalled(t, "Update", opts)
}
