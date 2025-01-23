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

	powerv1 "github.com/AMDEPYC/kubernetes-power-manager/api/v1"
	"github.com/AMDEPYC/kubernetes-power-manager/pkg/testutils"

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
			testCase: "Test Case 1 - Min is provied and Max is omitted",
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
					"cpuscalingprofile spec is not correct: Max and Min "+
						"frequency values must be both provided or omitted")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:          ptr.To(intstr.FromInt32(2000)),
						SamplePeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 2 - Max is provied and Min is omitted",
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
					"cpuscalingprofile spec is not correct: Max and Min "+
						"frequency values must be both provided or omitted")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Max:          ptr.To(intstr.FromInt32(3000)),
						SamplePeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 3 - Min must be of the same type as Max",
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
					"cpuscalingprofile spec is not correct: Max and Min frequency "+
						"values must be both numeric or percentage")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:            ptr.To(intstr.FromInt32(2000)),
						Max:            ptr.To(intstr.FromString("100%")),
						SamplePeriod:   &metav1.Duration{Duration: 100 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 300 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 4 - Max is string in invalid format",
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
					"cpuscalingprofile spec is not correct: Max is not correct: ")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:            ptr.To(intstr.FromString("0%")),
						Max:            ptr.To(intstr.FromString("90-percent")),
						SamplePeriod:   &metav1.Duration{Duration: 100 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 300 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 5 - Min is string in invalid format",
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
					"cpuscalingprofile spec is not correct: Min is not correct: ")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:            ptr.To(intstr.FromString("8-percent")),
						Max:            ptr.To(intstr.FromString("100%")),
						SamplePeriod:   &metav1.Duration{Duration: 100 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 300 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 6 - Min larger than Max in string format",
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
						Min:            ptr.To(intstr.FromString("60%")),
						Max:            ptr.To(intstr.FromString("50%")),
						SamplePeriod:   &metav1.Duration{Duration: 100 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 300 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 7 - Min larger than Max in int format",
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
						Min:            ptr.To(intstr.FromInt32(3000)),
						Max:            ptr.To(intstr.FromInt32(2700)),
						SamplePeriod:   &metav1.Duration{Duration: 100 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 300 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 8 - Sample period too large",
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
						Min:            ptr.To(intstr.FromInt32(3000)),
						Max:            ptr.To(intstr.FromInt32(3100)),
						SamplePeriod:   &metav1.Duration{Duration: 1100 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 4000 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 9 - Sample period too small",
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
						Min:            ptr.To(intstr.FromInt32(3000)),
						Max:            ptr.To(intstr.FromInt32(3100)),
						SamplePeriod:   &metav1.Duration{Duration: 5 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 20 * time.Millisecond},
					},
				},
			},
		},
		{
			testCase: "Test Case 8 - Cooldown period smaller than SamplePeriod",
			validateStatus: func(c client.Client) {
				csc := &powerv1.CPUScalingProfile{}
				if !assert.NoError(t, c.Get(context.TODO(),
					client.ObjectKey{Name: "cpuscalingprofile1", Namespace: IntelPowerNamespace}, csc)) {
					return
				}
				assert.Contains(t, csc.Status.Errors,
					"cpuscalingprofile spec is not correct: CooldownPeriod must be larger than SamplePeriod")
			},
			clientObjs: []client.Object{
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile1",
						Namespace: IntelPowerNamespace,
						UID:       "lkj",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:            ptr.To(intstr.FromInt32(3000)),
						Max:            ptr.To(intstr.FromInt32(3100)),
						SamplePeriod:   &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod: &metav1.Duration{Duration: 14 * time.Millisecond},
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
						APIVersion:         "power.amdepyc.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Governor: userspaceGovernor,
					Epp:      powerv1.EPPBalancePerformance,
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
						Min:          ptr.To(intstr.FromInt32(2000)),
						Max:          ptr.To(intstr.FromInt32(3000)),
						SamplePeriod: &metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          powerv1.EPPBalancePerformance,
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
						APIVersion:         "power.amdepyc.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Governor: userspaceGovernor,
					Epp:      powerv1.EPPBalancePerformance,
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
						Min:          ptr.To(intstr.FromInt32(2000)),
						Max:          ptr.To(intstr.FromInt32(3000)),
						SamplePeriod: &metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          powerv1.EPPBalancePerformance,
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
								APIVersion:         "power.amdepyc.com/v1",
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
						Epp:      powerv1.EPPBalancePerformance,
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
								APIVersion:         "power.amdepyc.com/v1",
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
										APIVersion: "power.amdepyc.com/v1",
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
					Min:      ptr.To(intstr.FromString("20%")),
					Max:      ptr.To(intstr.FromString("70%")),
					Shared:   false,
					Epp:      powerv1.EPPPerformance,
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
						Min:          ptr.To(intstr.FromString("30%")),
						Max:          ptr.To(intstr.FromString("80%")),
						SamplePeriod: &metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          powerv1.EPPBalancePerformance,
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
						Min:      ptr.To(intstr.FromString("20%")),
						Max:      ptr.To(intstr.FromString("70%")),
						Shared:   false,
						Epp:      powerv1.EPPPerformance,
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
						APIVersion:         "power.amdepyc.com/v1",
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
					Min:      ptr.To(intstr.FromString("30%")),
					Max:      ptr.To(intstr.FromString("80%")),
					Shared:   false,
					Epp:      powerv1.EPPBalancePerformance,
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
						Min:          ptr.To(intstr.FromString("30%")),
						Max:          ptr.To(intstr.FromString("80%")),
						SamplePeriod: &metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          powerv1.EPPBalancePerformance,
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
								APIVersion:         "power.amdepyc.com/v1",
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
						Min:      ptr.To(intstr.FromString("20%")),
						Max:      ptr.To(intstr.FromString("70%")),
						Shared:   false,
						Epp:      powerv1.EPPBalancePerformance,
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
						APIVersion:         "power.amdepyc.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromInt32(2000)),
					Max:      ptr.To(intstr.FromInt32(3000)),
					Shared:   false,
					Epp:      powerv1.EPPBalancePerformance,
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
						Min:          ptr.To(intstr.FromInt32(2000)),
						Max:          ptr.To(intstr.FromInt32(3000)),
						SamplePeriod: &metav1.Duration{Duration: 15 * time.Millisecond},
						Epp:          powerv1.EPPBalancePerformance,
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
								APIVersion:         "power.amdepyc.com/v1",
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
						Epp:      powerv1.EPPBalancePerformance,
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
						APIVersion: "power.amdepyc.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile:               "cpuscalingprofile1",
							CpuIDs:                     []uint{5, 6, 7, 8},
							SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
							CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
							TargetBusyness:             70,
							AllowedBusynessDifference:  10,
							AllowedFrequencyDifference: 100,
							PodUID:                     "abcde",
							ScalePercentage:            150,
							FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
								configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromInt32(2000)),
						Max:                        ptr.To(intstr.FromInt32(3000)),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(150),
						Epp:                        powerv1.EPPBalancePerformance,
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
						APIVersion: "power.amdepyc.com/v1",
					},
					{
						Name:       "cpuscalingprofile2",
						UID:        "hgf",
						Kind:       "CPUScalingProfile",
						APIVersion: "power.amdepyc.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile:               "cpuscalingprofile1",
							CpuIDs:                     []uint{3, 4},
							SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
							CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
							TargetBusyness:             70,
							AllowedBusynessDifference:  10,
							AllowedFrequencyDifference: 100,
							ScalePercentage:            190,
							PodUID:                     "abcde",
							FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
								configItem.FallbackFreqPercent,
						},
						{
							PowerProfile:               "cpuscalingprofile2",
							CpuIDs:                     []uint{1, 2},
							SamplePeriod:               metav1.Duration{Duration: 89 * time.Millisecond},
							CooldownPeriod:             metav1.Duration{Duration: 170 * time.Millisecond},
							TargetBusyness:             60,
							AllowedBusynessDifference:  0,
							AllowedFrequencyDifference: 40,
							ScalePercentage:            20,
							PodUID:                     "fghij",
							FallbackFreqPercent:        eppDefaults[powerv1.EPPBalancePower].configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromInt32(2000)),
						Max:                        ptr.To(intstr.FromInt32(3000)),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(190),
						Epp:                        powerv1.EPPBalancePerformance,
					},
				},
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile2",
						Namespace: IntelPowerNamespace,
						UID:       "hgf",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:                        ptr.To(intstr.FromInt32(1000)),
						Max:                        ptr.To(intstr.FromInt32(2000)),
						SamplePeriod:               &metav1.Duration{Duration: 89 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 170 * time.Millisecond},
						TargetBusyness:             ptr.To(60),
						AllowedBusynessDifference:  ptr.To(0),
						AllowedFrequencyDifference: ptr.To(40),
						ScalePercentage:            ptr.To(20),
						Epp:                        powerv1.EPPBalancePower,
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
								APIVersion: "power.amdepyc.com/v1",
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile:               "cpuscalingprofile1",
								CpuIDs:                     []uint{3, 4},
								SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
								CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
								TargetBusyness:             70,
								AllowedBusynessDifference:  10,
								AllowedFrequencyDifference: 100,
								ScalePercentage:            190,
								PodUID:                     "abcde",
								FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
									configItem.FallbackFreqPercent,
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
						APIVersion: "power.amdepyc.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile:               "cpuscalingprofile2",
							CpuIDs:                     []uint{1, 2},
							SamplePeriod:               metav1.Duration{Duration: 89 * time.Millisecond},
							CooldownPeriod:             metav1.Duration{Duration: 170 * time.Millisecond},
							TargetBusyness:             60,
							AllowedBusynessDifference:  0,
							AllowedFrequencyDifference: 40,
							ScalePercentage:            100,
							PodUID:                     "fghij",
							FallbackFreqPercent:        eppDefaults[powerv1.EPPBalancePower].configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromString("30%")),
						Max:                        ptr.To(intstr.FromString("80%")),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(70),
						Epp:                        powerv1.EPPBalancePerformance,
					},
				},
				&powerv1.CPUScalingProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cpuscalingprofile2",
						Namespace: IntelPowerNamespace,
						UID:       "hgf",
					},
					Spec: powerv1.CPUScalingProfileSpec{
						Min:                        ptr.To(intstr.FromString("20%")),
						Max:                        ptr.To(intstr.FromString("70%")),
						SamplePeriod:               &metav1.Duration{Duration: 89 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 170 * time.Millisecond},
						TargetBusyness:             ptr.To(60),
						AllowedBusynessDifference:  ptr.To(0),
						AllowedFrequencyDifference: ptr.To(40),
						ScalePercentage:            ptr.To(100),
						Epp:                        powerv1.EPPBalancePower,
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
								APIVersion: "power.amdepyc.com/v1",
							},
							{
								Name:       "cpuscalingprofile2",
								UID:        "hgf",
								Kind:       "CPUScalingProfile",
								APIVersion: "power.amdepyc.com/v1",
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile:               "cpuscalingprofile1",
								CpuIDs:                     []uint{5, 6, 7, 8},
								SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
								CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
								TargetBusyness:             70,
								AllowedBusynessDifference:  10,
								AllowedFrequencyDifference: 100,
								ScalePercentage:            70,
								PodUID:                     "abcde",
								FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
									configItem.FallbackFreqPercent,
							},
							{
								PowerProfile:               "cpuscalingprofile2",
								CpuIDs:                     []uint{1, 2},
								SamplePeriod:               metav1.Duration{Duration: 89 * time.Millisecond},
								CooldownPeriod:             metav1.Duration{Duration: 170 * time.Millisecond},
								TargetBusyness:             60,
								AllowedBusynessDifference:  0,
								AllowedFrequencyDifference: 40,
								ScalePercentage:            100,
								PodUID:                     "fghij",
								FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePower].
									configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromInt32(2000)),
						Max:                        ptr.To(intstr.FromInt32(3000)),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(100),
						Epp:                        powerv1.EPPBalancePerformance,
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
								APIVersion: "power.amdepyc.com/v1",
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile:               "cpuscalingprofile1",
								CpuIDs:                     []uint{5, 6, 7, 8},
								SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
								CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
								TargetBusyness:             70,
								AllowedBusynessDifference:  10,
								AllowedFrequencyDifference: 100,
								ScalePercentage:            100,
								PodUID:                     "abcde",
								FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
									configItem.FallbackFreqPercent,
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
						APIVersion: "power.amdepyc.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile:               "cpuscalingprofile1",
							CpuIDs:                     []uint{5, 6, 7, 8},
							SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
							CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
							TargetBusyness:             70,
							AllowedBusynessDifference:  10,
							AllowedFrequencyDifference: 100,
							ScalePercentage:            100,
							PodUID:                     "abcde",
							FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
								configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromString("30%")),
						Max:                        ptr.To(intstr.FromString("80%")),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(100),
						Epp:                        powerv1.EPPBalancePerformance,
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
								APIVersion:         "power.amdepyc.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(false),
							},
						},
					},
					Spec: powerv1.CPUScalingConfigurationSpec{
						Items: []powerv1.ConfigItem{
							{
								PowerProfile:               "cpuscalingprofile1",
								CpuIDs:                     []uint{99, 999},
								SamplePeriod:               metav1.Duration{Duration: 99 * time.Millisecond},
								CooldownPeriod:             metav1.Duration{Duration: 11 * time.Millisecond},
								TargetBusyness:             1,
								AllowedBusynessDifference:  2,
								AllowedFrequencyDifference: 3,
								ScalePercentage:            14,
								PodUID:                     "mnbvc",
								FallbackFreqPercent:        eppDefaults[powerv1.EPPPerformance].configItem.FallbackFreqPercent,
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
						APIVersion:         "power.amdepyc.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      ptr.To(intstr.FromString("20%")),
					Max:      ptr.To(intstr.FromString("70%")),
					Shared:   false,
					Epp:      powerv1.EPPBalancePerformance,
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
						APIVersion: "power.amdepyc.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile:               "cpuscalingprofile1",
							CpuIDs:                     []uint{1, 2},
							SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
							CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
							TargetBusyness:             70,
							AllowedBusynessDifference:  10,
							AllowedFrequencyDifference: 100,
							ScalePercentage:            90,
							PodUID:                     "abcde",
							FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
								configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromString("20%")),
						Max:                        ptr.To(intstr.FromString("70%")),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(90),
						Epp:                        powerv1.EPPBalancePerformance,
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
								APIVersion:         "power.amdepyc.com/v1",
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: powerv1.PowerProfileSpec{
						Name:     "cpuscalingprofile1",
						Min:      ptr.To(intstr.FromString("20%")),
						Max:      ptr.To(intstr.FromString("70%")),
						Shared:   false,
						Epp:      powerv1.EPPBalancePerformance,
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
								PowerProfile:               "cpuscalingprofile1",
								CpuIDs:                     []uint{1, 2},
								SamplePeriod:               metav1.Duration{Duration: 15 * time.Millisecond},
								CooldownPeriod:             metav1.Duration{Duration: 25 * time.Millisecond},
								TargetBusyness:             70,
								AllowedBusynessDifference:  10,
								AllowedFrequencyDifference: 100,
								ScalePercentage:            90,
								PodUID:                     "abcde",
								FallbackFreqPercent: eppDefaults[powerv1.EPPBalancePerformance].
									configItem.FallbackFreqPercent,
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
						Min:                        ptr.To(intstr.FromInt32(1000)),
						Max:                        ptr.To(intstr.FromInt32(2000)),
						SamplePeriod:               &metav1.Duration{Duration: 15 * time.Millisecond},
						CooldownPeriod:             &metav1.Duration{Duration: 25 * time.Millisecond},
						TargetBusyness:             ptr.To(70),
						AllowedBusynessDifference:  ptr.To(10),
						AllowedFrequencyDifference: ptr.To(100),
						ScalePercentage:            ptr.To(90),
						Epp:                        powerv1.EPPBalancePerformance,
					},
				},
			},
		},
		{
			testCase:              "Test Case 14 - Empty CPUScalingProfile is added",
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
						APIVersion:         "power.amdepyc.com/v1",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					},
				}, pp.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.PowerProfileSpec{
					Name:     "cpuscalingprofile1",
					Min:      eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.Min,
					Max:      eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.Max,
					Governor: userspaceGovernor,
					Epp:      powerv1.EPPPower,
					Shared:   false,
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
						APIVersion: "power.amdepyc.com/v1",
					},
				}, csc.ObjectMeta.OwnerReferences)
				assert.Equal(t, powerv1.CPUScalingConfigurationSpec{
					Items: []powerv1.ConfigItem{
						{
							PowerProfile: "cpuscalingprofile1",
							CpuIDs:       []uint{5, 6, 7, 8},
							SamplePeriod: *eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.
								SamplePeriod,
							CooldownPeriod: *eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.
								CooldownPeriod,
							TargetBusyness: *eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.
								TargetBusyness,
							AllowedBusynessDifference: *eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.
								AllowedBusynessDifference,
							AllowedFrequencyDifference: *eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.
								AllowedFrequencyDifference,
							FallbackFreqPercent: eppDefaults[powerv1.EPPPower].configItem.FallbackFreqPercent,
							ScalePercentage:     *eppDefaults[powerv1.EPPPower].cpuScalingProfileSpec.ScalePercentage,
							PodUID:              "abcde",
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
					Spec: powerv1.CPUScalingProfileSpec{},
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
