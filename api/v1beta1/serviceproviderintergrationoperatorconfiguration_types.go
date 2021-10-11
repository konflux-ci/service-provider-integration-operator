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

// ServiceProviderIntergrationOperatorConfigurationSpec defines the desired state of ServiceProviderIntergrationOperatorConfiguration
type ServiceProviderIntergrationOperatorConfigurationSpec struct {
	// The URL of HTTP API of the externally installed Vault server. If left empty a new instance of Vault is deployed using
	// the VaultDeployment configuration.
	VaultURL string `json:"vaultURL,omitempty"`

	// VaultDeployment configures the deployment of the managed Vault installation. Either this or a URL to an external Vault
	// installation must be provided.
	VaultDeployment *VaultDeployment `json:"vaultDeployment,omitempty"`
}

type VaultDeployment struct {

	// TargetNamespace is the namespace where Vault should be installed. If left empty, Vault is installed into the namespace where
	// operator is running.
	TargetNamespace string `json:"targetNamespace"`
}

// ServiceProviderIntergrationOperatorConfigurationStatus defines the observed state of ServiceProviderIntergrationOperatorConfiguration
type ServiceProviderIntergrationOperatorConfigurationStatus struct {
	// The URL of the Vault installation used. This is coming either from the VaultURL or determined from the managed Vault installation.
	VaultURL string `json:"vaultURL"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServiceProviderIntergrationOperatorConfiguration is the Schema for the serviceproviderintergrationoperatorconfigurations API
type ServiceProviderIntergrationOperatorConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceProviderIntergrationOperatorConfigurationSpec   `json:"spec,omitempty"`
	Status ServiceProviderIntergrationOperatorConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceProviderIntergrationOperatorConfigurationList contains a list of ServiceProviderIntergrationOperatorConfiguration
type ServiceProviderIntergrationOperatorConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceProviderIntergrationOperatorConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceProviderIntergrationOperatorConfiguration{}, &ServiceProviderIntergrationOperatorConfigurationList{})
}
