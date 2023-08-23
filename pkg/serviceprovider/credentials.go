package serviceprovider

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Credentials struct {
	Username string
	Password string
	//TODO Questionable!!!
	Owner *metav1.OwnerReference
}

type CredentialsSource interface {
	//TODO think about notfound error or nil
	LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error)
}

type SpiTokenCredentialsSource struct {
}

func (s SpiTokenCredentialsSource) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	//TODO implement me
	panic("implement me")
}

type RemoteSecretCredentialsSource struct {
}

func (r RemoteSecretCredentialsSource) LookupCredentials(ctx context.Context, cl client.Client, matchable Matchable) (*Credentials, error) {
	//TODO implement me
	panic("implement me")
}

/*


// LookupRemoteSecrets searches for RemoteSecrets with RSServiceProviderHostLabel in the same namespaces matchable.
// These RemoteSecrets are then filtered using GenericLookup's RemoteSecretFilter.
func (l GenericLookup) LookupRemoteSecrets(ctx context.Context, cl client.Client, matchable Matchable) ([]v1beta1.RemoteSecret, error) {
	lg := log.FromContext(ctx)

	repoHost, err := l.RepoHostParser(matchable.RepoUrl())
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

	matches := make([]v1beta1.RemoteSecret, 0)
	// For now let's just do a linear search. In the future we can think about go func like in Lookup.
	for i := range potentialMatches.Items {
		if l.RemoteSecretFilter == nil || l.RemoteSecretFilter.Matches(ctx, matchable, &potentialMatches.Items[i]) {
			matches = append(matches, potentialMatches.Items[i])
		}
	}

	return matches, nil
}

func (l GenericLookup) LookupRemoteSecretSecret(ctx context.Context, cl client.Client, matchable Matchable, remoteSecrets []v1beta1.RemoteSecret, repoName string) (*v1beta1.RemoteSecret, *v1.Secret, error) {
	if len(remoteSecrets) == 0 {
		return nil, nil, nil
	}

	matchingRemoteSecret := remoteSecrets[0]
	for _, rs := range remoteSecrets {
		accessibleRepositories := rs.Annotations[api.RSServiceProviderRepositoryAnnotation]
		if slices.Contains(commaseparated.Value(accessibleRepositories).Values(), repoName) {
			matchingRemoteSecret = rs
			break
		}
	}

	targetIndex := getLocalNamespaceTargetIndex(matchingRemoteSecret.Status.Targets, matchable.ObjNamespace())
	if targetIndex < 0 || targetIndex >= len(matchingRemoteSecret.Status.Targets) {
		return nil, nil, missingTargetError // Should not happen, but avoids panicking just in case.
	}

	secret := &v1.Secret{}
	err := cl.Get(ctx, client.ObjectKey{Namespace: matchable.ObjNamespace(), Name: matchingRemoteSecret.Status.Targets[targetIndex].SecretName}, secret)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to find Secret created by RemoteSecret: %w", err)
	}

	return &matchingRemoteSecret, secret, nil
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
*/
