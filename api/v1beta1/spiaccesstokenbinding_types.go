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

// SPIAccessTokenBindingSpec defines the desired state of SPIAccessTokenBinding
type SPIAccessTokenBindingSpec struct {
	RepoUrl     string                  `json:"repoUrl"`
	Permissions []Permission            `json:"permissions"`
	Secret      AccessTokenTargetSecret `json:"secret"`
}

// SPIAccessTokenBindingStatus defines the observed state of SPIAccessTokenBinding
type SPIAccessTokenBindingStatus struct {
	Phase                 SPIAccessTokenBindingPhase       `json:"phase"`
	ErrorReason           SPIAccessTokenBindingErrorReason `json:"errorReason,omitempty"`
	ErrorMessage          string                           `json:"errorMessage,omitempty"`
	LinkedAccessTokenName string                           `json:"linkedAccessTokenName"`
}

type SPIAccessTokenBindingPhase string

const (
	SPIAccessTokenBindingPhaseAwaitingTokenData = "AwaitingTokenData"
	SPIAccessTokenBindingPhaseInjected          = "Injected"
	SPIAccessTokenBindingPhaseError             = "Error"
)

type SPIAccessTokenBindingErrorReason string

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SPIAccessTokenBinding is the Schema for the spiaccesstokenbindings API
type SPIAccessTokenBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SPIAccessTokenBindingSpec   `json:"spec,omitempty"`
	Status SPIAccessTokenBindingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SPIAccessTokenBindingList contains a list of SPIAccessTokenBinding
type SPIAccessTokenBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIAccessTokenBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SPIAccessTokenBinding{}, &SPIAccessTokenBindingList{})
}
