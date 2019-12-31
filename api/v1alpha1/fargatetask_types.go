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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhaseDone = "DONE"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type AwsVpcConfiguration struct {
	Subnets        []string `json:"subnets,omitempty"`
	SecurityGroups []string `json:"securityGroups,omitempty"`
	AssignPublicIp string   `json:"assignPublicIp,omitempty"`
}

type NetworkConfiguration struct {
	AwsVpcConfiguration AwsVpcConfiguration `json:"awsVpcConfiguration,omitempty"`
}

// FargateTaskSpec defines the desired state of FargateTask
type FargateTaskSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of FargateTask. Edit FargateTask_types.go to remove/update
	Region               string               `json:"region,omitempty"`
	Cluster              string               `json:"cluster,omitempty"`
	Count                int64                `json:"count,omitempty"`
	NetworkConfiguration NetworkConfiguration `json:"networkConfiguration,omitempty"`
	TaskDefinition       string               `json:"taskDefinition,omitempty"`
}

// FargateTaskStatus defines the observed state of FargateTask
type FargateTaskStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	TaskArns []string `json:"taskArns,omitempty"`
	Phase    string   `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true

// FargateTask is the Schema for the fargatetasks API
type FargateTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FargateTaskSpec   `json:"spec,omitempty"`
	Status FargateTaskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FargateTaskList contains a list of FargateTask
type FargateTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FargateTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FargateTask{}, &FargateTaskList{})
}
