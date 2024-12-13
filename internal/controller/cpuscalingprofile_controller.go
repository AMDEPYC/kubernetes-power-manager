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
	"reflect"
	"slices"
	"strings"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CPUScalingProfileReconciler reconciles a CPUScalingProfile object
type CPUScalingProfileReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=power.intel.com,resources=cpuscalingprofiles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=power.intel.com,resources=cpuscalingprofiles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=power.intel.com,resources=cpuscalingprofiles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *CPUScalingProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	logger := r.Log.WithValues("cpuscalingprofile", req.NamespacedName)

	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the power-manager namespace, ignoring")
		return ctrl.Result{}, err
	}

	scalingProfile := &powerv1.CPUScalingProfile{}
	defer func() { _ = writeUpdatedStatusErrsIfRequired(ctx, r.Status(), scalingProfile, err) }()

	err = r.Client.Get(context.TODO(), req.NamespacedName, scalingProfile)
	logger.V(5).Info("retrieving the cpuscalingprofile instance")
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "error retrieving reconciled cpuscalingprofile")
			return ctrl.Result{}, err
		}

		logger.V(5).Info("cpuscalingprofile profile not found")

		// Owned PowerProfile will be automatically deleted by K8s GC

		// Search for CPUScalingProfile in CPUScalingConfigurations to delete associated entries
		cpuScalingConfList := &powerv1.CPUScalingConfigurationList{}
		err = r.Client.List(context.TODO(), cpuScalingConfList)
		logger.V(5).Info("retrieving the cpuscalingconfiguration list")
		if err != nil {
			logger.Error(err, "error retrieving cpuscalingconfiguration list")
			return ctrl.Result{}, err
		}

		// Set Name and Namespace so helper func controllerutil.RemoveOwnerReference can be used
		scalingProfile = &powerv1.CPUScalingProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      req.Name,
				Namespace: req.Namespace,
			},
		}
		for _, cpuScalingConf := range cpuScalingConfList.Items {
			if err = r.updateOrDeleteCPUScalingConfiguration(
				&cpuScalingConf, []powerv1.ConfigItem{}, scalingProfile, logger); err != nil {
				logger.Error(err, "error while cleaning after deleted cpuscalingprofile")
				return ctrl.Result{}, err
			}
		}
		logger.V(5).Info("succesfully cleaned after deleted cpuscalingprofile")

		return ctrl.Result{}, nil
	}

	// Ideally, verification should be in admission webhook
	err = r.verifyCPUScalingProfileParams(&scalingProfile.Spec)
	if err != nil {
		err = errors.NewServiceUnavailable(err.Error())
		logger.Error(err, "")
		return ctrl.Result{}, nil
	}

	if scalingProfile.Spec.Epp == "" {
		scalingProfile.Spec.Epp = powerv1.EPPPower
	}

	if scalingProfile.Spec.SamplePeriod == nil {
		scalingProfile.Spec.SamplePeriod = eppDefaults[scalingProfile.Spec.Epp].cpuScalingProfileSpec.SamplePeriod
	}
	if scalingProfile.Spec.Min == nil {
		scalingProfile.Spec.Min = eppDefaults[scalingProfile.Spec.Epp].cpuScalingProfileSpec.Min
	}
	if scalingProfile.Spec.Max == nil {
		scalingProfile.Spec.Max = eppDefaults[scalingProfile.Spec.Epp].cpuScalingProfileSpec.Max
	}

	err = r.createOrUpdatePowerProfile(scalingProfile, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Search for CPUs associated with CPUScalingProfile in PowerWorkloads
	powerWorkloadList := &powerv1.PowerWorkloadList{}
	err = r.Client.List(context.TODO(), powerWorkloadList)
	logger.V(5).Info("retrieving the powerworkload list")
	if err != nil {
		logger.Error(err, "error retrieving powerworkload list")
		return ctrl.Result{}, err
	}
	nodesItems := r.createConfigItems(powerWorkloadList, scalingProfile)

	// Create, delete or update CPUScalingConfiguration on nodes on which CPUScalingProfile is requested
	for nodeName, items := range nodesItems {
		confKey := client.ObjectKey{Name: nodeName, Namespace: IntelPowerNamespace}
		cpuScalingConf := &powerv1.CPUScalingConfiguration{}
		err := r.Client.Get(context.TODO(), confKey, cpuScalingConf)
		// CPUScalingConfiguration could not be retrieved
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "error retrieving cpuscalingconfiguration")
				return ctrl.Result{}, err
			}

			cpuScalingConf = &powerv1.CPUScalingConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: IntelPowerNamespace,
				},
			}
			if err = r.setCPUScalingConfiguration(cpuScalingConf, items, scalingProfile); err != nil {
				return ctrl.Result{}, err
			}

			if len(cpuScalingConf.Spec.Items) == 0 {
				logger.V(5).Info("not creating cpuscalingconfiguration, spec would be empty")
				continue
			}

			if err := r.Client.Create(context.TODO(), cpuScalingConf); err != nil {
				err = fmt.Errorf("error creating cpuscalingconfiguration: %w", err)
				logger.Error(err, "")
				return ctrl.Result{}, err
			}

			logger.V(5).Info("cpuscalingconfiguration successfully created")
			continue
		}

		if err = r.updateOrDeleteCPUScalingConfiguration(cpuScalingConf, items, scalingProfile, logger); err != nil {
			logger.Error(err, "error while reconciling after cluster state change")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// createOrUpdatePowerProfile creates or updates existing PowerProfile Owned by CPUScalingProfile.
// PowerProfile spec is taken directly from CPUScalingProfile spec with Shared and Governor fields hardcoded.
func (r *CPUScalingProfileReconciler) createOrUpdatePowerProfile(scalingProfile *powerv1.CPUScalingProfile,
	logger logr.Logger) error {
	logger = logger.WithValues("powerprofile", scalingProfile.Name)

	powerProfile := &powerv1.PowerProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scalingProfile.Name,
			Namespace: scalingProfile.Namespace,
		},
	}

	op, err := ctrl.CreateOrUpdate(context.TODO(), r.Client, powerProfile, func() error {
		// If PowerProfile exists, check if CPUScalingProfile is its owner, if not, stop reconciliation
		if !powerProfile.ObjectMeta.CreationTimestamp.IsZero() {
			if !metav1.IsControlledBy(powerProfile, scalingProfile) {
				return fmt.Errorf(
					"reconciled cpuscalingprofile is not owner of this powerprofile, stopping reconciliation")
			}
		}

		powerProfile.Spec = powerv1.PowerProfileSpec{
			Name:     scalingProfile.Name,
			Max:      scalingProfile.Spec.Max,
			Min:      scalingProfile.Spec.Min,
			Governor: userspaceGovernor,
			Shared:   false,
			Epp:      scalingProfile.Spec.Epp,
		}

		if err := controllerutil.SetControllerReference(scalingProfile, powerProfile, r.Scheme); err != nil {
			return fmt.Errorf("error setting up ownership of powerprofile: %w", err)
		}

		return nil
	})
	if err != nil {
		err := fmt.Errorf("error during powerprofile CreateOrUpdate operation: %w", err)
		logger.Error(err, "")
		return err
	}
	switch op {
	case controllerutil.OperationResultNone:
		logger.V(5).Info("powerprofile already in desired state")
	case controllerutil.OperationResultCreated:
		logger.V(5).Info("powerprofile successfully created")
	case controllerutil.OperationResultUpdated:
		logger.V(5).Info("powerprofile successfully updated")
	}

	return nil
}

