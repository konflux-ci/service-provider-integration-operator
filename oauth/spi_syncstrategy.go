//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oauth

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	v1 "k8s.io/api/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SPIAccessTokenSyncStrategy is used to check before the OAuth flow started whether user has permission to create
// SPIAccessTokenDataUpdate, and after OAuth flow is done it stores the OAuth token to (notifying) Token Storage.
// This triggers creation of SPIAccessTokenDataUpdate using user's k8s token.
type SPIAccessTokenSyncStrategy struct {
	ClientFactory kubernetesclient.K8sClientFactory
	TokenStorage  tokenstorage.TokenStorage
}

var _ tokenDataSyncStrategy = (*SPIAccessTokenSyncStrategy)(nil)

func (s SPIAccessTokenSyncStrategy) checkIdentityHasAccess(ctx context.Context, namespace string) (bool, error) {
	review := v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "create",
				Group:     api.GroupVersion.Group,
				Version:   api.GroupVersion.Version,
				Resource:  "spiaccesstokendataupdates",
			},
		},
	}

	k8sClient, err := s.ClientFactory.CreateClient(clientfactory.NamespaceIntoContext(ctx, namespace))
	if err != nil {
		return false, fmt.Errorf("failed to create K8S client for namespace %s: %w", namespace, err)
	}
	if err := k8sClient.Create(ctx, &review); err != nil {
		return false, fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}

	log.FromContext(ctx).V(logs.DebugLevel).Info("self subject review result", "review", &review)
	return review.Status.Allowed, nil
}

func (s SPIAccessTokenSyncStrategy) syncTokenData(ctx context.Context, exchange *exchangeResult) error {
	ctx = clientfactory.WithAuthIntoContext(exchange.authorizationHeader, ctx)

	accessToken := &api.SPIAccessToken{}
	ctx = clientfactory.NamespaceIntoContext(ctx, exchange.ObjectNamespace)
	k8sClient, err := s.ClientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create K8S client for namespace %s: %w", exchange.ObjectNamespace, err)
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: exchange.ObjectName, Namespace: exchange.ObjectNamespace}, accessToken); err != nil {
		return fmt.Errorf("failed to get the SPIAccessToken object %s/%s: %w", exchange.ObjectNamespace, exchange.ObjectName, err)
	}

	apiToken := api.Token{
		AccessToken:  exchange.token.AccessToken,
		TokenType:    exchange.token.TokenType,
		RefreshToken: exchange.token.RefreshToken,
		Expiry:       uint64(exchange.token.Expiry.Unix()),
	}

	if err := s.TokenStorage.Store(ctx, accessToken, &apiToken); err != nil {
		return fmt.Errorf("failed to persist the token to storage: %w", err)
	}

	return nil
}
