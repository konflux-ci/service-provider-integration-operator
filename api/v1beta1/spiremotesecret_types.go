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

// SPIRemoteSecretSpec defines the desired state of SPIRemoteSecret
type SPIRemoteSecretSpec struct {
	EnvironmentName string `json:"environmentName,omitempty"`
	SecretName      string `json:"secretname,omitempty"`
}

// SPIRemoteSecretStatus defines the observed state of SPIRemoteSecret
type SPIRemoteSecretStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

// SPIRemoteSecretReason is the reconciliation status of the SPIRemoteSecret object
type SPIRemoteSecretReason string

const (
	SPIRemoteSecretReasonAwaitingTokenData SPIRemoteSecretReason = "AwaitingSecretData"
	SPIRemoteSecretReasonInjected          SPIRemoteSecretReason = "Injected"
	SPIRemoteSecretReasonError             SPIRemoteSecretReason = "Error"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SPIRemoteSecret is the Schema for the spiremotesecret API
type SPIRemoteSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SPIAccessTokenSpec   `json:"spec,omitempty"`
	Status SPIAccessTokenStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SPIRemoteSecretList contains a list of SPIRemoteSecret
type SPIRemoteSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIRemoteSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SPIRemoteSecret{}, &SPIRemoteSecretList{})
}
