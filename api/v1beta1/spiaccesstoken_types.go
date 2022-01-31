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
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ServiceProviderTypeLabel = "spi.appstudio.redhat.com/service-provider-type"
	ServiceProviderHostLabel = "spi.appstudio.redhat.com/service-provider-host"
)

// SPIAccessTokenSpec defines the desired state of SPIAccessToken
type SPIAccessTokenSpec struct {
	//+kubebuilder:validation:Required
	ServiceProviderType ServiceProviderType `json:"serviceProviderType"`
	Permissions         Permissions         `json:"permissions"`
	//+kubebuilder:validation:Required
	ServiceProviderUrl string `json:"serviceProviderUrl"`
	DataLocation       string `json:"dataLocation,omitempty"`
	RawTokenData       *Token `json:"rawTokenData,omitempty"`
}

// Token is copied from golang.org/x/oauth2 and made easily json-serializable. It represents the data obtained from the
// OAuth flow.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expiry       uint64 `json:"expiry,omitempty"`
}

// TokenMetadata is data about the token retrieved from the service provider. This data can be used for matching the
// tokens with the token bindings.
type TokenMetadata struct {
	UserName string   `json:"userName"`
	UserId   string   `json:"userId"`
	Scopes   []string `json:"scopes"`
}

// Permissions is a collection of operator-defined permissions (which are translated to service-provider-specific
// scopes) and potentially additional service-provider-specific scopes that are not covered by the operator defined
// abstraction. The permissions are used in SPIAccessTokenBinding objects to express the requirements on the tokens as
// well as in the SPIAccessToken objects to express the "capabilities" of the token.
type Permissions struct {
	Required         []Permission `json:"required,omitempty"`
	AdditionalScopes []string     `json:"additionalScopes,omitempty"`
}

// ServiceProviderType defines the set of supported service providers
type ServiceProviderType string

const (
	ServiceProviderTypeGitHub ServiceProviderType = "GitHub"
	ServiceProviderTypeQuay   ServiceProviderType = "Quay"
)

// Permission is an element of Permissions and express a requirement on the service provider scopes in an agnostic
// manner.
type Permission struct {
	// Type is the type of the permission required
	Type PermissionType `json:"type"`

	// Area express the "area" in the service provider scopes to which the permission is required.
	Area PermissionArea `json:"area"`
}

// PermissionType expresses whether we need a permission to read or write data in a specific PermissionArea of
// the service provider
type PermissionType string

const (
	PermissionTypeRead      PermissionType = "r"
	PermissionTypeWrite     PermissionType = "w"
	PermissionTypeReadWrite PermissionType = "rw"
)

// IsRead returns true if the permission type requires read access to the service provider.
func (pt PermissionType) IsRead() bool {
	return pt == PermissionTypeRead || pt == PermissionTypeReadWrite
}

// IsWrite returns true if the permission type requires write access to the service provider.
func (pt PermissionType) IsWrite() bool {
	return pt == PermissionTypeWrite || pt == PermissionTypeReadWrite
}

// PermissionArea defines a set of the supported permission areas. A service provider implementation might not support
// all of them depending on the capabilities of the service provider (e.g. if a service provider doesn't support
// webhooks, it doesn't make sense to specify permissions in the webhook area).
type PermissionArea string

const (
	PermissionAreaRepository PermissionArea = "repository"
	PermissionAreaWebhooks   PermissionArea = "webhooks"
	PermissionAreaUser       PermissionArea = "user"
)

// SPIAccessTokenStatus defines the observed state of SPIAccessToken
type SPIAccessTokenStatus struct {
	Phase         SPIAccessTokenPhase `json:"phase"`
	OAuthUrl      string              `json:"oAuthUrl"`
	TokenMetadata *TokenMetadata      `json:"tokenMetadata"`
}

// SPIAccessTokenPhase is the reconciliation phase of the SPIAccessToken object
type SPIAccessTokenPhase string

const (
	SPIAccessTokenPhaseAwaitingTokenData SPIAccessTokenPhase = "AwaitingTokenData"
	SPIAccessTokenPhaseReady             SPIAccessTokenPhase = "Ready"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SPIAccessToken is the Schema for the spiaccesstokens API
type SPIAccessToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SPIAccessTokenSpec   `json:"spec,omitempty"`
	Status SPIAccessTokenStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SPIAccessTokenList contains a list of SPIAccessToken
type SPIAccessTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SPIAccessToken `json:"items"`
}

// EnsureLabels makes sure that the object has labels set according to its spec. The labels are used for faster lookup during
// token matching with bindings. Returns `true` if the labels were changed, `false` otherwise.
func (t *SPIAccessToken) EnsureLabels() (changed bool) {
	if t.Labels == nil {
		t.Labels = map[string]string{}
	}

	if t.Labels[ServiceProviderTypeLabel] != string(t.Spec.ServiceProviderType) {
		t.Labels[ServiceProviderTypeLabel] = string(t.Spec.ServiceProviderType)
		changed = true
	}

	if len(t.Spec.ServiceProviderUrl) > 0 {
		// we can't use the full service provider URL as a label value, because K8s doesn't allow :// in label values.
		spUrl, err := url.Parse(t.Spec.ServiceProviderUrl)
		if err == nil {
			if t.Labels[ServiceProviderHostLabel] != spUrl.Host {
				t.Labels[ServiceProviderHostLabel] = spUrl.Host
				changed = true
			}
		}
	}

	return
}

func init() {
	SchemeBuilder.Register(&SPIAccessToken{}, &SPIAccessTokenList{})
}