// Runtime verification of CPUScalingProfile spec parameters. Ideally this should be implemented by Admission Webhook.
func (r *CPUScalingProfileReconciler) verifyCPUScalingProfileParams(scalingSpec *powerv1.CPUScalingProfileSpec,
) error {
	errMsg := "cpuscalingprofile spec is not correct"
	if scalingSpec.SamplePeriod != nil {
		if scalingSpec.SamplePeriod.Duration < minSamplePeriod || scalingSpec.SamplePeriod.Duration > maxSamplePeriod {
			return fmt.Errorf("%s: SamplePeriod must be larger than %d and lower than %d",
				errMsg, minSamplePeriod, maxSamplePeriod)
		}
	}

	if (scalingSpec.Max == nil && scalingSpec.Min != nil) || (scalingSpec.Max != nil && scalingSpec.Min == nil) {
		return fmt.Errorf("%s: Max and Min frequency values must be both provided or omitted", errMsg)
	}

	if scalingSpec.Max != nil && scalingSpec.Min != nil {
		if scalingSpec.Max.Type != scalingSpec.Min.Type {
			return fmt.Errorf("%s: Max and Min frequency values must be both numeric or percentage", errMsg)
		}
		max, err := intstr.GetScaledValueFromIntOrPercent(scalingSpec.Max, 100, true)
		if err != nil {
			return fmt.Errorf("%s: Max is not correct: %w", errMsg, err)
		}
		min, err := intstr.GetScaledValueFromIntOrPercent(scalingSpec.Min, 100, true)
		if err != nil {
			return fmt.Errorf("%s: Min is not correct: %w", errMsg, err)
		}
		if min > max {
			return fmt.Errorf("%s: Min must be lower or equal to Max", errMsg)
		}
	}

	return nil
}

