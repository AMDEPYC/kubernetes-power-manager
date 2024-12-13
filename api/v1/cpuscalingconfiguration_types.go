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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ConfigItem struct {
	// PowerProfile is the CPUScalingProfile that this CPUScalingConfiguration is based on
	PowerProfile string `json:"powerProfile"`

	// Frequency to set when CPU busyness is not available, in percent of max frequency
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	FallbackFreqPercent int `json:"fallbackFreqPercent"`

	// List of CPU IDs which should adhere to the configuration in this item
	//+kubebuilder:validation:MinItems=1
	CpuIDs []uint `json:"cpuIDs"`

	// Minimum time to elapse between two CPU sample periods
	//+kubebuilder:validation:Format=duration
	SamplePeriod metav1.Duration `json:"samplePeriod"`

	// Time to elapse after setting a new frequency target before next CPU sampling
	//+kubebuilder:validation:Format=duration
	CooldownPeriod metav1.Duration `json:"cooldownPeriod"`

	// Target CPU busyness, in percents
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	TargetBusyness int `json:"targetBusyness"`

	// Maximum difference between target and actual CPU busyness on which
	// frequency re-evaluation will not happen, in percent points
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=50
	AllowedBusynessDifference int `json:"allowedBusynessDifference"`

	// Maximum difference between target and actual CPU frequency on which
	// frequency re-evaluation will not happen, in MHz
	// +kubebuilder:validation:Minimum=0
	AllowedFrequencyDifference int `json:"allowedFrequencyDifference,omitempty"`

	// UID of the Pod that this ConfigItem is associated with
	PodUID types.UID `json:"podUID"`
}

// CPUScalingConfigurationSpec defines the desired state of CPUScalingConfiguration
type CPUScalingConfigurationSpec struct {
	// List of configurations that should be applied on a node.
	Items []ConfigItem `json:"items,omitempty"`
}

// CPUScalingConfigurationStatus defines the observed state of CPUScalingConfiguration
type CPUScalingConfigurationStatus struct {
	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CPUScalingConfiguration is the Schema for the cpuscalingconfiguration API
type CPUScalingConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CPUScalingConfigurationSpec   `json:"spec,omitempty"`
	Status CPUScalingConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CPUScalingConfigurationList contains a list of CPUScalingConfiguration
type CPUScalingConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CPUScalingConfiguration `json:"items"`
}

func (config *CPUScalingConfiguration) SetStatusErrors(errs *[]string) {
	config.Status.Errors = *errs
}

func (config *CPUScalingConfiguration) GetStatusErrors() *[]string {
	return &config.Status.Errors
}

func init() {
	SchemeBuilder.Register(&CPUScalingConfiguration{}, &CPUScalingConfigurationList{})
}
