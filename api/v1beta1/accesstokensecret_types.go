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
	// Phase describes the state in which the process of retrieving the secret data currently is.
	// Possible values are: Retrieving, Injecting, Injected or Failed. The final states are either Injected or Failed.
	Phase AccessTokenSecretPhase `json:"phase"`
	// If the phase is Failed, this field describes the reason for the failure. Possible values are:
	// SPIClientSetup, TokenRetrieval, Injection.
	Reason AccessTokenSecretFailureReason `json:"reason,omitempty"`
	// Message is the error message describing the reason for a failure.
	Message string `json:"message,omitempty"`
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
	// ConfigMap describes the configmap in which the data should be stored. The configmap will always be created in the same namespace
	// as the AccessTokenSecret.
	ConfigMap *AccessTokenTargetConfigMap `json:"configMap,omitempty"`
	// Secret describes the secret in which the data should be stored. The secret will always be created in the same namespace
	// as the AccessTokenSecret.
	Secret *AccessTokenTargetSecret `json:"secret,omitempty"`
	// Containers describes the pods/containers into which the data should be injected. THIS IS CURRENTLY NOT SUPPORTED.
	Containers *AccessTokenTargetContainers `json:"containers,omitempty"`
}

type AccessTokenTargetConfigMap struct {
	// Name is the name of the configmap to be created.
	Name string `json:"name"`
	// Labels contains the labels that the created configmap should be labeled with.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is the keys and values that the create configmap should be annotated with.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Fields specifies the mapping from the token record fields to the keys in the configmap data.
	Fields AccessTokenSecretFieldMapping `json:"fields"`
}

type AccessTokenTargetSecret struct {
	// Name is the name of the secret to be created.
	Name string `json:"name"`
	// Labels contains the labels that the created secret should be labeled with.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is the keys and values that the create secret should be annotated with.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Type is the type of the secret to be created. If left empty, the default type used in the cluster is assumed (typically Opaque).
	// The type of the secret defines the automatic mapping of the token record fields to keys in the secret data
	// according to the documentation https://kubernetes.io/docs/concepts/configuration/secret/#secret-types.
	// Only kubernetes.io/service-account-token, kubernetes.io/dockercfg, kubernetes.io/dockerconfigjson and kubernetes.io/basic-auth
	// are supported. All other secret types need to have their mapping specified manually using the Fields.
	Type corev1.SecretType `json:"type,omitempty"`
	// Fields specifies the mapping from the token record fields to the keys in the secret data.
	Fields AccessTokenSecretFieldMapping `json:"fields,omitempty"`
}

type AccessTokenTargetContainers struct {
	PodLabels  metav1.LabelSelector `json:"podLabels"`
	Containers []string             `json:"containers"`
	MountPath  string               `json:"mountPath"`
}

type AccessTokenSecretFieldMapping struct {
	// Token specifies the data key in which the token should be stored.
	Token string `json:"token,omitempty"`
	// Name specifies the data key in which the name of the token record should be stored.
	Name string `json:"name,omitempty"`
	// ServiceProviderUrl specifies the data key in which the url of the service provider should be stored.
	ServiceProviderUrl string `json:"serviceProviderUrl,omitempty"`
	// ServiceProviderUserName specifies the data key in which the url of the user name used in the service provider should be stored.
	ServiceProviderUserName string `json:"serviceProviderUserName,omitempty"`
	// ServiceProviderUserId specifies the data key in which the url of the user id used in the service provider should be stored.
	ServiceProviderUserId string `json:"serviceProviderUserId,omitempty"`
	// UserId specifies the data key in which the user id as known to the SPI should be stored (note that this is usually different from
	// ServiceProviderUserId, because the former is usually a kubernetes user, while the latter is some arbitrary ID used by the service provider
	// which might or might not correspond to the Kubernetes user id).
	UserId string `json:"userId,omitempty"`
	// ExpiredAfter specifies the data key in which the expiry date of the token should be stored.
	ExpiredAfter string `json:"expiredAfter,omitempty"`
	// Scopes specifies the data key in which the comma-separated list of token scopes should be stored.
	Scopes string `json:"scopes,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AccessTokenSecret{}, &AccessTokenSecretList{})
}
