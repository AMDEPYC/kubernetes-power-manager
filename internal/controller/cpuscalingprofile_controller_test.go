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
	"testing"
	"time"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createCPUScalingProfileReconcilerObject(objs []client.Object, objsLists []client.ObjectList,
) (*CPUScalingProfileReconciler, error) {
	log.SetLogger(zap.New(zap.UseDevMode(true), func(opts *zap.Options) {
		opts.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))

	// Register operator types with the runtime scheme.
	s := runtime.NewScheme()
	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().
		WithObjects(objs...).
		WithStatusSubresource(objs...).
		WithLists(objsLists...).
		WithScheme(s).
		Build()

	// Create a reconciler object with the scheme and fake client.
	r := &CPUScalingProfileReconciler{
		Client: cl,
		Log:    ctrl.Log.WithName("testing"),
		Scheme: s,
	}

	return r, nil
}

func TestCPUScalingProfile_SetupWithManager(t *testing.T) {
	r, err := createProfileReconcilerObject([]runtime.Object{})
	if !assert.NoError(t, err) {
		return
	}

	mgr := new(testutils.MgrMock)
	mgr.On("GetControllerOptions").Return(config.Controller{})
	mgr.On("GetScheme").Return(r.Scheme)
	mgr.On("GetLogger").Return(r.Log)
	mgr.On("SetFields", mock.Anything).Return(nil)
	mgr.On("Add", mock.Anything).Return(nil)
	mgr.On("GetRESTMapper").Return(new(testutils.RestMapperMk))
	mgr.On("GetCache").Return(new(testutils.CacheMk))

	err = r.SetupWithManager(mgr)
	assert.NoError(t, err)
}

func TestCPUScalingProfile_WrongNamespace(t *testing.T) {
	r, err := createCPUScalingProfileReconcilerObject([]client.Object{}, []client.ObjectList{})
	if !assert.NoError(t, err) {
		return
	}
	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "shared",
			Namespace: "wrong-namespace",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "incorrect namespace")
}

func TestCPUScalingProfile_Reconcile_Validate(t *testing.T) {
	tCases := []struct {
		testCase       string
		validateStatus func(client.Client)
		clientObjs     []client.Object
	}{
		{
			testCase: "Test Case 1 - Min larger than Max",
			validateStatus: func(c client.Client) {
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Contains(t, csc.Status.Errors,
					"cpuscalingprofile spec is not correct: Min must be lower or equal to Max")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Max:          2900,
						Min:          3050,
						SamplePeriod: metav1.Duration{Duration: 500 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 2 - Sample period too large",
			validateStatus: func(c client.Client) {
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				if !assert.Len(t, csc.Status.Errors, 1) {
					return
				}
				assert.Contains(t, csc.Status.Errors[0],
					"cpuscalingprofile spec is not correct: SamplePeriod must be larger than")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Max:          3100,
						Min:          3000,
						SamplePeriod: metav1.Duration{Duration: 1100 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 3 - Sample period too small",
			validateStatus: func(c client.Client) {
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				if !assert.Len(t, csc.Status.Errors, 1) {
					return
				}
				assert.Contains(t, csc.Status.Errors[0],
					"cpuscalingprofile spec is not correct: SamplePeriod must be larger than")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Max:          3100,
						Min:          3000,
						SamplePeriod: metav1.Duration{Duration: 5 * time.Millisecond},
					},
				},
			},
		},
	}

	for _, tc := range tCases {
		t.Log(tc.testCase)
		r, err := createCPUScalingProfileReconcilerObject(tc.clientObjs, []client.ObjectList{})
		if !assert.NoError(t, err) {
			return
		}

		objKey := client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}
		_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: objKey})
		if !assert.NoError(t, err) {
			return
		}

		tc.validateStatus(r.Client)
		assert.True(t, errors.IsNotFound(r.Client.Get(context.TODO(),
			client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace},
			&powerv1.PowerProfile{})))
	}
}

