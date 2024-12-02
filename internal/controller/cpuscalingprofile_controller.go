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

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
		return ctrl.Result{}, nil
	}

	// Ideally, verification should be in admission webhook
	err = r.verifyCPUScalingProfileParams(&scalingProfile.Spec)
	if err != nil {
		err = errors.NewServiceUnavailable(err.Error())
		logger.Error(err, "")
		return ctrl.Result{}, nil
	}

	err = r.createOrUpdatePowerProfile(scalingProfile, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: CPUScalingConfiguration related code

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
	if scalingSpec.Min > scalingSpec.Max {
		return fmt.Errorf("%s: Min must be lower or equal to Max", errMsg)
	}

	if scalingSpec.SamplePeriod.Duration < minSamplePeriod || scalingSpec.SamplePeriod.Duration > maxSamplePeriod {
		return fmt.Errorf("%s: SamplePeriod must be larger than %d and lower than %d",
			errMsg, minSamplePeriod, maxSamplePeriod)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CPUScalingProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.CPUScalingProfile{}).
		Owns(&powerv1.PowerProfile{}).
		Complete(r)
}
