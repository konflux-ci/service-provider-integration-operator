package serviceprovider

import (
	"context"
	"errors"
	"fmt"
	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	"github.com/redhat-appstudio/remote-secret/pkg/commaseparated"
	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

type Credentials struct {
	Username string
	Password string
	//TODO Questionable!!!
	Owner *metav1.OwnerReference
}

var accessTokenNotFoundError = errors.New("token data is not found in token storage")

type CredentialsSource[D any] interface {
	////TODO not sure if this method is needed.
	LookupCredentialSource(ctx context.Context, cl client.Client, matchable Matchable) (*D, error)
	//TODO think about notfound error or nil
	LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error)
}

// TODO is it working?
var _ CredentialsSource[api.SPIAccessToken] = (*SpiTokenCredentialsSource)(nil)

type SpiTokenCredentialsSource struct {
	CredentialsSource[api.SPIAccessToken]
	// ServiceProviderType is just the type of the provider we're dealing with. It is used to limit the number of
	// results the filter function needs to sift through.
	ServiceProviderType api.ServiceProviderType
	// TokenFilter is the filter function that decides whether a token matches the requirements of a binding, given
	// the token's service-provider-specific state
	TokenFilter TokenFilter
	//	RemoteSecretFilter RemoteSecretFilter
	// MetadataProvider is used to figure out metadata of a token in the service provider useful for token lookup
	MetadataProvider MetadataProvider
	// MetadataCache is an abstraction used for storing/fetching the metadata of tokens
	MetadataCache *MetadataCache
	// RepoHostParser is a function that extracts the host from the repoUrl
	RepoHostParser RepoHostParser

	TokenStorage tokenstorage.TokenStorage
}

func (s SpiTokenCredentialsSource) LookupCredentialSource(ctx context.Context, cl client.Client, matchable Matchable) (*api.SPIAccessToken, error) {
	lg := log.FromContext(ctx)

	var result = make([]api.SPIAccessToken, 0)

	potentialMatches := &api.SPIAccessTokenList{}

	repoHost, err := s.RepoHostParser(matchable.RepoUrl())
	if err != nil {
		return nil, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}

	if err := cl.List(ctx, potentialMatches, client.InNamespace(matchable.ObjNamespace()), client.MatchingLabels{
		api.ServiceProviderTypeLabel: string(s.ServiceProviderType),
		api.ServiceProviderHostLabel: repoHost,
	}); err != nil {
		return nil, fmt.Errorf("failed to list the potentially matching tokens: %w", err)
	}

	lg.V(logs.DebugLevel).Info("lookup", "potential_matches", len(potentialMatches.Items))

	errs := make([]error, 0)

	mutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, t := range potentialMatches.Items {
		if t.Status.Phase != api.SPIAccessTokenPhaseReady {
			lg.V(logs.DebugLevel).Info("skipping lookup, token not ready", "token", t.Name)
			continue
		}

		wg.Add(1)
		go func(tkn api.SPIAccessToken) {
			lg.V(logs.DebugLevel).Info("matching", "token", tkn.Name)
			defer wg.Done()
			if err := s.MetadataCache.Ensure(ctx, &tkn, s.MetadataProvider); err != nil {
				mutex.Lock()
				defer mutex.Unlock()
				lg.Error(err, "failed to refresh the metadata of candidate token", "token", tkn.Namespace+"/"+tkn.Name)
				errs = append(errs, err)
				return
			}

			ok, err := s.TokenFilter.Matches(ctx, matchable, &tkn)
			if err != nil {
				mutex.Lock()
				defer mutex.Unlock()
				lg.Error(err, "failed to match candidate token", "token", tkn.Namespace+"/"+tkn.Name)
				errs = append(errs, err)
				return
			}
			if ok {
				mutex.Lock()
				defer mutex.Unlock()
				result = append(result, tkn)
			}
		}(t)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("errors while examining the potential matches: %w", kubeerrors.NewAggregate(errs))
	}
	lg.V(logs.DebugLevel).Info("lookup finished", "matching_tokens", len(result))
	return &result[0], nil
}
func (s SpiTokenCredentialsSource) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	lg := log.FromContext(ctx)
	spiToken, err := s.LookupCredentialSource(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("unable to find Secret created by SpiToken: %w", err)
	}

	tokenData, tsErr := s.TokenStorage.Get(ctx, spiToken)
	//lg := log.FromContext(ctx)
	if tsErr != nil {

		lg.Error(tsErr, "failed to get token from storage for", "token", spiToken)
		return nil, fmt.Errorf("failed to get token from storage for %s/%s: %w", spiToken.Namespace, spiToken.Name, tsErr)
	}
	if tokenData == nil {
		lg.Error(accessTokenNotFoundError, "token data not found", "token-name", spiToken.Name)
		return nil, accessTokenNotFoundError
	}
	// Create a new owner ref.
	gvk, err := apiutil.GVKForObject(spiToken, scheme.Scheme)
	if err != nil {
		return nil, err
	}
	ref := &metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        spiToken.GetUID(),
		Name:       spiToken.GetName(),
	}

	return &Credentials{Username: tokenData.Username, Password: tokenData.AccessToken, Owner: ref}, nil
}