func TestCPUScalingProfile_Reconcile(t *testing.T) {
	tCases := []struct {
		testCase                   string
		cpuScalingProfileName      string
		validateReconcileAndStatus func(error, client.Client)
		validateObjects            func(client.Client)
		clientObjs                 []client.Object
		objsLists                  []client.ObjectList
	}{
		{
			testCase:              "Test Case 1 - CPUScalingProfile is added",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				pp := &powerv1.PowerProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, pp)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:               "cpuscalingprofile1",
						UID:                "lkj",
						Kind:               "CPUScalingProfile",
						APIVersion:         "power.intel.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Governor: userspaceGovernor,
					Epp:      "balance_performance",
					Shared:   false,
				}, pp.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
			},
		},
		{
			testCase:              "Test Case 2 - CPUScalingProfile got modified",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				pp := &powerv1.PowerProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, pp)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:               "cpuscalingprofile1",
						UID:                "lkj",
						Kind:               "CPUScalingProfile",
						APIVersion:         "power.intel.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Governor: userspaceGovernor,
					Epp:      "balance_performance",
					Shared:   false,
				}, pp.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cpuscalingprofile1",
						Namespace:         IntelPowerNamespace,
						UID:               "hgf",
						CreationTimestamp: metav1.Now(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "cpuscalingprofile1",
								UID:                "lkj",
								Kind:               "CPUScalingProfile",
								APIVersion:         "power.intel.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "cpuscalingprofile1",
						Min:      ptr.To(intstr.FromInt32(1999)),
						Max:      ptr.To(intstr.FromInt32(2001)),
						Shared:   false,
						Epp:      "balance_performance",
						Governor: "powersave",
					},
				},
			},
		},
		{
			testCase:              "Test Case 3 - CPUScalingProfile was deleted",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				assert.NoError(t, err)
			},
			validateObjects: func(c client.Client) {
				// We leave deleting of PowerProfile to K8s GC, fakeclient does not
				// have GC capabilities, hence cannot assert PowerProfile deletion here

				// CPUScalingConfiguration should be deleted
				assert.True(t, errors.IsNotFound(c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace},
					&powerv1.CPUScalingConfiguration{})))
			},
			clientObjs: []client.Object{
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cpuscalingprofile1",
						Namespace:         IntelPowerNamespace,
						UID:               "hgf",
						CreationTimestamp: metav1.Now(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "cpuscalingprofile1",
								UID:                "lkj",
								Kind:               "CPUScalingProfile",
								APIVersion:         "power.intel.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.CPUScalingConfigurationList{
					Items: []powerv1.CPUScalingConfiguration{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "worker1",
								Namespace: IntelPowerNamespace,
								OwnerReferences: []metav1.OwnerReference{
									{
										Name:       "cpuscalingprofile1",
										UID:        "lkj",
										Kind:       "CPUScalingProfile",
										APIVersion: "power.intel.com/v1",
									},
								},
							},
							Spec: powerv1.CPUScalingConfigurationSpec{
								Items: []powerv1.ConfigItem{
									{
										PowerProfile: "cpuscalingprofile1",
										CpuIDs:       []uint{5, 6, 7, 8},
										SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
										PodUID:       "abcde",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			testCase: "Test Case 4 - CPUScalingProfile is added, but PowerProfile with the same name" +
				" already exists and doesn't have CPUScalingProfile ownership",
			cpuScalingProfileName: "performance",
			validateReconcileAndStatus: func(err error, c client.Client) {
				assert.Error(t, err)
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "performance", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.ElementsMatch(t, csc.Status.Errors,
					[]string{"error during powerprofile CreateOrUpdate operation: reconciled cpuscalingprofile " +
						"is not owner of this powerprofile, stopping reconciliation"})
			},
			validateObjects: func(c client.Client) {
				pp := &powerv1.PowerProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "performance", Namespace: IntelPowerNamespace}, pp)) {
					return
				}
				assert.Empty(t, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "performance",
					Min:      ptr.To(intstr.FromInt32(1000)),
					Max:      ptr.To(intstr.FromInt32(2000)),
					Shared:   false,
					Epp:      "performance",
					Governor: "performance",
				}, pp.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "performance",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "performance",
						Namespace:         IntelPowerNamespace,
						UID:               "hgf",
						CreationTimestamp: metav1.Now(),
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "performance",
						Min:      ptr.To(intstr.FromInt32(1000)),
						Max:      ptr.To(intstr.FromInt32(2000)),
						Shared:   false,
						Epp:      "performance",
						Governor: "performance",
					},
				},
			},
		},
		{
			testCase:              "Test Case 5 - PowerProfile exists and have more, non-controller Ownerships added",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				pp := &powerv1.PowerProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, pp)) {
					return
				}
				// New Ownership entry will be respected, but any changes to spec will be reverted
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:               "cpuscalingprofile1",
						UID:                "lkj",
						Kind:               "CPUScalingProfile",
						APIVersion:         "power.intel.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
					{
						Name:       "other-owner",
						UID:        "gfd",
						Kind:       "other",
						APIVersion: "other-comp.com/v1",
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Shared:   false,
					Epp:      "balance_performance",
					Governor: userspaceGovernor,
				}, pp.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cpuscalingprofile1",
						Namespace:         IntelPowerNamespace,
						UID:               "hgf",
						CreationTimestamp: metav1.Now(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "cpuscalingprofile1",
								UID:                "lkj",
								Kind:               "CPUScalingProfile",
								APIVersion:         "power.intel.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
							{
								Name:       "other-owner",
								UID:        "gfd",
								Kind:       "other",
								APIVersion: "other-comp.com/v1",
							},
						},
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "cpuscalingprofile1",
						Min:      ptr.To(intstr.FromInt32(2000)),
						Max:      ptr.To(intstr.FromInt32(1000)),
						Shared:   false,
						Epp:      "balance_performance",
						Governor: "powersave",
					},
				},
			},
		},
		{
			testCase:              "Test Case 6 - Owned PowerProfile got modified",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				pp := &powerv1.PowerProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, pp)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:               "cpuscalingprofile1",
						UID:                "lkj",
						Kind:               "CPUScalingProfile",
						APIVersion:         "power.intel.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Shared:   false,
					Epp:      "balance_performance",
					Governor: userspaceGovernor,
				}, pp.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cpuscalingprofile1",
						Namespace:         IntelPowerNamespace,
						UID:               "hgf",
						CreationTimestamp: metav1.Now(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "cpuscalingprofile1",
								UID:                "lkj",
								Kind:               "CPUScalingProfile",
								APIVersion:         "power.intel.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "cpuscalingprofile1",
						Min:      ptr.To(intstr.FromInt32(2000)),
						Max:      ptr.To(intstr.FromInt32(2500)),
						Shared:   false,
						Epp:      "balance_performance",
						Governor: "powersave",
					},
				},
			},
		},
		{
			testCase:              "Test Case 7 - Valid CPUScalingProfile is added, containers are requesting it",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				csc := &powerv1.CPUScalingConfiguration{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:       "cpuscalingprofile1",
						UID:        "lkj",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.intel.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile: "cpuscalingprofile1",
							CpuIDs:       []uint{5, 6, 7, 8},
							SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
							PodUID:       "abcde",
						},
					},
				}, csc.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.PowerWorkloadList{
					Items: []powerv1.PowerWorkload{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile1",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
									Containers: []powerv1.Container{
										{
											Name:          "container1",
											Namespace:     IntelPowerNamespace,
											Pod:           "pod1",
											PodUID:        "abcde",
											ExclusiveCPUs: []uint{5, 6, 7, 8},
											PowerProfile:  "cpuscalingprofile1",
										},
									},
									CpuIds: []uint{5, 6, 7, 8},
								},
							},
						},
					},
				},
			},
		},
		{
			testCase: "Test Case 8 - One CPUScalingProfile is already used by containers, " +
				"another is requested.",
			cpuScalingProfileName: "cpuscalingprofile2",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				csc := &powerv1.CPUScalingConfiguration{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:       "cpuscalingprofile1",
						UID:        "lkj",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.intel.com/v1",
					},
					{
						Name:       "cpuscalingprofile2",
						UID:        "hgf",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.intel.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile: "cpuscalingprofile1",
							CpuIDs:       []uint{3, 4},
							SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
							PodUID:       "abcde",
						},
						{
							PowerProfile: "cpuscalingprofile2",
							CpuIDs:       []uint{1, 2},
							SamplePeriod: metav1.Duration{Duration: 89 * time.Millisecond},
							PodUID:       "fghij",
						},
					},
				}, csc.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile2",
						Namespace: IntelPowerNamespace,
						UID:       "hgf",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          1000,
						Max:          2000,
						SamplePeriod: metav1.Duration{Duration: 89 * time.Millisecond},
						Epp:          "balance_power",
					},
				},
				&powerv1.CPUScalingConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker1",
						Namespace: IntelPowerNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "cpuscalingprofile1",
								UID:        "lkj",
								Kind:       "CPUScalingProfile",
								APIVersion: "power.intel.com/v1",
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile: "cpuscalingprofile1",
								CpuIDs:       []uint{3, 4},
								SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
								PodUID:       "abcde",
							},
						},
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.PowerWorkloadList{
					Items: []powerv1.PowerWorkload{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile1",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
									Containers: []powerv1.Container{
										{
											Name:          "container1",
											Namespace:     IntelPowerNamespace,
											Pod:           "pod1",
											PodUID:        "abcde",
											ExclusiveCPUs: []uint{3, 4},
											PowerProfile:  "cpuscalingprofile2",
										},
									},
									CpuIds: []uint{3, 4},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile2-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile2",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
									Containers: []powerv1.Container{
										{
											Name:          "container1",
											Namespace:     IntelPowerNamespace,
											Pod:           "pod2",
											PodUID:        "fghij",
											ExclusiveCPUs: []uint{1, 2},
											PowerProfile:  "cpuscalingprofile2",
										},
									},
									CpuIds: []uint{1, 2},
								},
							},
						},
					},
				},
			},
		},
		{
			testCase: "Test Case 9 - Containers are no longer requesting one CPUScalingProfile, " +
				"but other CPUScalingProfiles are still in use",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				csc := &powerv1.CPUScalingConfiguration{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:       "cpuscalingprofile2",
						UID:        "hgf",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.intel.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile: "cpuscalingprofile2",
							CpuIDs:       []uint{1, 2},
							SamplePeriod: metav1.Duration{Duration: 89 * time.Millisecond},
							PodUID:       "fghij",
						},
					},
				}, csc.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile2",
						Namespace: IntelPowerNamespace,
						UID:       "hgf",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          1000,
						Max:          2000,
						SamplePeriod: metav1.Duration{Duration: 89 * time.Millisecond},
						Epp:          "balance_power",
					},
				},
				&powerv1.CPUScalingConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker1",
						Namespace: IntelPowerNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "cpuscalingprofile1",
								UID:        "lkj",
								Kind:       "CPUScalingProfile",
								APIVersion: "power.intel.com/v1",
							},
							{
								Name:       "cpuscalingprofile2",
								UID:        "hgf",
								Kind:       "CPUScalingProfile",
								APIVersion: "power.intel.com/v1",
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile: "cpuscalingprofile1",
								CpuIDs:       []uint{5, 6, 7, 8},
								SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
								PodUID:       "abcde",
							},
							{
								PowerProfile: "cpuscalingprofile2",
								CpuIDs:       []uint{1, 2},
								SamplePeriod: metav1.Duration{Duration: 89 * time.Millisecond},
								PodUID:       "fghij",
							},
						},
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.PowerWorkloadList{
					Items: []powerv1.PowerWorkload{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile2-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile2",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
									Containers: []powerv1.Container{
										{
											Name:          "container1",
											Namespace:     IntelPowerNamespace,
											Pod:           "pod2",
											PodUID:        "fghij",
											ExclusiveCPUs: []uint{1, 2},
											PowerProfile:  "cpuscalingprofile2",
										},
									},
									CpuIds: []uint{1, 2},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile1",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
								},
							},
						},
					},
				},
			},
		},
		{
			testCase: "Test Case 10 - CPUScalingProfile is present, containers are no longer " +
				"requesting it",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				// Empty CPUScalingConfiguration should be deleted
				assert.True(t, errors.IsNotFound(c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace},
					&powerv1.CPUScalingConfiguration{}),
				))
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.CPUScalingConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker1",
						Namespace: IntelPowerNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "cpuscalingprofile1",
								UID:        "lkj",
								Kind:       "CPUScalingProfile",
								APIVersion: "power.intel.com/v1",
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile: "cpuscalingprofile1",
								CpuIDs:       []uint{5, 6, 7, 8},
								SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
								PodUID:       "abcde",
							},
						},
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.PowerWorkloadList{
					Items: []powerv1.PowerWorkload{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile1",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
								},
							},
						},
					},
				},
			},
		},
		{
			testCase:              "Test Case 11 - Owned CPUScalingConfiguration got modified",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				csc := &powerv1.CPUScalingConfiguration{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:       "cpuscalingprofile1",
						UID:        "lkj",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.intel.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile: "cpuscalingprofile1",
							CpuIDs:       []uint{5, 6, 7, 8},
							SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
							PodUID:       "abcde",
						},
					},
				}, csc.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          2000,
						Max:          3000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.CPUScalingConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker1",
						Namespace: IntelPowerNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "cpuscalingprofile1",
								UID:                "lkj",
								Kind:               "CPUScalingProfile",
								APIVersion:         "power.intel.com/v1",
								Controller:         ptr.To(false),
								BlockOwnerDeletion: ptr.To(false),
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile: "cpuscalingprofile1",
								CpuIDs:       []uint{99, 999},
								SamplePeriod: metav1.Duration{Duration: 99 * time.Millisecond},
								PodUID:       "mnbvc",
							},
						},
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.PowerWorkloadList{
					Items: []powerv1.PowerWorkload{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile1",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
									Containers: []powerv1.Container{
										{
											Name:          "container1",
											Namespace:     IntelPowerNamespace,
											Pod:           "pod1",
											PodUID:        "abcde",
											ExclusiveCPUs: []uint{5, 6, 7, 8},
											PowerProfile:  "cpuscalingprofile1",
										},
									},
									CpuIds: []uint{5, 6, 7, 8},
								},
							},
						},
					},
				},
			},
		},
		{
			testCase:              "Test Case 12 - Empty reconciliation",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {
				pp := &powerv1.PowerProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, pp)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:               "cpuscalingprofile1",
						UID:                "lkj",
						Kind:               "CPUScalingProfile",
						APIVersion:         "power.intel.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(1000)),
					Max:      ptr.To(intstr.FromInt32(2000)),
					Shared:   false,
					Epp:      "balance_performance",
					Governor: userspaceGovernor,
				}, pp.Spec)
				csc := &powerv1.CPUScalingConfiguration{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "worker1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.ElementsMatch(t, []metav1.OwnerReference{
					{
						Name:       "cpuscalingprofile1",
						UID:        "lkj",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.intel.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile: "cpuscalingprofile1",
							CpuIDs:       []uint{1, 2},
							SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
							PodUID:       "abcde",
						},
					},
				}, csc.Spec)
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          1000,
						Max:          2000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
				&powerv1.PowerProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cpuscalingprofile1",
						Namespace:         IntelPowerNamespace,
						UID:               "hgf",
						CreationTimestamp: metav1.Now(),
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "cpuscalingprofile1",
								UID:                "lkj",
								Kind:               "CPUScalingProfile",
								APIVersion:         "power.intel.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "cpuscalingprofile1",
						Min:      ptr.To(intstr.FromInt32(1000)),
						Max:      ptr.To(intstr.FromInt32(2000)),
						Shared:   false,
						Epp:      "balance_performance",
						Governor: userspaceGovernor,
					},
				},
				&powerv1.CPUScalingConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "worker1",
						Namespace: IntelPowerNamespace,
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile: "cpuscalingprofile1",
								CpuIDs:       []uint{1, 2},
								SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
								PodUID:       "abcde",
							},
						},
					},
				},
			},
			objsLists: []client.ObjectList{
				&powerv1.PowerWorkloadList{
					Items: []powerv1.PowerWorkload{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1-worker1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.PowerWorkloadSpec{
								PowerProfile: "cpuscalingprofile1",
								Node: powerv1.WorkloadNode{
									Name: "worker1",
									Containers: []powerv1.Container{
										{
											Name:          "container1",
											Namespace:     IntelPowerNamespace,
											Pod:           "pod1",
											PodUID:        "abcde",
											ExclusiveCPUs: []uint{1, 2},
											PowerProfile:  "cpuscalingprofile1",
										},
									},
									CpuIds: []uint{1, 2},
								},
							},
						},
					},
				},
			},
		},
		{
			testCase:              "Test Case 13 - Status cleared from previous reconciliation error",
			cpuScalingProfileName: "cpuscalingprofile1",
			validateReconcileAndStatus: func(err error, c client.Client) {
				if !assert.NoError(t, err) {
					return
				}
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Empty(t, csc.Status.Errors)
			},
			validateObjects: func(c client.Client) {},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Status: powerv1.CPUScalingProfileStatus{
						ID: 0,
						StatusErrors: powerv1.StatusErrors{
							Errors: []string{"error happened on last reconcile"},
						},
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          1000,
						Max:          2000,
						SamplePeriod: metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          "balance_performance",
					},
				},
			},
		},
	}

	for _, tc := range tCases {
		t.Log(tc.testCase)
		r, err := createCPUScalingProfileReconcilerObject(tc.clientObjs, tc.objsLists)
		if !assert.NoError(t, err) {
			return
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      tc.cpuScalingProfileName,
				Namespace: IntelPowerNamespace,
			},
		}
		_, err = r.Reconcile(context.TODO(), req)

		tc.validateReconcileAndStatus(err, r.Client)
		tc.validateObjects(r.Client)
	}
}

