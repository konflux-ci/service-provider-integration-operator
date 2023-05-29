/*
Copyright 2021.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemoteSecretSpec defines the desired state of RemoteSecret
type RemoteSecretSpec struct {
	// Secret defines the properties of the secret and the linked service accounts that should be
	// created in the target namespaces.
	Secret LinkableSecretSpec `json:"secret"`
	// Targets is the list of the target namespaces that the secret and service accounts should be deployed to.
	// +optional
	Targets []RemoteSecretTarget `json:"targets,omitempty"`
}

type RemoteSecretTarget struct {
	// Namespace is the name of the target namespace to which to deploy.
	Namespace string `json:"namespace,omitempty"`
	// ApiUrl specifies the URL of the API server of a remote Kubernetes cluster that this target points to. If left empty,
	// the local cluster is assumed.
	ApiUrl string `json:"apiUrl,omitempty"`
	// ClusterCredentialsSecret is the name of the secret in the same namespace as the RemoteSecret that contains the token
	// to use to authenticate with the remote Kubernetes cluster. This is ignored if `apiUrl` is empty.
	ClusterCredentialsSecret string `json:"clusterCredentialsSecret,omitempty"`
}

// RemoteSecretStatus defines the observed state of RemoteSecret
type RemoteSecretStatus struct {
	// Conditions is the list of conditions describing the state of the deployment
	// to the targets.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Targets is the list of the deployment statuses for individual targets in the spec.
	// +optional
	Targets []TargetStatus `json:"targets,omitempty"`
}

type TargetStatus struct {
	// Namespace is the namespace of the target where the secret and the service accounts have been deployed to.
	Namespace string `json:"namespace"`
	// ApiUrl is the URL of the remote Kubernetes cluster to which the target points to.
	ApiUrl string `json:"apiUrl,omitempty"`
	// SecretName is the name of the secret that is actually deployed to the target namespace
	SecretName string `json:"secretName"`
	// ServiceAccountNames is the names of the service accounts that have been deployed to the target namespace
	// +optional
	ServiceAccountNames []string `json:"serviceAccountNames,omitempty"`
	// Error the optional error message if the deployment of either the secret or the service accounts failed.
	// +optional
	Error string `json:"error,omitempty"`
}

// RemoteSecretReason is the reconciliation status of the RemoteSecret object
type RemoteSecretReason string

// RemoteSecretConditionType lists the types of conditions we track in the remote secret status
type RemoteSecretConditionType string

const (
	RemoteSecretConditionTypeDeployed     RemoteSecretConditionType = "Deployed"
	RemoteSecretConditionTypeDataObtained RemoteSecretConditionType = "DataObtained"
	RemoteSecretConditionTypeSpecValid    RemoteSecretConditionType = "SpecValid"

	RemoteSecretReasonAwaitingTokenData RemoteSecretReason = "AwaitingData"
	RemoteSecretReasonDataFound         RemoteSecretReason = "DataFound"
	RemoteSecretReasonInjected          RemoteSecretReason = "Injected"
	RemoteSecretReasonPartiallyInjected RemoteSecretReason = "PartiallyInjected"
	RemoteSecretReasonError             RemoteSecretReason = "Error"
	RemoteSecretReasonValid             RemoteSecretReason = "Valid"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RemoteSecret is the Schema for the RemoteSecret API
type RemoteSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteSecretSpec   `json:"spec,omitempty"`
	Status RemoteSecretStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RemoteSecretList contains a list of RemoteSecret
type RemoteSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteSecret{}, &RemoteSecretList{})
}