type RemoteSecretCredentialsSource struct {
	CredentialsSource[v1beta1.RemoteSecret]
	// RepoHostParser is a function that extracts the host from the repoUrl
	RepoHostParser     RepoHostParser
	RemoteSecretFilter RemoteSecretFilter
}

// TODO is it working?
var _ CredentialsSource[v1beta1.RemoteSecret] = (*RemoteSecretCredentialsSource)(nil)

func (r RemoteSecretCredentialsSource) LookupCredentialSource(ctx context.Context, cl client.Client, matchable Matchable) (*v1beta1.RemoteSecret, error) {
	lg := log.FromContext(ctx)

	repoHost, err := r.RepoHostParser(matchable.RepoUrl())
	if err != nil {
		return nil, fmt.Errorf("error parsing the host from repo URL %s: %w", matchable.RepoUrl(), err)
	}

	potentialMatches := &v1beta1.RemoteSecretList{}
	if err := cl.List(ctx, potentialMatches, client.InNamespace(matchable.ObjNamespace()), client.MatchingLabels{
		api.RSServiceProviderHostLabel: repoHost,
	}); err != nil {
		return nil, fmt.Errorf("failed to list the potentially matching remote secrets: %w", err)
	}
	lg.V(logs.DebugLevel).Info("remote secret lookup", "potential_matches", len(potentialMatches.Items))

	remoteSecrets := make([]v1beta1.RemoteSecret, 0)
	// For now let's just do a linear search. In the future we can think about go func like in Lookup.
	for i := range potentialMatches.Items {
		if r.RemoteSecretFilter == nil || r.RemoteSecretFilter.Matches(ctx, matchable, &potentialMatches.Items[i]) {
			remoteSecrets = append(remoteSecrets, potentialMatches.Items[i])
		}
	}

	if len(remoteSecrets) == 0 {
		//TODO
		return nil, nil
	}

	matchingRemoteSecret := remoteSecrets[0]
	for _, rs := range remoteSecrets {
		accessibleRepositories := rs.Annotations[api.RSServiceProviderRepositoryAnnotation]
		//TODO fix url match
		if slices.Contains(commaseparated.Value(accessibleRepositories).Values(), matchable.RepoUrl()) {
			matchingRemoteSecret = rs
			break
		}
	}

	return &matchingRemoteSecret, nil
}
func (r RemoteSecretCredentialsSource) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {

	matchingRemoteSecret, err := r.LookupCredentialSource(ctx, cl, matchable)
	if err != nil {
		return nil, fmt.Errorf("unable to find Secret created by RemoteSecret: %w", err)
	}
	targetIndex := getLocalNamespaceTargetIndex(matchingRemoteSecret.Status.Targets, matchable.ObjNamespace())
	if targetIndex < 0 || targetIndex >= len(matchingRemoteSecret.Status.Targets) {
		return nil, missingTargetError // Should not happen, but avoids panicking just in case.
	}
	secret := &v1.Secret{}
	err = cl.Get(ctx, client.ObjectKey{Namespace: matchable.ObjNamespace(), Name: matchingRemoteSecret.Status.Targets[targetIndex].SecretName}, secret)
	if err != nil {
		return nil, fmt.Errorf("unable to find Secret created by RemoteSecret: %w", err)
	}

	// Create a new owner ref.
	gvk, err := apiutil.GVKForObject(matchingRemoteSecret, scheme.Scheme)
	if err != nil {
		return nil, err
	}
	ref := &metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        matchingRemoteSecret.GetUID(),
		Name:       matchingRemoteSecret.GetName(),
	}

	return &Credentials{Username: string(secret.Data[v1.BasicAuthUsernameKey]), Password: string(secret.Data[v1.BasicAuthPasswordKey]), Owner: ref}, nil
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
