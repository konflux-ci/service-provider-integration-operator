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
	"net/url"
)

const (
	ServiceProviderTypeLabel = "spi.appstudio.redhat.com/service-provider-type"
	ServiceProviderHostLabel = "spi.appstudio.redhat.com/service-provider-host"
)

// SPIAccessTokenSpec defines the desired state of SPIAccessToken
type SPIAccessTokenSpec struct {
	ServiceProviderType ServiceProviderType `json:"serviceProviderType"`
	Permissions         []Permission        `json:"permissions"`
	ServiceProviderUrl  string              `json:"serviceProviderUrl"`
	DataLocation        string              `json:"dataLocation"`
	TokenMetadata       *TokenMetadata      `json:"tokenMetadata,omitempty"`
	RawTokenData        *Token              `json:"rawTokenData,omitempty"`
}

// Token is copied from golang.org/x/oauth2 and made easily json-serializable. It represents the data obtained from the
// OAuth flow.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expiry       uint64 `json:"expiry,omitempty"`
}

type TokenMetadata struct {
	UserName string   `json:"userName"`
	UserId   string   `json:"userId"`
	Scopes   []string `json:"scopes"`
}

type ServiceProviderType string

const (
	ServiceProviderTypeGitHub ServiceProviderType = "GitHub"
	ServiceProviderTypeQuay   ServiceProviderType = "Quay"
)

type Permission string

const (
	PermissionRead  Permission = "r"
	PermissionWrite Permission = "w"
)

// SPIAccessTokenStatus defines the observed state of SPIAccessToken
type SPIAccessTokenStatus struct {
	Phase    SPIAccessTokenPhase `json:"phase"`
	OAuthUrl string              `json:"oAuthUrl"`
}

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

// SyncLabels makes sure that the object has labels set according to its spec. The labels are used for faster lookup during
// token matching with bindings. Returns `true` if the labels were changed, `false` otherwise.
func (t *SPIAccessToken) SyncLabels() (changed bool) {
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
