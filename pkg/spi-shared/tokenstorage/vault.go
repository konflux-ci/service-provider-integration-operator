package tokenstorage

import (
	"context"
	"encoding/json"
	"fmt"

	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const vaultDataPathFormat = "spi/data/%s/%s"

type vaultTokenStorage struct {
	*vault.Client
}

// NewVaultStorage creates a new `TokenStorage` instance using the provided Vault instance.
func NewVaultStorage(k8sClient k8s.Client, role string, vaultHost string, serviceAccountToken string) (TokenStorage, error) {
	config := vault.DefaultConfig()
	config.Address = vaultHost
	vaultClient, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}
	var k8sAuth *auth.KubernetesAuth
	if serviceAccountToken == "" {
		k8sAuth, err = auth.NewKubernetesAuth(role)
	} else {
		k8sAuth, err = auth.NewKubernetesAuth(role, auth.WithServiceAccountTokenPath(serviceAccountToken))
	}
	if err != nil {
		return nil, err
	}

	authInfo, err := vaultClient.Auth().Login(context.TODO(), k8sAuth)
	if err != nil {
		return nil, err
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login to vault")
	}
	return &NotifyingTokenStorage{
		Client:       k8sClient,
		TokenStorage: &vaultTokenStorage{vaultClient},
	}, nil
}

func (v *vaultTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) error {
	data := map[string]interface{}{
		"data": token,
	}
	path := getVaultPath(owner)
	s, err := v.Client.Logical().Write(path, data)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("failed to store the token, no error but returned nil")
	}
	for _, w := range s.Warnings {
		logf.FromContext(ctx).Info(w)
	}

	return nil
}

func (v *vaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	path := getVaultPath(owner)

	secret, err := v.Client.Logical().Read(path)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil || len(secret.Data) == 0 || secret.Data["data"] == nil {
		logf.FromContext(ctx).Info("no data found in vault at", "path", path)
		return nil, nil
	}
	for _, w := range secret.Warnings {
		logf.FromContext(ctx).Info(w)
	}
	data, dataOk := secret.Data["data"]
	if !dataOk {
		return nil, fmt.Errorf("corrupted data in Vault at '%s'", path)
	}

	return parseToken(ctx, data)
}

func parseToken(ctx context.Context, data interface{}) (*api.Token, error) {
	// We only get `interface{}` from Vault. In order to get `Token{}` struct,
	// we first marshall interface into JSON, and then unmarshall it back to `Token{}` struct.
	jsonData, err := json.Marshal(data)
	if err != nil {
		logf.FromContext(ctx).Error(err, "failed to marshall token data back")
		return nil, err
	}

	token := &api.Token{}
	err = json.Unmarshal(jsonData, token)
	if err != nil {
		logf.FromContext(ctx).Error(err, "failed to unmarshall databack to token")
		return nil, err
	}
	return token, nil
}

func (v *vaultTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	s, err := v.Client.Logical().Delete(getVaultPath(owner))
	if err != nil {
		return err
	}
	logf.FromContext(ctx).Info("deleted", "secret", s)
	return nil
}

func getVaultPath(owner *api.SPIAccessToken) string {
	return fmt.Sprintf(vaultDataPathFormat, owner.Namespace, owner.Name)
}
