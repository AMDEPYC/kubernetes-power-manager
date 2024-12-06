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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/internal/metrics"
	"github.com/intel/kubernetes-power-manager/internal/scaling"
	"github.com/intel/kubernetes-power-manager/pkg/testutils"
	"github.com/intel/power-optimization-library/pkg/power"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	nodeName  string = "TestNode"
	namespace string = "power-manager"
)

type ScalingMgrMock struct {
	scaling.CPUScalingManager
	mock.Mock
}

func (m *ScalingMgrMock) UpdateConfig(configs []scaling.CPUScalingOpts) {
	m.Called(configs)
}

type DPDKTelemetryClientMock struct {
	metrics.DPDKTelemetryClient
	mock.Mock
}

func (cl *DPDKTelemetryClientMock) CreateConnection(data *metrics.DPDKTelemetryConnectionData) {
	cl.Called(data)
}

func (cl *DPDKTelemetryClientMock) ListConnections() []metrics.DPDKTelemetryConnectionData {
	args := cl.Called()
	return args.Get(0).([]metrics.DPDKTelemetryConnectionData)
}

func (cl *DPDKTelemetryClientMock) CloseConnection(podUID string) {
	cl.Called(podUID)
}

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
		s,
		nil,
		nil,
		nil,
	}

	return r, nil
}

func TestCPUScalingConfiguration_Reconcile_InvalidRequests(t *testing.T) {
	t.Setenv("NODE_NAME", nodeName)
	r, err := createCPUScalingConfigurationReconcilerObject([]client.Object{})
	assert.NoError(t, err)

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
		{
			testCase: "Test Case 6 - Cooldown Period is smaller than Sample Period",
			spec: powerv1.CPUScalingConfigurationSpec{
				Items: []powerv1.ConfigItem{
					{
						CpuIDs: []uint{0},
						SamplePeriod: metav1.Duration{
							Duration: 100 * time.Millisecond,
						},
						CooldownPeriod: metav1.Duration{
							Duration: 99 * time.Millisecond,
						},
					},
				},
			},
			expectedErr: "cooldown period 99ms must be larger than sample period 100ms",
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
		assert.NoError(t, err)
		r.PowerLibrary = nodemk

		_, err = r.Reconcile(context.TODO(), req)
		assert.NoError(t, err)

		err = r.Client.Get(context.TODO(), client.ObjectKey{
			Name:      nodeName,
			Namespace: namespace,
		}, config)
		assert.NoError(t, err)
		assert.Contains(t, config.Status.Errors, tc.expectedErr)
	}
}

