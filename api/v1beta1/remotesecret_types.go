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
	Secret LinkableSecretSpec `json:"secret"`
	Target RemoteSecretTarget `json:"target"`
}

type RemoteSecretTarget struct {
	Environment string `json:"environment,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

// RemoteSecretStatus defines the observed state of RemoteSecret
type RemoteSecretStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	Target TargetStatus `json:"target,omitempty"`
}

type TargetStatus struct {
	Namespace NamespaceTargetStatus `json:"namespace,omitempty"`
}

type NamespaceTargetStatus struct {
	Namespace  string `json:"namespace"`
	SecretName string `json:"secretName"`
	// +optional
	ServiceAccountNames []string `json:"serviceAccountNames,omitempty"`
}

// RemoteSecretReason is the reconciliation status of the RemoteSecret object
type RemoteSecretReason string

// RemoteSecretConditionType lists the types of conditions we track in the remote secret status
type RemoteSecretConditionType string

const (
	RemoteSecretConditionTypeDeployed     RemoteSecretConditionType = "Deployed"
	RemoteSecretConditionTypeDataObtained RemoteSecretConditionType = "DataObtained"
	RemoteSecretConditionTypeError        RemoteSecretConditionType = "Error"

	RemoteSecretReasonAwaitingTokenData RemoteSecretReason = "AwaitingData"
	RemoteSecretReasonInjected          RemoteSecretReason = "Injected"
	RemoteSecretReasonError             RemoteSecretReason = "Error"
	RemoteSecretReasonCreated           RemoteSecretReason = "Created"
	RemoteSecretReasonUnsupported       RemoteSecretReason = "Unsupported"
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
