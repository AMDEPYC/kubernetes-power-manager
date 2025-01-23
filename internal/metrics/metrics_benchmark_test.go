package metrics

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"testing"

	"github.com/AMDEPYC/kubernetes-power-manager/pkg/util"
	"github.com/intel/power-optimization-library/pkg/power"

	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Benchmarks need to run with root priviliges on baremetal Linux server.
// To run: 'go test -bench=. -run='^#' ./internal/metrics/...' - adjust path if needed

var setupLog = ctrl.Log.WithName("setup")

func BenchmarkMSRClient(b *testing.B) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))

	power.SetLogger(ctrl.Log.WithName("powerLibrary"))
	hostname, err := os.Hostname()
	if err != nil {
		setupLog.Error(err, "cannot retrieve hostname")
		os.Exit(1)
	}
	powerLibrary, err := power.CreateInstance(hostname)
	if powerLibrary == nil {
		setupLog.Error(err, "unable to create Power Library instance")
		os.Exit(1)
	}
	perfEventClient := NewPerfEventClient(
		ctrl.Log.WithName("metrics").WithName("perf"),
		powerLibrary,
	)
	defer perfEventClient.Close()

	// TODO: select methods for benchmarking that will be used in optimization loop
	methods := []func(cpu power.Cpu) (uint64, error){
		perfEventClient.GetCycles,
		perfEventClient.GetInstructions,
		perfEventClient.GetBranchMisses,
		perfEventClient.GetStalledCyclesFrontend,
	}

	for _, f := range methods {
		method := runtime.FuncForPC(reflect.ValueOf(f).Pointer())
		b.Run(fmt.Sprintf("PerfEventClient method name: %s", method.Name()), func(b *testing.B) {
			// Use randCPU defined below if testing fetch time on single CPU
			// topology := powerLibrary.Topology()
			// randCPU := topology.CPUs().ByID(uint(rand.IntN(len(*topology.CPUs()))))
			for i := 0; i < b.N; i++ {
				util.IterateOverCPUs(powerLibrary, func(cpu power.Cpu, _ power.Core, _ power.Die, _ power.Package) {
					f(cpu)
				})
			}
		})
	}
}