func TestCPUScalingProfile_mapPowerWorkloadToCPUScalingProfile(t *testing.T) {
	tCases := []struct {
		testCase       string
		powerWorkload  *powerv1.PowerWorkload
		validateResult func(reqs []reconcile.Request)
		objsLists      []client.ObjectList
	}{
		{
			testCase: "Test Case 1 - PowerWorkload matched to existing CPUScalingProfile",
			powerWorkload: &powerv1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cpuscalingprofile1-worker1",
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerWorkloadSpec{
					PowerProfile: "cpuscalingprofile1",
				},
			},
			validateResult: func(reqs []reconcile.Request) {
				assert.Equal(t, []reconcile.Request{
					{
						NamespacedName: types.NamespacedName{
							Name:      "cpuscalingprofile1",
							Namespace: IntelPowerNamespace,
						},
					},
				}, reqs)
			},
			objsLists: []client.ObjectList{
				&powerv1.CPUScalingProfileList{
					Items: []powerv1.CPUScalingProfile{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1",
								Namespace: IntelPowerNamespace,
							},
							Spec: powerv1.CPUScalingProfileSpec{},
						},
					},
				},
			},
		},
		{
			testCase: "Test Case 2 - PowerWorkload not matched to existing CPUScalingProfile",
			powerWorkload: &powerv1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cpuscalingprofile2-worker1",
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerWorkloadSpec{
					PowerProfile: "cpuscalingprofile2",
				},
			},
			validateResult: func(reqs []reconcile.Request) {
				assert.Empty(t, reqs)
			},
			objsLists: []client.ObjectList{
				&powerv1.CPUScalingProfileList{
					Items: []powerv1.CPUScalingProfile{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "cpuscalingprofile1",
								Namespace: IntelPowerNamespace,
							},
						},
					},
				},
			},
		},
		{
			testCase: "Test Case 3 - CPUScalingProfileList cannot be retrieved",
			powerWorkload: &powerv1.PowerWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cpuscalingprofile1-worker1",
					Namespace: IntelPowerNamespace,
				},
				Spec: powerv1.PowerWorkloadSpec{
					PowerProfile: "cpuscalingprofile1",
				},
			},
			validateResult: func(reqs []reconcile.Request) {
				assert.Empty(t, reqs)
			},
			objsLists: []client.ObjectList{},
		},
	}

	for _, tc := range tCases {
		t.Log(tc.testCase)
		r, err := createCPUScalingProfileReconcilerObject([]client.Object{}, tc.objsLists)
		assert.Nilf(t, err, "%s - error creating the reconciler object", tc.testCase)

		reqs := r.mapPowerWorkloadToCPUScalingProfile(context.TODO(), tc.powerWorkload)
		tc.validateResult(reqs)
	}
}
