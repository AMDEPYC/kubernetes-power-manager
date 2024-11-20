package scaling

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"
)

var (
	testHookStopLoop func() bool
)

type CPUScalingWorker interface {
	UpdateOpts(opts *CPUScalingOpts)
	Stop()
}

type cpuScalingWorkerImpl struct {
	cpuID      uint
	opts       atomic.Pointer[CPUScalingOpts]
	cancelFunc func()
	waitGroup  sync.WaitGroup
	updater    CPUScalingUpdater
}

func NewCPUScalingWorker(
	cpuID uint,
	powerLib *power.Host,
	dpdkClient metrics.DPDKTelemetryClient,
	opts *CPUScalingOpts,
) CPUScalingWorker {
	ctx, cancelFunc := context.WithCancel(context.Background())

	worker := &cpuScalingWorkerImpl{
		cpuID:      cpuID,
		cancelFunc: cancelFunc,
		waitGroup:  sync.WaitGroup{},
	}

	worker.opts.Store(opts)
	worker.updater = NewCPUScalingUpdater(powerLib, dpdkClient)
	worker.waitGroup.Add(1)

	go worker.runLoop(ctx)

	return worker
}

func (w *cpuScalingWorkerImpl) UpdateOpts(opts *CPUScalingOpts) {
	w.opts.Store(opts)
}

func (w *cpuScalingWorkerImpl) Stop() {
	w.cancelFunc()
	w.waitGroup.Wait()
}

func (w *cpuScalingWorkerImpl) runLoop(ctx context.Context) {
	defer w.waitGroup.Done()

	for {
		if testHookStopLoop != nil {
			if testHookStopLoop() {
				return
			}
		}

		opts := w.opts.Load()
		select {
		case <-ctx.Done():
			return
		case <-time.After(opts.SamplePeriod):
			w.updater.Update(opts)
		}
	}
}
