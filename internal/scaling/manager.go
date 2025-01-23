package scaling

import (
	"context"
	"os"
	"sync"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/AMDEPYC/kubernetes-power-manager/internal/metrics"
	"github.com/intel/power-optimization-library/pkg/power"
)

// Func definitions for unit testing
var (
	newCPUScalingWorkerFunc = NewCPUScalingWorker
)

type CPUScalingManager interface {
	manager.Runnable
	UpdateConfig(optList []CPUScalingOpts)
}

type cpuScalingManagerImpl struct {
	powerLibrary *power.Host
	dpdkClient   metrics.DPDKTelemetryClient
	workers      sync.Map
	logger       logr.Logger
}

func NewCPUScalingManager(powerLib *power.Host, dpdkClient metrics.DPDKTelemetryClient) CPUScalingManager {
	nodeName := os.Getenv("NODE_NAME")

	mgr := &cpuScalingManagerImpl{
		powerLibrary: powerLib,
		dpdkClient:   dpdkClient,
		logger:       ctrl.Log.WithName("CPUScalingManager").WithName(nodeName),
	}

	return mgr
}

func (s *cpuScalingManagerImpl) Start(ctx context.Context) error {
	<-ctx.Done()
	s.stop()
	return nil
}

func (s *cpuScalingManagerImpl) stop() {
	s.logger.V(5).Info("stopping all workers")

	managedCPUs := s.getManagedCPUIDs()

	for _, cpuID := range managedCPUs {
		worker, found := s.workers.LoadAndDelete(cpuID)
		if found {
			worker := worker.(CPUScalingWorker)
			worker.Stop()
			s.logger.V(5).Info("worker stopped successfully", "cpuID", cpuID)
		}
	}

	s.logger.V(5).Info("successfully stopped all")
}

func (s *cpuScalingManagerImpl) UpdateConfig(optsList []CPUScalingOpts) {
	incomingManagedCPUs := map[uint]struct{}{}
	currentManagedCPUs := s.getManagedCPUIDs()

	// create or update workers as per new config
	for _, opts := range optsList {
		incomingManagedCPUs[opts.CPUID] = struct{}{}

		worker, found := s.getCPUScalingWorker(opts.CPUID)
		if !found {
			s.logger.V(5).Info("creating worker", "cpuID", opts.CPUID)

			s.workers.Store(
				opts.CPUID,
				newCPUScalingWorkerFunc(
					opts.CPUID,
					s.powerLibrary,
					s.dpdkClient,
					&opts,
				),
			)
		} else {
			worker.UpdateOpts(&opts)
		}
	}

	// stop workers on cpus that are no longer managed
	for _, cpuID := range currentManagedCPUs {
		if _, contains := incomingManagedCPUs[cpuID]; !contains {
			s.logger.V(5).Info("stopping  worker", "cpuID", cpuID)

			worker, found := s.workers.LoadAndDelete(cpuID)
			if !found {
				s.logger.V(5).Info("worker already stopped", "cpuID", cpuID)
			} else {
				worker := worker.(CPUScalingWorker)
				worker.Stop()
				s.logger.V(5).Info("worker stopped successfully", "cpuID", cpuID)
			}
		}
	}
}

func (s *cpuScalingManagerImpl) getManagedCPUIDs() []uint {
	managedCPUs := make([]uint, 0)
	s.workers.Range(func(key, value any) bool {
		managedCPUs = append(managedCPUs, key.(uint))
		return true
	})

	return managedCPUs
}

func (s *cpuScalingManagerImpl) getCPUScalingWorker(cpuID uint) (CPUScalingWorker, bool) {
	if value, found := s.workers.Load(cpuID); found {
		return value.(CPUScalingWorker), true
	}

	return nil, false
}
