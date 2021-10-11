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

// AccessTokenSecretSpec defines the desired state of AccessTokenSecret
type AccessTokenSecretSpec struct {
	// ServiceAccount is the service account to impersonate as when retrieving the access token from the storage
	// If the namespace is not specified, the namespace of this object is assumed
	// If the name is not specified, the controller attempts to access the storage using its service account
	ServiceAccount NamespacedName `json:"serviceAccount"`

	// AccessTokenId is the ID of the access token in the storage
	AccessTokenId string `json:"accessTokenId"`

	// Target specifies the object to which the access token should be persisted to
	Target AccessTokenTarget `json:"target"`
}

// AccessTokenSecretStatus defines the observed state of AccessTokenSecret
type AccessTokenSecretStatus struct {
	Phase   AccessTokenSecretPhase         `json:"phase"`
	Reason  AccessTokenSecretFailureReason `json:"reason"`
	Message string                         `json:"message"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=ats
//+kubebuilder:subresource:status

// AccessTokenSecret is the Schema for the accesstokensecrets API
type AccessTokenSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessTokenSecretSpec   `json:"spec,omitempty"`
	Status AccessTokenSecretStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AccessTokenSecretList contains a list of AccessTokenSecret
type AccessTokenSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessTokenSecret `json:"items"`
}

// NamespacedName is a copy of the "canonical" NamespacedName from k8s.io/apimachinery/pkg/types but with additional
// serialization info so that it can be used in the CR spec.
type NamespacedName struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

type AccessTokenSecretPhase string

const (
	AccessTokenSecretPhaseFailed     AccessTokenSecretPhase = "Failed"
	AccessTokenSecretPhaseRetrieving AccessTokenSecretPhase = "Retrieving"
	AccessTokenSecretPhaseInjecting  AccessTokenSecretPhase = "Injecting"
	AccessTokenSecretPhaseInjected   AccessTokenSecretPhase = "Injected"
)

type AccessTokenSecretFailureReason string

const (
	AccessTokenSecretFailureRetrieval AccessTokenSecretFailureReason = "Retrieval"
	AccessTokenSecretFailureInjection AccessTokenSecretFailureReason = "Injection"
)

type AccessTokenTarget struct {
	ConfigMap  *AccessTokenTargetConfigMap  `json:"configMap,omitempty"`
	Secret     *AccessTokenTargetSecret     `json:"secret,omitempty"`
	Containers *AccessTokenTargetContainers `json:"containers,omitempty"`
}

type AccessTokenTargetConfigMap struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type AccessTokenTargetSecret struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type AccessTokenTargetContainers struct {
	PodLabels  metav1.LabelSelector `json:"podLabels"`
	Containers []string             `json:"containers"`
}

func init() {
	SchemeBuilder.Register(&AccessTokenSecret{}, &AccessTokenSecretList{})
}
