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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AccessTokenSecretSpec defines the desired state of AccessTokenSecret
type AccessTokenSecretSpec struct {
	// TODO use this
	// SpiAccessSecret is the unique string provided when an access token was stored using
	// Service Provider Integration REST API.
	//SpiAccessSecret string `json:"spiAccessSecret"`

	// AccessTokenName is the ID of the access token in the storage
	AccessTokenName string `json:"accessTokenName"`

	// Target specifies the object to which the access token should be persisted
	Target AccessTokenTarget `json:"target"`
}

// AccessTokenSecretStatus defines the observed state of AccessTokenSecret
type AccessTokenSecretStatus struct {
	Phase   AccessTokenSecretPhase         `json:"phase"`
	Reason  AccessTokenSecretFailureReason `json:"reason,omitempty"`
	Message string                         `json:"message,omitempty"`
	// ObjectRef stores the information about the object into which the token information has been injected
	// This field is filled in only if the `Phase` is "Injected".
	ObjectRef AccessTokenSecretStatusObjectRef `json:"objectRef,omitempty"`
}

type AccessTokenSecretStatusObjectRef struct {
	// Name is the name of the object with the injected data. This always lives in the same namespace as the AccessTokenSecret object.
	Name string `json:"name"`
	// Kind is the kind of the object with the injected data.
	Kind string `json:"kind"`
	// ApiVersion is the api version of the object with the injected data.
	ApiVersion string `json:"apiVersion"`
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

type AccessTokenSecretPhase string

const (
	AccessTokenSecretPhaseFailed     AccessTokenSecretPhase = "Failed"
	AccessTokenSecretPhaseRetrieving AccessTokenSecretPhase = "Retrieving"
	AccessTokenSecretPhaseInjecting  AccessTokenSecretPhase = "Injecting"
	AccessTokenSecretPhaseInjected   AccessTokenSecretPhase = "Injected"
)

type AccessTokenSecretFailureReason string

const (
	AccessTokenSecretFailureSPIClientSetup AccessTokenSecretFailureReason = "SPIClientSetup"
	AccessTokenSecretFailureTokenRetrieval AccessTokenSecretFailureReason = "TokenRetrieval"
	AccessTokenSecretFailureInjection      AccessTokenSecretFailureReason = "Injection"
)

type AccessTokenTarget struct {
	ConfigMap  *AccessTokenTargetConfigMap  `json:"configMap,omitempty"`
	Secret     *AccessTokenTargetSecret     `json:"secret,omitempty"`
	Containers *AccessTokenTargetContainers `json:"containers,omitempty"`
}

type AccessTokenTargetConfigMap struct {
	Name        string                        `json:"name"`
	Labels      map[string]string             `json:"labels,omitempty"`
	Annotations map[string]string             `json:"annotations,omitempty"`
	Fields      AccessTokenSecretFieldMapping `json:"fields"`
}

type AccessTokenTargetSecret struct {
	Name        string                        `json:"name"`
	Labels      map[string]string             `json:"labels,omitempty"`
	Annotations map[string]string             `json:"annotations,omitempty"`
	Type        corev1.SecretType             `json:"type,omitempty"`
	Fields      AccessTokenSecretFieldMapping `json:"fields,omitempty"`
}

type AccessTokenTargetContainers struct {
	PodLabels  metav1.LabelSelector `json:"podLabels"`
	Containers []string             `json:"containers"`
	MountPath  string               `json:"mountPath"`
}

type AccessTokenSecretFieldMapping struct {
	Token                   string `json:"token,omitempty"`
	Name                    string `json:"name,omitempty"`
	ServiceProviderUrl      string `json:"serviceProviderUrl,omitempty"`
	ServiceProviderUserName string `json:"serviceProviderUserName,omitempty"`
	ServiceProviderUserId   string `json:"serviceProviderUserId,omitempty"`
	UserId                  string `json:"userId,omitempty"`
	ExpiredAfter            string `json:"expiredAfter,omitempty"`
	Scopes                  string `json:"scopes,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AccessTokenSecret{}, &AccessTokenSecretList{})
}
