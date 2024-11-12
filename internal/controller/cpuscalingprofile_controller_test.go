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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/testutils"
)

type PowerProfileParams struct {
	min int
	max int
	epp string
}


func createScalingProfileReconcilerObject(objs []runtime.Object) (*CPUScalingProfileReconciler, error) {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	),
	)
	// Register operator types with the runtime scheme.
	s := scheme.Scheme

	// Add route Openshift scheme
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithRuntimeObjects(objs...).WithScheme(s).Build()

	// Create a ReconcileNode object with the scheme and fake client.
	r := &CPUScalingProfileReconciler{cl, ctrl.Log.WithName("testing"), s}

	return r, nil
}

func TestCPUPerformanceScalingProfile_Wrong_Namespace(t *testing.T) {
	r, err := createScalingProfileReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "shared",
			Namespace: "wrong-namespace",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "incorrect namespace")
}

func TestCPUPerformanceScalingProfile_Reconcile_SetupPass(t *testing.T) {
	r, err := createProfileReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(testutils.MgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetCache").Return(new(testutils.CacheMk))
	err = (&CPUScalingProfileReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Nil(t, err)
}

func TestCPUPerformanceScalingProfile_Reconcile_SetupFail(t *testing.T) {
	r, err := createProfileReconcilerObject([]runtime.Object{})
	assert.Nil(t, err)
	mgr := new(testutils.MgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("Add", mock.Anything).Return(fmt.Errorf("setup fail"))

	err = (&PowerProfileReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(mgr)
	assert.Error(t, err)
}

func TestCPUPerformanceScalingProfile_ensurePowerProfile(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		params      PowerProfileParams
		validateErr func(e error) bool
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Create PowerProfile",
			nodeName:    "TestNode",
			profileName: "user-created",
			params: PowerProfileParams{
				min: 2500,
				max: 3500,
				epp: "power",
			},
			validateErr: func(e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{},
		},
		{
			testCase:    "Test Case 2 - Update PowerProfile",
			nodeName:    "TestNode",
			profileName: "user-created",
			params: PowerProfileParams{
				min: 2500,
				max: 3500,
				epp: "power",
			},
			validateErr: func(e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "user-created-TestNode",
						Shared:   true,
						Min:      2500,
						Max:      3500,
						Epp:      "power",
						Governor: "userspace",
					},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createScalingProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		err = r.ensurePowerProfile(
			&req,
			tc.params.min,
			tc.params.max,
			tc.params.epp,
		)
		tc.validateErr(err)

		powerProfile := &powerv1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.profileName + "-" + tc.nodeName,
			Namespace: IntelPowerNamespace,
		}, powerProfile)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile object after testing", tc.testCase)
		}

		assert.Equal(t, tc.params.min, powerProfile.Spec.Min)
		assert.Equal(t, tc.params.max, powerProfile.Spec.Max)
		assert.Equal(t, tc.params.epp, powerProfile.Spec.Epp)
		assert.Equal(t, true, powerProfile.Spec.Shared)
		assert.Equal(t, "userspace", powerProfile.Spec.Governor)
	}
}

func TestCPUPerformanceScalingProfile_createPowerProfile(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		params      PowerProfileParams
		validateErr func(e error) bool
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Create PowerProfile with parameters",
			nodeName:    "TestNode",
			profileName: "user-created-TestNode",
			params: PowerProfileParams{
				min: 2500,
				max: 3500,
				epp: "power",
			},
			validateErr: func(e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createScalingProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		err = r.createPowerProfile(
			tc.profileName,
			IntelPowerNamespace,
			tc.params.min,
			tc.params.max,
			tc.params.epp,
		)
		tc.validateErr(err)

		powerProfile := &powerv1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.profileName,
			Namespace: IntelPowerNamespace,
		}, powerProfile)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile object - should be present", tc.testCase)
		}

		assert.Equal(t, tc.params.min, powerProfile.Spec.Min)
		assert.Equal(t, tc.params.max, powerProfile.Spec.Max)
		assert.Equal(t, tc.params.epp, powerProfile.Spec.Epp)
		assert.Equal(t, true, powerProfile.Spec.Shared)
		assert.Equal(t, "userspace", powerProfile.Spec.Governor)
	}
}

