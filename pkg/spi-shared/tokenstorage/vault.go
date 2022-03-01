package tokenstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const baseVaultPath = "spi/data/spiaccesstoken/"

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

	return getVaultDataLocation(path), nil
}

func (v *vaultTokenStorage) Get(ctx context.Context, owner *api.SPIAccessToken) (*api.Token, error) {
	path, err := getVaultPathFromLocation(owner)
	if err != nil {
		return nil, err
	}

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
	if _, ok := secret.Data["data"]; !ok {
		return nil, fmt.Errorf("corrupted data in Vault at '%s'", path)
	}

	jsonData, err := json.Marshal(secret.Data["data"])
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
	token, err := v.Get(ctx, owner)
	if err != nil {
		log.Error(err, "not found?")
		return "", err
	} else {
		log.Info("found", "token", token)
		return getVaultPathFromLocation(owner)
	}
}

func (v *vaultTokenStorage) Delete(ctx context.Context, owner *api.SPIAccessToken) error {
	s, err := v.Client.Logical().Delete(getVaultPath(owner))
	if err != nil {
		return err
	}
	log.Info("deleted", "secret", s)
	return nil
}

func getVaultDataLocation(path string) string {
	return "vault:" + path
}

func getVaultPath(owner *api.SPIAccessToken) string {
	name := owner.Name
	if name == "" {
		name = owner.GenerateName
	}
	return baseVaultPath + name
}

func getVaultPathFromLocation(owner *api.SPIAccessToken) (string, error) {
	if owner.Spec.DataLocation == "" {
		return "", nil
	}
	dataLoc := strings.Split(owner.Spec.DataLocation, ":")
	if len(dataLoc) != 2 || dataLoc[0] != "vault" {
		return "", fmt.Errorf("unknown format of data location '%s'", owner.Spec.DataLocation)
	}

	return dataLoc[1], nil
}
