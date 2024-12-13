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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	powerv1 "github.com/intel/kubernetes-power-manager/api/v1"
	"github.com/intel/kubernetes-power-manager/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const userspaceGovernor = "userspace"

type eppValues struct {
	cpuScalingProfileSpec powerv1.CPUScalingProfileSpec
	configItem            powerv1.ConfigItem
}

var eppDefaults map[powerv1.EPP]eppValues = map[powerv1.EPP]eppValues{
	powerv1.EPPPerformance: {
		cpuScalingProfileSpec: powerv1.CPUScalingProfileSpec{
			Min:                        ptr.To(intstr.FromString("0%")),
			Max:                        ptr.To(intstr.FromString("100%")),
			SamplePeriod:               &metav1.Duration{Duration: 10 * time.Millisecond},
			CooldownPeriod:             &metav1.Duration{Duration: 3 * 10 * time.Millisecond},
			TargetBusyness:             ptr.To(80),
			AllowedBusynessDifference:  ptr.To(5),
			AllowedFrequencyDifference: ptr.To(25),
		},
		configItem: powerv1.ConfigItem{
			FallbackFreqPercent: 100,
		},
	},
	powerv1.EPPBalancePerformance: {
		cpuScalingProfileSpec: powerv1.CPUScalingProfileSpec{
			Min:                        ptr.To(intstr.FromString("0%")),
			Max:                        ptr.To(intstr.FromString("100%")),
			SamplePeriod:               &metav1.Duration{Duration: 10 * time.Millisecond},
			CooldownPeriod:             &metav1.Duration{Duration: 3 * 10 * time.Millisecond},
			TargetBusyness:             ptr.To(80),
			AllowedBusynessDifference:  ptr.To(5),
			AllowedFrequencyDifference: ptr.To(25),
		},
		configItem: powerv1.ConfigItem{
			FallbackFreqPercent: 50,
		},
	},
	powerv1.EPPBalancePower: {
		cpuScalingProfileSpec: powerv1.CPUScalingProfileSpec{
			Min:                        ptr.To(intstr.FromString("0%")),
			Max:                        ptr.To(intstr.FromString("100%")),
			SamplePeriod:               &metav1.Duration{Duration: 10 * time.Millisecond},
			CooldownPeriod:             &metav1.Duration{Duration: 3 * 10 * time.Millisecond},
			TargetBusyness:             ptr.To(80),
			AllowedBusynessDifference:  ptr.To(5),
			AllowedFrequencyDifference: ptr.To(25),
		},
		configItem: powerv1.ConfigItem{
			FallbackFreqPercent: 25,
		},
	},
	powerv1.EPPPower: {
		cpuScalingProfileSpec: powerv1.CPUScalingProfileSpec{
			Min:                        ptr.To(intstr.FromString("0%")),
			Max:                        ptr.To(intstr.FromString("100%")),
			SamplePeriod:               &metav1.Duration{Duration: 10 * time.Millisecond},
			CooldownPeriod:             &metav1.Duration{Duration: 3 * 10 * time.Millisecond},
			TargetBusyness:             ptr.To(80),
			AllowedBusynessDifference:  ptr.To(5),
			AllowedFrequencyDifference: ptr.To(25),
		},
		configItem: powerv1.ConfigItem{
			FallbackFreqPercent: 0,
		},
	},
}

// write errors to the status filed, pass nil to clear errors, will only do update resource is valid and not being deleted
// if object already has the correct errors it will not be updated in the API
func writeUpdatedStatusErrsIfRequired(ctx context.Context, statusWriter client.SubResourceWriter, object powerv1.PowerCRWithStatusErrors, objectErrors error) error {
	var err error
	// if invalid or marked for deletion don't do anything
	if object.GetUID() == "" || object.GetDeletionTimestamp() != nil {
		return err
	}
	errList := util.UnpackErrsToStrings(objectErrors)
	// no updates are needed
	if reflect.DeepEqual(*errList, *object.GetStatusErrors()) {
		return err
	}
	object.SetStatusErrors(errList)
	err = statusWriter.Update(ctx, object)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Error(err, "failed to write status update")
	}
	return err
}