func TestCPUPerformanceScalingProfile_updatePowerProfile(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		params      PowerProfileParams
		validateErr func(changed bool, e error) bool
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - Same parameters",
			nodeName:    "TestNode",
			profileName: "user-created-TestNode",
			params: PowerProfileParams{
				min: 2500,
				max: 3500,
				epp: "power",
			},
			validateErr: func(changed bool, e error) bool {
				return assert.False(t, changed) && assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "user-created-TestNode",
						Shared:   true,
						Min:      2500,
						Max:      3500,
						Epp:      "power",
						Governor: "userspace",
					},
				},
			},
		},
		{
			testCase:    "Test Case 2 - Different min",
			nodeName:    "TestNode",
			profileName: "user-created-TestNode",
			params: PowerProfileParams{
				min: 3000,
				max: 3500,
				epp: "power",
			},
			validateErr: func(changed bool, e error) bool {
				return assert.True(t, changed) && assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "user-created-TestNode",
						Shared:   true,
						Min:      2500,
						Max:      3500,
						Epp:      "power",
						Governor: "userspace",
					},
				},
			},
		},
		{
			testCase:    "Test Case 3 - Different max",
			nodeName:    "TestNode",
			profileName: "user-created-TestNode",
			params: PowerProfileParams{
				min: 2500,
				max: 3000,
				epp: "power",
			},
			validateErr: func(changed bool, e error) bool {
				return assert.True(t, changed) && assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "user-created-TestNode",
						Shared:   true,
						Min:      2500,
						Max:      3500,
						Epp:      "power",
						Governor: "userspace",
					},
				},
			},
		},
		{
			testCase:    "Test Case 4 - Different EPP",
			nodeName:    "TestNode",
			profileName: "user-created-TestNode",
			params: PowerProfileParams{
				min: 2500,
				max: 3500,
				epp: "performance",
			},
			validateErr: func(changed bool, e error) bool {
				return assert.True(t, changed) && assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "user-created-TestNode",
						Shared:   true,
						Min:      2500,
						Max:      3500,
						Epp:      "power",
						Governor: "userspace",
					},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createScalingProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		powerProfile := &powerv1.PowerProfile{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.profileName,
			Namespace: IntelPowerNamespace,
		}, powerProfile)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile object before testing", tc.testCase)
		}

		changed, err := r.updatePowerProfile(
			powerProfile,
			tc.params.min,
			tc.params.max,
			tc.params.epp,
		)
		tc.validateErr(changed, err)

		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      tc.profileName,
			Namespace: IntelPowerNamespace,
		}, powerProfile)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile object after testing", tc.testCase)
		}

		assert.Equal(t, tc.params.min, powerProfile.Spec.Min)
		assert.Equal(t, tc.params.max, powerProfile.Spec.Max)
		assert.Equal(t, tc.params.epp, powerProfile.Spec.Epp)
		assert.Equal(t, true, powerProfile.Spec.Shared)
		assert.Equal(t, "userspace", powerProfile.Spec.Governor)
	}
}

func TestCPUPerformanceScalingProfile_deletePowerProfile(t *testing.T) {
	tcases := []struct {
		testCase    string
		nodeName    string
		profileName string
		validateErr func(e error) bool
		clientObjs  []runtime.Object
	}{
		{
			testCase:    "Test Case 1 - No PowerProfile to delete",
			nodeName:    "TestNode",
			profileName: "user-created",
			validateErr: func(e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{},
		},
		{
			testCase:    "Test Case 2 - Successful PowerProfile delete",
			nodeName:    "TestNode",
			profileName: "user-created",
			validateErr: func(e error) bool {
				return assert.Nil(t, e)
			},
			clientObjs: []runtime.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-created-TestNode",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.PowerProfileSpec{},
				},
			},
		},
	}
	for _, tc := range tcases {
		t.Setenv("NODE_NAME", tc.nodeName)

		r, err := createScalingProfileReconcilerObject(tc.clientObjs)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error creating the reconciler object", tc.testCase)
		}

		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      tc.profileName,
				Namespace: IntelPowerNamespace,
			},
		}

		err = r.deletePowerProfile(&req)
		tc.validateErr(err)

		powerProfiles := &powerv1.PowerProfileList{}
		err = r.Client.List(context.TODO(), powerProfiles)
		if err != nil {
			t.Error(err)
			t.Fatalf("%s - error retrieving the power profile objects", tc.testCase)
		}
		if len(powerProfiles.Items) > 0 {
			t.Errorf("%s - failed: expected the number of power profile objects to be zero", tc.testCase)
		}
	}
}