func TestCPUScalingConfiguration_Reconcile_Success(t *testing.T) {
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
		Spec: powerv1.CPUScalingConfigurationSpec{
			Items: []powerv1.ConfigItem{
				{
					CpuIDs: []uint{0, 1},
					SamplePeriod: metav1.Duration{
						Duration: 10 * time.Millisecond,
					},
					CooldownPeriod: metav1.Duration{
						Duration: 30 * time.Millisecond,
					},
					TargetBusyness:             100,
					AllowedBusynessDifference:  10,
					AllowedFrequencyDifference: 25,
					FallbackFreqPercent:        50,
					PodUID:                     "foo",
				},
				{
					CpuIDs: []uint{5},
					SamplePeriod: metav1.Duration{
						Duration: 100 * time.Millisecond,
					},
					CooldownPeriod: metav1.Duration{
						Duration: 100 * time.Millisecond,
					},
					TargetBusyness:             50,
					AllowedBusynessDifference:  5,
					AllowedFrequencyDifference: 45,
					FallbackFreqPercent:        0,
					PodUID:                     "bar",
				},
				{
					CpuIDs: []uint{7},
					SamplePeriod: metav1.Duration{
						Duration: 300 * time.Millisecond,
					},
					CooldownPeriod: metav1.Duration{
						Duration: 301 * time.Millisecond,
					},
					TargetBusyness:             0,
					AllowedBusynessDifference:  0,
					AllowedFrequencyDifference: 0,
					FallbackFreqPercent:        100,
					PodUID:                     "qux",
				},
			},
		},
	}
	expectedOpts := []scaling.CPUScalingOpts{
		{
			CPUID:                      0,
			SamplePeriod:               10 * time.Millisecond,
			CooldownPeriod:             30 * time.Millisecond,
			TargetBusyness:             100,
			AllowedBusynessDifference:  10,
			AllowedFrequencyDifference: 25,
			HWMaxFrequency:             3700000,
			HWMinFrequency:             400000,
			CurrentFrequency:           -1,
			FallbackFreq:               2050000,
		},
		{
			CPUID:                      1,
			SamplePeriod:               10 * time.Millisecond,
			CooldownPeriod:             30 * time.Millisecond,
			TargetBusyness:             100,
			AllowedBusynessDifference:  10,
			AllowedFrequencyDifference: 25,
			HWMaxFrequency:             3700000,
			HWMinFrequency:             400000,
			CurrentFrequency:           -1,
			FallbackFreq:               2050000,
		},
		{
			CPUID:                      5,
			SamplePeriod:               100 * time.Millisecond,
			CooldownPeriod:             100 * time.Millisecond,
			TargetBusyness:             50,
			AllowedBusynessDifference:  5,
			AllowedFrequencyDifference: 45,
			HWMaxFrequency:             3700000,
			HWMinFrequency:             400000,
			CurrentFrequency:           -1,
			FallbackFreq:               400000,
		},
		{
			CPUID:                      7,
			SamplePeriod:               300 * time.Millisecond,
			CooldownPeriod:             301 * time.Millisecond,
			TargetBusyness:             0,
			AllowedBusynessDifference:  0,
			AllowedFrequencyDifference: 0,
			HWMaxFrequency:             3700000,
			HWMinFrequency:             400000,
			CurrentFrequency:           -1,
			FallbackFreq:               3700000,
		},
	}
	expectedFooConnData := &metrics.DPDKTelemetryConnectionData{
		PodUID:      "foo",
		WatchedCPUs: []uint{0, 1},
	}
	expectedBarConnData := &metrics.DPDKTelemetryConnectionData{
		PodUID:      "bar",
		WatchedCPUs: []uint{5},
	}

	cpuListMock := power.CpuList{}
	for i := 0; i < 10; i++ {
		mockCore := new(testutils.MockCPU)
		mockCore.On("GetID").Return(uint(i))
		cpuListMock = append(cpuListMock, mockCore)
	}
	nodemk := new(testutils.MockHost)
	nodemk.On("GetAllCpus").Return(&cpuListMock)

	freqsetmk := new(testutils.MockFreqSet)
	freqsetmk.On("GetMin").Return(uint(400000))
	freqsetmk.On("GetMax").Return(uint(3700000))
	nodemk.On("GetFreqRanges").Return(power.CoreTypeList{freqsetmk})

	mgrmk := new(ScalingMgrMock)
	mgrmk.On("UpdateConfig", expectedOpts).Return()

	dpdkmk := new(DPDKTelemetryClientMock)
	dpdkmk.On("ListConnections").Return(
		[]metrics.DPDKTelemetryConnectionData{
			{PodUID: "baz", WatchedCPUs: []uint{5}},
			{PodUID: "qux", WatchedCPUs: []uint{7}},
		})
	dpdkmk.On("CloseConnection", "baz").Return()
	dpdkmk.On("CreateConnection", expectedFooConnData).Return()
	dpdkmk.On("CreateConnection", expectedBarConnData).Return()

	r, err := createCPUScalingConfigurationReconcilerObject([]client.Object{config})
	assert.NoError(t, err)

	r.PowerLibrary = nodemk
	r.CPUScalingManager = mgrmk
	r.DPDKTelemetryClient = dpdkmk

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	mgrmk.AssertCalled(t, "UpdateConfig", expectedOpts)
	dpdkmk.AssertCalled(t, "ListConnections")
	dpdkmk.AssertCalled(t, "CloseConnection", "baz")
	dpdkmk.AssertCalled(t, "CreateConnection", expectedFooConnData)
	dpdkmk.AssertCalled(t, "CreateConnection", expectedBarConnData)

	err = r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      nodeName,
		Namespace: namespace,
	}, config)
	assert.NoError(t, err)
	assert.Empty(t, config.Status.Errors)
}

func TestCPUScalingConfiguration_Reconcile_NotFound(t *testing.T) {
	t.Setenv("NODE_NAME", nodeName)

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      nodeName,
			Namespace: namespace,
		},
	}

	cpuListMock := power.CpuList{}
	for i := 0; i < 10; i++ {
		mockCore := new(testutils.MockCPU)
		mockCore.On("GetID").Return(uint(i))
		cpuListMock = append(cpuListMock, mockCore)
	}
	nodemk := new(testutils.MockHost)
	nodemk.On("GetAllCpus").Return(&cpuListMock)

	mgrmk := new(ScalingMgrMock)
	mgrmk.On("UpdateConfig", []scaling.CPUScalingOpts{}).Return()

	dpdkmk := new(DPDKTelemetryClientMock)
	dpdkmk.On("ListConnections").Return(
		[]metrics.DPDKTelemetryConnectionData{
			{PodUID: "foo", WatchedCPUs: []uint{2, 3}},
			{PodUID: "bar", WatchedCPUs: []uint{5}},
		})
	dpdkmk.On("CloseConnection", "foo").Return()
	dpdkmk.On("CloseConnection", "bar").Return()

	r, err := createCPUScalingConfigurationReconcilerObject([]client.Object{})
	assert.NoError(t, err)

	r.PowerLibrary = nodemk
	r.CPUScalingManager = mgrmk
	r.DPDKTelemetryClient = dpdkmk

	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	mgrmk.AssertCalled(t, "UpdateConfig", []scaling.CPUScalingOpts{})
	dpdkmk.AssertCalled(t, "ListConnections")
	dpdkmk.AssertCalled(t, "CloseConnection", "foo")
	dpdkmk.AssertCalled(t, "CloseConnection", "bar")
}
