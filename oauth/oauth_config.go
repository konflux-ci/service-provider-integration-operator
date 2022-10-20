package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	kuberrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	oauthCfgSecretLabel             = "service-provider-integration/service-provider-config"
	oauthCfgSecretFieldClientId     = "clientId"
	oauthCfgSecretFieldClientSecret = "clientSecret"
	oauthCfgSecretFieldAuthUrl      = "authUrl"
	oauthCfgSecretFieldTokenUrl     = "tokenUrl"
)

var (
	missingFieldError = errors.New("missing mandatory field in oauth configuration")
)

// obtainOauthConfig is responsible for getting oauth configuration of service provider.
// Currently, this can be configured with labeled secret living in namespace together with SPIAccessToken.
// If no such secret is found, global configuration of oauth service is used.
func (c *commonController) obtainOauthConfig(ctx context.Context, info *oauthstate.OAuthInfo) (*oauth2.Config, error) {
	lg := log.FromContext(ctx).WithValues("oauthInfo", info)

	oauthCfg := &oauth2.Config{
		Endpoint:    c.Endpoint,
		RedirectURL: c.redirectUrl(),
	}

	found, oauthCfgSecret, findErr := c.findOauthConfigSecret(ctx, info)
	if findErr != nil {
		return nil, findErr
	}

	if found {
		if createOauthCfgErr := createConfigFromSecret(oauthCfgSecret, oauthCfg); createOauthCfgErr == nil {
			lg.V(logs.DebugLevel).Info("using custom user oauth config")
			return oauthCfg, nil
		} else {
			return nil, fmt.Errorf("failed to create oauth config from the secret: %w", createOauthCfgErr)
		}
	} else {
		lg.V(logs.DebugLevel).Info("using default oauth config")
		oauthCfg.ClientID = c.Config.ClientId
		oauthCfg.ClientSecret = c.Config.ClientSecret
		return oauthCfg, nil
	}
}

func (c *commonController) findOauthConfigSecret(ctx context.Context, info *oauthstate.OAuthInfo) (bool, *corev1.Secret, error) {
	lg := log.FromContext(ctx).WithValues("oauthInfo", info)

	secrets := &corev1.SecretList{}
	if listErr := c.K8sClient.List(ctx, secrets, client.InNamespace(info.TokenNamespace), client.MatchingLabels{
		oauthCfgSecretLabel: string(c.Config.ServiceProviderType),
	}); listErr != nil {
		if kuberrors.IsForbidden(listErr) {
			lg.Info("user is not able to read secrets")
			return false, nil, nil
		} else {
			return false, nil, fmt.Errorf("failed to list oauth config secrets: %w", listErr)
		}
	}

	lg.V(logs.DebugLevel).Info("found secrets with oauth configuration", "count", len(secrets.Items))
	if len(secrets.Items) == 1 {
		return true, &secrets.Items[0], nil
	} else {
		return false, nil, nil
	}
}

func createConfigFromSecret(secret *corev1.Secret, oauthCfg *oauth2.Config) error {
	if clientId, has := secret.Data[oauthCfgSecretFieldClientId]; has {
		oauthCfg.ClientID = string(clientId)
	} else {
		return fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientId': %w", secret.Namespace, secret.Name, missingFieldError)
	}

	if clientSecret, has := secret.Data[oauthCfgSecretFieldClientSecret]; has {
		oauthCfg.ClientSecret = string(clientSecret)
	} else {
		return fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientSecret': %w", secret.Namespace, secret.Name, missingFieldError)
	}

	if authUrl, has := secret.Data[oauthCfgSecretFieldAuthUrl]; has {
		oauthCfg.Endpoint.AuthURL = string(authUrl)
	}

	if tokenUrl, has := secret.Data[oauthCfgSecretFieldTokenUrl]; has {
		oauthCfg.Endpoint.TokenURL = string(tokenUrl)
	}

	return nil
}
