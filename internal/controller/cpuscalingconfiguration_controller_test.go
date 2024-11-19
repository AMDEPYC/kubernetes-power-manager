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

	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/testutils"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
)

const (
	nodeName  string = "TestNode"
	namespace string = "power-manager"
)

func createCPUScalingConfigurationReconcilerObject(objs []client.Object) (*CPUScalingConfigurationReconciler, error) {
	log.SetLogger(zap.New(
		zap.UseDevMode(true),
		func(opts *zap.Options) {
			opts.TimeEncoder = zapcore.ISO8601TimeEncoder
		},
	))
	// Register operator types with the runtime scheme.
	s := runtime.NewScheme()

	if err := powerv1.AddToScheme(s); err != nil {
		return nil, err
	}
	cl := fake.NewClientBuilder().WithObjects(objs...).WithStatusSubresource(objs...).WithScheme(s).Build()
	r := &CPUScalingConfigurationReconciler{
		cl,
		ctrl.Log.WithName("testing"),
		scheme.Scheme,
		nil,
	}

	return r, nil
}

func TestCPUScalingConfiguration_Reconcile_InvalidRequests(t *testing.T) {
	t.Setenv("NODE_NAME", nodeName)
	r, err := createCPUScalingConfigurationReconcilerObject([]client.Object{})
	assert.Nil(t, err)

	// NOTE: wrong nodename in request is not tested because we do not
	// have enough information returned to reliably judge if it
	// actually works as expected

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodeName,
			Namespace: "wrong-ns",
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.ErrorContains(t, err, "incorrect namespace")
}

func TestCPUScalingConfiguration_Reconcile_Validation(t *testing.T) {
	cpuListMock := power.CpuList{}
	for i := 0; i < 10; i++ {
		mockCore := new(testutils.MockCPU)
		mockCore.On("GetID").Return(uint(i))
		cpuListMock = append(cpuListMock, mockCore)
	}
	nodemk := new(testutils.MockHost)
	nodemk.On("GetAllCpus").Return(&cpuListMock)

	tcases := []struct {
		testCase    string
		spec        powerv1.CPUScalingConfigurationSpec
		expectedErr string
	}{
		{
			testCase: "Test Case 1 - CPU ID not available on node",
			spec: powerv1.CPUScalingConfigurationSpec{
				Items: []powerv1.ConfigItem{
					{
						CpuIDs:       []uint{0, 1, 3, 4},
						SamplePeriod: metav1.Duration{},
					},
					{
						CpuIDs:       []uint{15},
						SamplePeriod: metav1.Duration{},
					},
				},
			},
			expectedErr: "cpu with id 15 is not available on node TestNode",
		},
		{
			testCase: "Test Case 2 - Duplicate CPU IDs in single config item",
			spec: powerv1.CPUScalingConfigurationSpec{
				Items: []powerv1.ConfigItem{
					{
						CpuIDs:       []uint{0, 4, 1, 3, 4},
						SamplePeriod: metav1.Duration{},
					},
				},
			},
			expectedErr: "cpu with id 4 is specified more than once on node TestNode",
		},
		{
			testCase: "Test Case 3 - Duplicate CPU IDs in different config items",
			spec: powerv1.CPUScalingConfigurationSpec{
				Items: []powerv1.ConfigItem{
					{
						CpuIDs:       []uint{0, 1, 3, 4},
						SamplePeriod: metav1.Duration{},
					},
					{
						CpuIDs:       []uint{1},
						SamplePeriod: metav1.Duration{},
					},
				},
			},
			expectedErr: "cpu with id 1 is specified more than once on node TestNode",
		},
		{
			testCase: "Test Case 4 - Sample Period is too low",
			spec: powerv1.CPUScalingConfigurationSpec{
				Items: []powerv1.ConfigItem{
					{
						CpuIDs: []uint{0},
						SamplePeriod: metav1.Duration{
							Duration: 1 * time.Millisecond,
						},
					},
				},
			},
			expectedErr: "sample period 1ms is below minimum limit 10ms",
		},
		{
			testCase: "Test Case 5 - Sample Period is too high",
			spec: powerv1.CPUScalingConfigurationSpec{
				Items: []powerv1.ConfigItem{
					{
						CpuIDs: []uint{0},
						SamplePeriod: metav1.Duration{
							Duration: 2 * time.Second,
						},
					},
				},
			},
			expectedErr: "sample period 2s is above maximum limit 1s",
		},
	}

	for _, tc := range tcases {
		t.Log(tc.testCase)
		t.Setenv("NODE_NAME", nodeName)
		req := reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      nodeName,
				Namespace: namespace,
			},
		}
		config := &powerv1.CPUScalingConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: namespace,
				UID:       "test",
			},
			Spec: tc.spec,
		}

		r, err := createCPUScalingConfigurationReconcilerObject([]client.Object{config})
		assert.Nil(t, err)
		r.PowerLibrary = nodemk

		_, err = r.Reconcile(context.TODO(), req)
		assert.Nil(t, err)

		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      nodeName,
			Namespace: namespace,
		}, config)
		assert.Nil(t, err)
		assert.Contains(t, config.Status.Errors, tc.expectedErr)
	}
}
