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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CPUPerformanceScalingProfileSpec defines the desired state of CPUPerformanceScalingProfile
type CPUPerformanceScalingProfileSpec struct {
	// Name of the CPUPerformanceScalingProfile
	Name string `json:"name"`

	// Minimum time to elapse between two CPU sample periods
	//+kubebuilder:validation:Format=duration
	//+kubebuilder:default="10ms"
	SamplePeriod metav1.Duration `json:"samplePeriod,omitempty"`

	// Minimum frequency cores can run at
	Min int `json:"min,omitempty"`

	// Maximum frequency cores can run at
	Max int `json:"max,omitempty"`

	// The priority value associated with this CPUPerformanceScalingProfile
	Epp string `json:"epp,omitempty"`
}

// CPUPerformanceScalingProfileStatus defines the observed state of CPUPerformanceScalingProfile
type CPUPerformanceScalingProfileStatus struct {
	// The ID given to the CPUPerformanceScalingProfile
	ID           int `json:"id,omitempty"`
	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CPUPerformanceScalingProfile is the Schema for the cpuperformancescalingprofiles API
type CPUPerformanceScalingProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CPUPerformanceScalingProfileSpec   `json:"spec,omitempty"`
	Status CPUPerformanceScalingProfileStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CPUPerformanceScalingProfileList contains a list of CPUPerformanceScalingProfile
type CPUPerformanceScalingProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CPUPerformanceScalingProfile `json:"items"`
}

func (prfl *CPUPerformanceScalingProfile) SetStatusErrors(errs *[]string) {
	prfl.Status.Errors = *errs
}
func (prfl *CPUPerformanceScalingProfile) GetStatusErrors() *[]string {
	return &prfl.Status.Errors
}

func init() {
	SchemeBuilder.Register(&CPUPerformanceScalingProfile{}, &CPUPerformanceScalingProfileList{})
}