// updateOrDeleteCPUScalingConfiguration sets existing CPUScalingConfiguration with newItems and then decide if
// CPUScalingConfiguration should be updated or deleted in apiserver.
func (r *CPUScalingProfileReconciler) updateOrDeleteCPUScalingConfiguration(
	scalingConfig *powerv1.CPUScalingConfiguration, newItems []powerv1.ConfigItem, scalingProfile metav1.Object,
	logger logr.Logger) error {
	logger = logger.WithValues("cpuscalingconfiguration", scalingConfig.Name)

	origScalingConfig := scalingConfig.DeepCopy()
	if err := r.setCPUScalingConfiguration(scalingConfig, newItems, scalingProfile); err != nil {
		err = fmt.Errorf("error modifying cpuscalingconfiguration object: %w", err)
		logger.Error(err, "")
		return err
	}

	if len(scalingConfig.Spec.Items) == 0 {
		if err := r.Client.Delete(context.TODO(), scalingConfig); err != nil {
			err = fmt.Errorf("error deleting cpuscalingconfiguration from the cluster: %w", err)
			logger.Error(err, "")
			return err
		}
		logger.V(5).Info(
			"succesfully deleted empty cpuscalingconfiguration",
		)
		return nil
	}

	// Guard to not call unnecessary Update
	if reflect.DeepEqual(origScalingConfig, scalingConfig) {
		logger.V(5).Info("cpuscalingconfiguration is already in desired state")
		return nil
	}

	if err := r.Client.Update(context.TODO(), scalingConfig); err != nil {
		err = fmt.Errorf("error updating cpuscalingconfiguration: %w", err)
		logger.Error(err, "")
		return err
	}
	logger.V(5).Info("cpuscalingconfiguration successfully updated")

	return nil
}

