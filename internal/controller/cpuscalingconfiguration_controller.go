/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/kubernetes-power-manager/internal/scaling"
	"github.com/intel/power-optimization-library/pkg/power"
)

type dpdkTelemetryConfiguration struct {
	current *metrics.DPDKTelemetryConnectionData
	new     *metrics.DPDKTelemetryConnectionData
}

// CPUScalingConfigurationReconciler reconciles a CPUScalingConfiguration object
type CPUScalingConfigurationReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	PowerLibrary        power.Host
	CPUScalingManager   scaling.CPUScalingManager
	DPDKTelemetryClient metrics.DPDKTelemetryClient
}

var (
	minSamplePeriod = time.Duration(10 * time.Millisecond)
	maxSamplePeriod = time.Duration(1 * time.Second)
)

//+kubebuilder:rbac:groups=power.amdepyc.com,resources=cpuscalingconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=power.amdepyc.com,resources=cpuscalingconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=power.amdepyc.com,resources=cpuscalingconfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *CPUScalingConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	nodeName := os.Getenv("NODE_NAME")

	// check if this config belongs to the current node
	if req.Name != nodeName {
		return ctrl.Result{}, nil
	}

	logger := r.Log.WithValues("cpuscalingconfiguration", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the power-manager namespace, ignoring")
		// NOTE: Returning error is not the correct way to refuse reconciliation as
		// it will not prevent requeueing. But it is used regardless because
		// it allows testing this specific condition.
		return ctrl.Result{}, err
	}

	config := &powerv1.CPUScalingConfiguration{}
	defer func() { _ = writeUpdatedStatusErrsIfRequired(ctx, r.Status(), config, err) }()

	err = r.Client.Get(context.TODO(), req.NamespacedName, config)
	logger.V(5).Info("retrieving the cpu scaling configuration instance")
	if err != nil {
		if errors.IsNotFound(err) {
			r.CPUScalingManager.UpdateConfig([]scaling.CPUScalingOpts{})

			for _, connData := range r.DPDKTelemetryClient.ListConnections() {
				r.DPDKTelemetryClient.CloseConnection(connData.PodUID)
			}

			return ctrl.Result{}, nil
		}

		logger.Error(err, "could not retrieve the cpu scaling configuration instance")
		return ctrl.Result{}, err
	}

	// validate values
	err = r.validateCPUIDs(config.Spec.Items)
	if err != nil {
		logger.Error(err, "error validating cpu ids")
		return ctrl.Result{}, nil
	}
	err = r.validateSamplePeriods(config.Spec.Items)
	if err != nil {
		logger.Error(err, "error validating sample periods")
		return ctrl.Result{}, nil
	}
	err = r.validateCooldownPeriods(config.Spec.Items)
	if err != nil {
		logger.Error(err, "error validating cooldown periods")
		return ctrl.Result{}, nil
	}

	r.reconcileDPDKTelemetryClient(config.Spec.Items)

	r.CPUScalingManager.UpdateConfig(
		r.parseConfig(config.Spec.Items),
	)

	return ctrl.Result{}, nil
}

func (r *CPUScalingConfigurationReconciler) validateCPUIDs(configItems []powerv1.ConfigItem) error {
	nodeName := os.Getenv("NODE_NAME")
	availableCPUs := r.PowerLibrary.GetAllCpus().IDs()
	affectedCPUs := []uint{}

	for _, item := range configItems {
		for _, cpuID := range item.CpuIDs {
			if !slices.Contains(availableCPUs, cpuID) {
				return fmt.Errorf("cpu with id %d is not available on node %s", cpuID, nodeName)
			}
			if slices.Contains(affectedCPUs, cpuID) {
				return fmt.Errorf("cpu with id %d is specified more than once on node %s", cpuID, nodeName)
			}
			affectedCPUs = append(affectedCPUs, cpuID)
		}
	}

	return nil
}

