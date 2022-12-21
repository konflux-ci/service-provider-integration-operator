package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	corev1 "k8s.io/api/core/v1"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var errMultipleMatchingSecrets = errors.New("found multiple matching oauth config secrets")

func FindServiceProviderConfigSecret(ctx context.Context, k8sClient client.Reader, tokenNamespace string, spHost string, spType ServiceProviderType) (bool, *corev1.Secret, error) {
	lg := log.FromContext(ctx).WithValues("spHost", spHost)

	secrets := &corev1.SecretList{}
	if listErr := k8sClient.List(ctx, secrets, client.InNamespace(tokenNamespace), client.MatchingLabels{
		v1beta1.ServiceProviderTypeLabel: string(spType),
	}); listErr != nil {
		if kuberrors.IsForbidden(listErr) {
			lg.Info("not enough permissions to list the secrets")
			return false, nil, nil
		} else {
			return false, nil, fmt.Errorf("failed to list oauth config secrets: %w", listErr)
		}
	}

	lg.V(logs.DebugLevel).Info("found secrets with oauth configuration", "count", len(secrets.Items))

	if len(secrets.Items) < 1 {
		return false, nil, nil
	}

	var oauthSecretWithoutHost *corev1.Secret
	var oauthSecretWithHost *corev1.Secret

	// go through all found oauth secret configs
	for _, oauthSecret := range secrets.Items {
		// if we find one labeled for sp host, we take it
		if labelSpHost, hasLabel := oauthSecret.ObjectMeta.Labels[v1beta1.ServiceProviderHostLabel]; hasLabel {
			if labelSpHost == spHost {
				if oauthSecretWithHost != nil { // if we found one before, return error because we can't tell which one to use
					return false, nil, errMultipleMatchingSecrets
				}
				oauthSecretWithHost = oauthSecret.DeepCopy()
			}
		} else { // if we found one without host label, we save it for later
			if oauthSecretWithoutHost != nil { // if we found one before, return error because we can't tell which one to use
				return false, nil, errMultipleMatchingSecrets
			}
			oauthSecretWithoutHost = oauthSecret.DeepCopy()
		}
	}

	if oauthSecretWithHost != nil {
		return true, oauthSecretWithHost, nil
	} else if oauthSecretWithoutHost != nil {
		return true, oauthSecretWithoutHost, nil
	} else {
		return false, nil, nil
	}

}
