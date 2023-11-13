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
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"
	v1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RemoteSecretSyncStrategy is used to check before OAuth flow whether user has permissions to sync (save) the OAuth
// token data to RemoteSecret, as well as uploading the OAuth token to RemoteSecret's data field using user's k8s token,
// after OAuth flow is done.
type RemoteSecretSyncStrategy struct {
	ClientFactory kubernetesclient.K8sClientFactory
}

var _ tokenDataSyncStrategy = (*RemoteSecretSyncStrategy)(nil)

const oAuthAssociatedUsername = "$oauthtoken"

// checkIdentityHasAccess, using user's k8s token, creates two SelfSubjectAccessReviews for RemoteSecrets in namespace,
// for 'get' and 'update' verbs. User has access if both objects have status allowed.
func (r RemoteSecretSyncStrategy) checkIdentityHasAccess(ctx context.Context, namespace string) (bool, error) {
	getReview := v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "get",
				Group:     v1beta1.GroupVersion.Group,
				Version:   v1beta1.GroupVersion.Version,
				Resource:  "remotesecrets",
			},
		},
	}

	updateReview := v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "update",
				Group:     v1beta1.GroupVersion.Group,
				Version:   v1beta1.GroupVersion.Version,
				Resource:  "remotesecrets",
			},
		},
	}

	k8sClient, err := r.ClientFactory.CreateClient(clientfactory.NamespaceIntoContext(ctx, namespace))
	if err != nil {
		return false, fmt.Errorf("failed to create K8S client for namespace %s: %w", namespace, err)
	}
	if err := k8sClient.Create(ctx, &getReview); err != nil {
		return false, fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}
	log.FromContext(ctx).V(logs.DebugLevel).Info("self subject review of 'get' verb result", "review", &getReview)
	if !getReview.Status.Allowed {
		return false, nil
	}

	if err := k8sClient.Create(ctx, &updateReview); err != nil {
		return false, fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}
	log.FromContext(ctx).V(logs.DebugLevel).Info("self subject review of 'update' verb result", "review", &updateReview)

	return updateReview.Status.Allowed, nil
}

// syncTokenData, using user's token, updates RemoteSecret with the OAuth token data. The RemoteSecret webhook should
// react to this update and store the data in secret storage.
func (r RemoteSecretSyncStrategy) syncTokenData(ctx context.Context, exchange *exchangeResult) error {
	ctx = clientfactory.WithAuthIntoContext(exchange.authorizationHeader, ctx)
	ctx = clientfactory.NamespaceIntoContext(ctx, exchange.ObjectNamespace)
	k8sClient, err := r.ClientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create K8S client for namespace %s: %w", exchange.ObjectNamespace, err)
	}

	data := map[string]string{
		// Mapping access token to password key so that it can be used from basic-auth secret.
		corev1.BasicAuthPasswordKey: exchange.token.AccessToken,
		// Hard-coding the username to this value because it is required for Quay and other providers should not need
		// username at all.
		corev1.BasicAuthUsernameKey: oAuthAssociatedUsername,
		// For now, we will ignore the refresh token to avoid spreading this powerful token because there is no mechanism
		// in RemoteSecret for selecting which key-value pair ends up in target secret. We keep tokenType and expiry as it
		// is just metadata.
		"tokenType": exchange.token.TokenType,
		"expiry":    strconv.FormatUint(uint64(exchange.token.Expiry.Unix()), 10),
	}

	remoteSecret := &v1beta1.RemoteSecret{}
	// We are using RetryOnConflict as we want to avoid error-ing out because of conflict on RemoteSecret update,
	// which would cause the whole OAuthFlow to be wasted.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: exchange.ObjectName, Namespace: exchange.ObjectNamespace}, remoteSecret); err != nil {
			return fmt.Errorf("failed to get the RemoteSecret object %s/%s: %w", exchange.ObjectNamespace, exchange.ObjectName, err)
		}
		remoteSecret.StringUploadData = data
		if err := k8sClient.Update(ctx, remoteSecret); err != nil {
			return fmt.Errorf("failed to persist token to RemoteSecret: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to persist the token to RemoteSecret after multiple retries: %w", err)
	}

	return nil
}
