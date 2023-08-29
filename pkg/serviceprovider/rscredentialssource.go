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

package serviceprovider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/commaseparated"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var missingTargetError = errors.New("found RemoteSecret does not have a target in the SPIAccessCheck's namespace, this should not happen")

type RemoteSecretCredentialsSource struct {
	// RepoHostParser is a function that extracts the host from the repoUrl
	RepoHostParser     RepoUrlParser
	RemoteSecretFilter RemoteSecretFilter
}

var _ CredentialsSource[v1beta1.RemoteSecret] = (*RemoteSecretCredentialsSource)(nil)

func (r RemoteSecretCredentialsSource) LookupCredentialsSource(ctx context.Context, cl client.Client, matchable Matchable) (*v1beta1.RemoteSecret, error) {
	lg := log.FromContext(ctx)

	repoUrl, err := r.RepoHostParser(matchable.RepoUrl())
	if err != nil {
		return nil, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}

	potentialMatches := &v1beta1.RemoteSecretList{}
	if err := cl.List(ctx, potentialMatches, client.InNamespace(matchable.ObjNamespace()), client.MatchingLabels{
		api.RSServiceProviderHostLabel: repoUrl.Host,
	}); err != nil {
		return nil, fmt.Errorf("failed to list the potentially matching RemoteSecrets: %w", err)
	}
	lg.V(logs.DebugLevel).Info("RemoteSecret lookup", "potential_matches", len(potentialMatches.Items))

	remoteSecrets := make([]v1beta1.RemoteSecret, 0)
	// For now let's just do a linear search. In the future we can think about go func like in SPIAccessTokenLookup.
	for i := range potentialMatches.Items {
		if r.RemoteSecretFilter == nil || r.RemoteSecretFilter.Matches(ctx, matchable, &potentialMatches.Items[i]) {
			remoteSecrets = append(remoteSecrets, potentialMatches.Items[i])
		}
	}

	if len(remoteSecrets) == 0 {
		return nil, nil
	}

	matchingRemoteSecret := remoteSecrets[0] // #nosec we check if empty list above
	for _, rs := range remoteSecrets {
		accessibleRepositories := rs.Annotations[api.RSServiceProviderRepositoryAnnotation]
		if slices.Contains(commaseparated.Value(accessibleRepositories).Values(), strings.TrimPrefix(repoUrl.Path, "/")) {
			matchingRemoteSecret = rs
			break
		}
	}

	return &matchingRemoteSecret, nil
}
func (r RemoteSecretCredentialsSource) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	matchingRemoteSecret, err := r.LookupCredentialsSource(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("failed to find suitable matching RemoteSecret: %w", err)
	}
	if matchingRemoteSecret == nil {
		return nil, nil
	}
	targetIndex := getLocalNamespaceTargetIndex(matchingRemoteSecret.Status.Targets, matchable.ObjNamespace())
	if targetIndex < 0 || targetIndex >= len(matchingRemoteSecret.Status.Targets) {
		return nil, missingTargetError // Should not happen, but avoids panicking just in case.
	}

	secret := &v1.Secret{}
	err = cl.Get(ctx, client.ObjectKey{Namespace: matchable.ObjNamespace(), Name: matchingRemoteSecret.Status.Targets[targetIndex].SecretName}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to find Secret created by RemoteSecret: %w", err)
	}

	return &Credentials{Username: string(secret.Data[v1.BasicAuthUsernameKey]), Password: string(secret.Data[v1.BasicAuthPasswordKey]), SourceObjectName: matchingRemoteSecret.Name}, nil
}

// getLocalNamespaceTargetIndex is helper function which finds the index of a target in targets such that the target
// references namespace in the local cluster. If no such target exists, -1 is returned.
func getLocalNamespaceTargetIndex(targets []v1beta1.TargetStatus, namespace string) int {
	for i, target := range targets {
		if target.ApiUrl == "" && target.Error == "" && target.Namespace == namespace {
			return i
		}
	}
	return -1
}
