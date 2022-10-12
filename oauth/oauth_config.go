package oauth

import (
	"context"
	"fmt"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	secretLabel             = "service-provider-integration/service-provider-config"
	secretFieldClientId     = "clientId"
	secretFieldClientSecret = "clientSecret"
	secretFieldAuthUrl      = "authUrl"
	secretFieldTokenUrl     = "tokenUrl"
)

func (c *commonController) obtainOauthConfig(ctx context.Context, info *oauthstate.OAuthInfo) (*oauth2.Config, error) {
	lg := log.FromContext(ctx).WithValues("oauthInfo", info)
	found, oauthCfgSecret, findErr := c.findOauthConfigSecret(ctx, info)
	if findErr != nil {
		return nil, findErr
	}

	if found {
		if oauthCfg, createOauthCfgErr := c.createConfigFromSecret(oauthCfgSecret); createOauthCfgErr == nil {
			lg.V(logs.DebugLevel).Info("using custom user oauth config")
			return oauthCfg, nil
		} else {
			return nil, fmt.Errorf("failed to create oauth config from the secret: %w", createOauthCfgErr)
		}
	} else {
		lg.V(logs.DebugLevel).Info("using default oauth config")
		return &oauth2.Config{
			ClientID:     c.Config.ClientId,
			ClientSecret: c.Config.ClientSecret,
			Endpoint:     c.Endpoint,
			RedirectURL:  c.redirectUrl(),
		}, nil
	}
}

func (c *commonController) findOauthConfigSecret(ctx context.Context, info *oauthstate.OAuthInfo) (bool, *corev1.Secret, error) {
	lg := log.FromContext(ctx).WithValues("oauthInfo", info)

	secrets := &corev1.SecretList{}
	if listErr := c.K8sClient.List(ctx, secrets, client.InNamespace(info.TokenNamespace), client.MatchingLabels{
		secretLabel: string(c.Config.ServiceProviderType),
	}); listErr != nil {
		return false, nil, fmt.Errorf("failed to list oauth config secrets: %w", listErr)
	}

	lg.V(logs.DebugLevel).Info("found secrets with oauth configuration", "count", len(secrets.Items))
	if len(secrets.Items) == 1 {
		return true, &secrets.Items[0], nil
	} else {
		return false, nil, nil
	}
}

func (c *commonController) createConfigFromSecret(secret *corev1.Secret) (*oauth2.Config, error) {
	oauthCfg := &oauth2.Config{
		Endpoint:    c.Endpoint,
		RedirectURL: c.redirectUrl(),
	}
	if clientId, has := secret.Data[secretFieldClientId]; has {
		oauthCfg.ClientID = string(clientId)
	} else {
		return nil, fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientId'", secret.Namespace, secret.Name)
	}

	if clientSecret, has := secret.Data[secretFieldClientSecret]; has {
		oauthCfg.ClientSecret = string(clientSecret)
	} else {
		return nil, fmt.Errorf("failed to create oauth config from the secret '%s/%s', missing 'clientSecret'", secret.Namespace, secret.Name)
	}

	if authUrl, has := secret.Data[secretFieldAuthUrl]; has {
		oauthCfg.Endpoint.AuthURL = string(authUrl)
	}

	if tokenUrl, has := secret.Data[secretFieldTokenUrl]; has {
		oauthCfg.Endpoint.AuthURL = string(tokenUrl)
	}

	return oauthCfg, nil
}