// createConfigItems creates slices of powerv1.ConfigItem mapped to node names. If no Containers are requesting
// passed CPUScalingProfile on iterated node, empty powerv1.ConfigItem slice is mapped to node name.
func (r *CPUScalingProfileReconciler) createConfigItems(powerWorkloadList *powerv1.PowerWorkloadList,
	scalingProfile *powerv1.CPUScalingProfile) map[string][]powerv1.ConfigItem {
	nodesItems := make(map[string][]powerv1.ConfigItem)
	for _, powerWorkload := range powerWorkloadList.Items {
		if powerWorkload.Spec.PowerProfile == scalingProfile.Name && !powerWorkload.Spec.AllCores {
			// ConfigItem is per Container
			for _, container := range powerWorkload.Spec.Node.Containers {
				nodesItems[powerWorkload.Spec.Node.Name] = append(nodesItems[powerWorkload.Spec.Node.Name],
					powerv1.ConfigItem{
						PowerProfile:        scalingProfile.Name,
						CpuIDs:              container.ExclusiveCPUs,
						SamplePeriod:        *scalingProfile.Spec.SamplePeriod,
						FallbackFreqPercent: eppDefaults[scalingProfile.Spec.Epp].configItem.FallbackFreqPercent,
						PodUID:              container.PodUID,
					},
				)
			}
			// No containers are using PowerProfile owned by this CPUScalingProfile
			if _, found := nodesItems[powerWorkload.Spec.Node.Name]; !found {
				nodesItems[powerWorkload.Spec.Node.Name] = []powerv1.ConfigItem{}
			}
		}
	}

	return nodesItems
}

// setCPUScalingConfiguration sets .Spec.Items to newItems and refreshes Ownership by CPUScalingProfile accordingly.
func (r *CPUScalingProfileReconciler) setCPUScalingConfiguration(scalingConfig *powerv1.CPUScalingConfiguration,
	newItems []powerv1.ConfigItem, scalingProfile metav1.Object) error {
	// Handle CPUScalingConfiguration.Spec.Items
	scalingConfig.Spec.Items = slices.DeleteFunc(scalingConfig.Spec.Items,
		func(item powerv1.ConfigItem) bool {
			return item.PowerProfile == scalingProfile.GetName()
		},
	)
	scalingConfig.Spec.Items = append(scalingConfig.Spec.Items, newItems...)
	// Sort is crucial to update decision, we don't want to update resource in apiserver just because order has changed
	slices.SortFunc(scalingConfig.Spec.Items, func(a, b powerv1.ConfigItem) int {
		return strings.Compare(string(a.PodUID), string(b.PodUID))
	})

	// Handle Ownership
	if len(newItems) == 0 {
		if err := controllerutil.RemoveOwnerReference(scalingProfile, scalingConfig, r.Scheme); err != nil {
			return fmt.Errorf("error deleting ownership of updated cpuscalingconfiguration: %w", err)
		}
	} else {
		if err := controllerutil.SetOwnerReference(scalingProfile, scalingConfig, r.Scheme); err != nil {
			return fmt.Errorf("error setting up ownership of updated cpuscalingconfiguration: %w", err)
		}
	}

	return nil
}

func (r *CPUScalingProfileReconciler) mapPowerWorkloadToCPUScalingProfile(ctx context.Context, o client.Object,
) []reconcile.Request {
	logger := r.Log.WithValues("method", "handler.MapFunc")

	powerWorkload := o.(*powerv1.PowerWorkload)
	cpuScalingProfileList := &powerv1.CPUScalingProfileList{}
	if err := r.Client.List(ctx, cpuScalingProfileList); err != nil {
		logger.Error(err, "error retrieving cpuscalingprofile list")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, cpuScalingProfile := range cpuScalingProfileList.Items {
		if powerWorkload.Spec.PowerProfile == cpuScalingProfile.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cpuScalingProfile.Name,
					Namespace: cpuScalingProfile.Namespace,
				},
			})
			break
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *CPUScalingProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.CPUScalingProfile{}).
		Owns(&powerv1.PowerProfile{}).
		Owns(&powerv1.CPUScalingConfiguration{}, builder.MatchEveryOwner).
		Watches(&powerv1.PowerWorkload{}, handler.EnqueueRequestsFromMapFunc(r.mapPowerWorkloadToCPUScalingProfile)).
		Complete(r)
}
