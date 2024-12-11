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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CPUScalingProfileSpec defines the desired state of CPUScalingProfile
type CPUScalingProfileSpec struct {
	// Name of the CPUScalingProfile
	Name string `json:"name"`

	// Minimum time to elapse between two CPU sample periods
	//+kubebuilder:validation:Format=duration
	SamplePeriod metav1.Duration `json:"samplePeriod,omitempty"`

	// Max frequency cores can run at
	//+kubebuilder:validation:XIntOrString
	//+kubebuilder:validation:Pattern="^([1-9]?[0-9]|100)%$"
	Max *intstr.IntOrString `json:"max,omitempty"`

	// Min frequency cores can run at
	//+kubebuilder:validation:XIntOrString
	//+kubebuilder:validation:Pattern="^([1-9]?[0-9]|100)%$"
	Min *intstr.IntOrString `json:"min,omitempty"`

	// The priority value associated with this CPUScalingProfile
	// +kubebuilder:validation:Enum=power;balance_power;balance_performance;performance
	Epp string `json:"epp,omitempty"`
}

// CPUScalingProfileStatus defines the observed state of CPUScalingProfile
type CPUScalingProfileStatus struct {
	// The ID given to the CPUScalingProfile
	ID           int `json:"id,omitempty"`
	StatusErrors `json:",inline,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CPUScalingProfile is the Schema for the cpuscalingprofiles API
type CPUScalingProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CPUScalingProfileSpec   `json:"spec,omitempty"`
	Status CPUScalingProfileStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CPUScalingProfileList contains a list of CPUScalingProfile
type CPUScalingProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CPUScalingProfile `json:"items"`
}

func (prfl *CPUScalingProfile) SetStatusErrors(errs *[]string) {
	prfl.Status.Errors = *errs
}
func (prfl *CPUScalingProfile) GetStatusErrors() *[]string {
	return &prfl.Status.Errors
}

func init() {
	SchemeBuilder.Register(&CPUScalingProfile{}, &CPUScalingProfileList{})
}
