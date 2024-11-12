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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CPUScalingProfile object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *CPUScalingProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	logger := r.Log.WithValues("scalingprofile", req.NamespacedName)
	if req.Namespace != IntelPowerNamespace {
		err := fmt.Errorf("incorrect namespace")
		logger.Error(err, "resource is not in the intel-power namespace, ignoring")
		return ctrl.Result{Requeue: false}, err
	}
	logger.Info("reconciling the scaling profile")

	scalingProfile := &powerv1.CPUScalingProfile{}
	defer func() { _ = writeUpdatedStatusErrsIfRequired(ctx, r.Status(), scalingProfile, err) }()
	err = r.Client.Get(context.TODO(), req.NamespacedName, scalingProfile)
	logger.V(5).Info("retrieving the scaling profile instances")

	if err != nil {
		if errors.IsNotFound(err) {
			// perform clean up on our side and PowerProfile side
			err = r.deletePowerProfile(&req)
			if err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}

		// Requeue the request
		return ctrl.Result{}, err
	}

	// verify received params
	logger.V(5).Info("making sure max value is higher than the min value")
	if scalingProfile.Spec.Max < scalingProfile.Spec.Min {
		maxLowerThanMinError := errors.NewServiceUnavailable("max scaling treshold value cannot be lower than the min value")
		logger.Error(maxLowerThanMinError, fmt.Sprintf("error creating the profile '%s'", scalingProfile.Spec.Name))
		return ctrl.Result{Requeue: false}, maxLowerThanMinError
	}

	// pass relevant values to associated PowerProfile
	err = r.ensurePowerProfile(
		&req,
		scalingProfile.Spec.Min,
		scalingProfile.Spec.Min,
		scalingProfile.Spec.Epp,
	)
	if err != nil {
		logger.Error(err, "")
		return ctrl.Result{}, err
	}

	// pass params to CPUPerformanceScalingWorker

	return ctrl.Result{}, nil
}

// create or update associated PowerProfile
func (r *CPUScalingProfileReconciler) ensurePowerProfile(req *ctrl.Request, min int, max int, epp string) error {
	var err error
	logger := r.Log.WithValues("scalingprofile", req.NamespacedName)
	nodeName := os.Getenv("NODE_NAME")

	powerProfileName := fmt.Sprintf("%s-%s", req.NamespacedName.Name, nodeName)
	powerProfile := &powerv1.PowerProfile{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      powerProfileName,
		Namespace: req.NamespacedName.Namespace,
	}, powerProfile)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf("error retrieving the power profile '%s'", powerProfileName))
			return err
		}

		err = r.createPowerProfile(
			powerProfileName,
			req.NamespacedName.Namespace,
			min,
			max,
			epp,
		)
		if err != nil {
			logger.Error(err, fmt.Sprintf("error creating the power profile '%s'", powerProfileName))
			return err
		}
		logger.V(5).Info("power profile successfully created", "name", powerProfileName)
	} else {
		wasUpdated, err := r.updatePowerProfile(powerProfile, min, max, epp)
		if !wasUpdated {
			logger.V(5).Info("power profile is already in desired stated", "name", powerProfileName)
		} else {
			if err != nil {
				logger.Error(err, fmt.Sprintf("error updating the power profile '%s'", powerProfileName))
				return err
			} else {
				logger.V(5).Info("power profile successfully updated", "name", powerProfileName)
			}
		}
	}

	return nil
}

// create the PowerProfile
func (r *CPUScalingProfileReconciler) createPowerProfile(name string, namespace string, min int, max int, epp string) error {
	powerProfile := &powerv1.PowerProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	powerProfileSpec := &powerv1.PowerProfileSpec{
		Name:     name,
		Shared:   true,
		Max:      max,
		Min:      min,
		Epp:      epp,
		Governor: "userspace",
	}
	powerProfile.Spec = *powerProfileSpec

	return r.Client.Create(context.TODO(), powerProfile)
}

// check if update is needed and update the PowerProfile
func (r *CPUScalingProfileReconciler) updatePowerProfile(powerProfile *powerv1.PowerProfile, min int, max int, epp string) (bool, error) {
	if powerProfile.Spec.Min == min &&
		powerProfile.Spec.Max == max &&
		powerProfile.Spec.Epp == epp {
		return false, nil
	}

	powerProfileSpec := &powerv1.PowerProfileSpec{
		Name:     powerProfile.Spec.Name,
		Shared:   true,
		Max:      max,
		Min:      min,
		Epp:      epp,
		Governor: "userspace",
	}
	powerProfile.Spec = *powerProfileSpec

	return true, r.Client.Update(context.TODO(), powerProfile)
}

// send delete request to associated PowerProfile
func (r *CPUScalingProfileReconciler) deletePowerProfile(req *ctrl.Request) error {
	var err error
	logger := r.Log.WithValues("scalingprofile", req.NamespacedName)
	nodeName := os.Getenv("NODE_NAME")

	powerProfileName := fmt.Sprintf("%s-%s", req.NamespacedName.Name, nodeName)
	powerProfile := &powerv1.PowerProfile{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      powerProfileName,
		Namespace: req.NamespacedName.Namespace,
	}, powerProfile)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf("error deleting the power profile '%s' from the cluster", powerProfileName))
			return err
		}
	} else {
		err = r.Client.Delete(context.TODO(), powerProfile)
		if err != nil {
			logger.Error(err, fmt.Sprintf("error deleting the power profile '%s' from the cluster", powerProfileName))
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CPUScalingProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&powerv1.CPUScalingProfile{}).
		Complete(r)
}
