package tokenstorage

import (
	"context"
	"encoding/json"
	"fmt"

	vault "github.com/hashicorp/vault/api"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const vaultDataPathFormat = "spi/data/%s/%s"

var log = logf.Log.WithName("vault")

type vaultTokenStorage struct {
	*vault.Client
}

func (v *vaultTokenStorage) Store(ctx context.Context, owner *api.SPIAccessToken, token *api.Token) (string, error) {
	tokenJson, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	path := getVaultPath(owner)
	s, err := v.Client.Logical().WriteBytes(path, tokenJson)
	if err != nil {
		return "", err
	}
	for _, w := range s.Warnings {
		log.Info(w)
	}

	return path, nil
}

func (v *vaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	path := getVaultPath(owner)

	secret, err := v.Client.Logical().Read(path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, nil
	}
	for _, w := range secret.Warnings {
		log.Info(w)
	}
	if secret.Data == nil || len(secret.Data) == 0 {
		return nil, fmt.Errorf("no data found in Vault at '%s'", path)
	}
	data, dataOk := secret.Data["data"]
	if !dataOk {
		return nil, fmt.Errorf("corrupted data in Vault at '%s'", path)
	}

	return parseToken(data)
}

func parseToken(data interface{}) (*api.Token, error) {
	// We only get `interface{}` from Vault. In order to get `Token{}` struct,
	// we first marshall interface into JSON, and then unmarshall it back to `Token{}` struct.
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Error(err, "failed to marshall token data back")
		return nil, err
	}

	token := &api.Token{}
	err = json.Unmarshal(jsonData, token)
	if err != nil {
		log.Error(err, "failed to unmarshall databack to token")
		return nil, err
	}
	return token, nil
}

func (v *vaultTokenStorage) GetDataLocation(ctx context.Context, owner *api.SPIAccessToken) (string, error) {
	return getVaultPath(owner), nil
}

func (v *vaultTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	s, err := v.Client.Logical().Delete(getVaultPath(owner))
	if err != nil {
		return err
	}
	log.Info("deleted", "secret", s)
	return nil
}

func getVaultPath(owner *api.SPIAccessToken) string {
	name := owner.Name
	if name == "" {
		name = owner.GenerateName
	}
	return fmt.Sprintf(vaultDataPathFormat, owner.Namespace, name)
}