func (r *CPUScalingConfigurationReconciler) validateSamplePeriods(configItems []powerv1.ConfigItem) error {
	for _, item := range configItems {
		samplePeriod := item.SamplePeriod.Duration
		if samplePeriod < minSamplePeriod {
			return fmt.Errorf("sample period %s is below minimum limit %s", samplePeriod, minSamplePeriod)
		}
		if samplePeriod > maxSamplePeriod {
			return fmt.Errorf("sample period %s is above maximum limit %s", samplePeriod, maxSamplePeriod)
		}
	}

	return nil
}

func (r *CPUScalingConfigurationReconciler) validateCooldownPeriods(configItems []powerv1.ConfigItem) error {
	for _, item := range configItems {
		cooldownPeriod := item.CooldownPeriod.Duration
		samplePeriod := item.SamplePeriod.Duration
		if cooldownPeriod < samplePeriod {
			return fmt.Errorf("cooldown period %s must be larger than sample period %s", cooldownPeriod, samplePeriod)
		}
	}

	return nil
}

func (r *CPUScalingConfigurationReconciler) parseConfig(configItems []powerv1.ConfigItem) []scaling.CPUScalingOpts {
	optsList := make([]scaling.CPUScalingOpts, 0)

	for _, item := range configItems {
		minFreq, maxFreq, err := getMaxMinFrequencyValues(r.PowerLibrary)
		if err != nil {
			r.Log.Error(err, "failed to get max and min frequency")
			continue
		}
		minFreq *= 1000
		maxFreq *= 1000
		fallbackFreq := scaling.GetFrequencyFromPercent(minFreq, maxFreq, item.FallbackFreqPercent)

		for _, cpuID := range item.CpuIDs {
			opts := scaling.CPUScalingOpts{
				CPUID:                      cpuID,
				SamplePeriod:               item.SamplePeriod.Duration,
				CooldownPeriod:             item.CooldownPeriod.Duration,
				TargetBusyness:             item.TargetBusyness,
				AllowedBusynessDifference:  item.AllowedBusynessDifference,
				AllowedFrequencyDifference: item.AllowedFrequencyDifference * 1000,
				HWMaxFrequency:             maxFreq,
				HWMinFrequency:             minFreq,
				CurrentTargetFrequency:     scaling.FrequencyNotYetSet,
				ScaleFactor:                float64(item.ScalePercentage) / 100.0,
				FallbackFreq:               fallbackFreq,
			}
			optsList = append(optsList, opts)
		}
	}

	return optsList
}

func (r *CPUScalingConfigurationReconciler) reconcileDPDKTelemetryClient(configItems []powerv1.ConfigItem) {
	// gather current connection configurations
	dpdkConfigMap := map[string]*dpdkTelemetryConfiguration{}
	for _, connData := range r.DPDKTelemetryClient.ListConnections() {
		dpdkConfigMap[connData.PodUID] = &dpdkTelemetryConfiguration{
			current: &connData,
			new:     nil,
		}
	}

	// gather incoming connection configurations and group them based on pod UID
	for _, configItem := range configItems {
		podUID := string(configItem.PodUID)
		if dpdkConfig, found := dpdkConfigMap[podUID]; found {
			if dpdkConfig.new == nil {
				dpdkConfig.new = &metrics.DPDKTelemetryConnectionData{
					PodUID:      podUID,
					WatchedCPUs: configItem.CpuIDs,
				}
			} else {
				dpdkConfig.new.WatchedCPUs = append(dpdkConfig.new.WatchedCPUs, configItem.CpuIDs...)
			}
		} else {
			dpdkConfigMap[podUID] = &dpdkTelemetryConfiguration{
				current: nil,
				new: &metrics.DPDKTelemetryConnectionData{
					PodUID:      podUID,
					WatchedCPUs: configItem.CpuIDs,
				},
			}
		}
	}

	// NOTE: Exclusive CPUs for containers are fixed
	// at Pod scheduling and don't change.
	// Thus, we can skip updating the CPU list post-connection.
	for podUID, dpdkConfig := range dpdkConfigMap {
		if dpdkConfig.new == nil {
			r.DPDKTelemetryClient.CloseConnection(podUID)
			continue
		}
		if dpdkConfig.current == nil {
			r.DPDKTelemetryClient.CreateConnection(dpdkConfig.new)
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *CPUScalingConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.CPUScalingConfiguration{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
