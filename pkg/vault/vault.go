package vault

import (
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Vault struct {
	// let's fake it for now
	_s map[client.ObjectKey]api.Token
}

func (v *Vault) Store(owner client.Object, token *api.Token) error {
	v.storage()[client.ObjectKeyFromObject(owner)] = *token
	return nil
}

func (v *Vault) Get(owner client.Object) (*api.Token, error) {
	key := client.ObjectKeyFromObject(owner)
	val, ok := v.storage()[key]
	if !ok {
		return nil, nil
	}
	return val.DeepCopy(), nil
}

func (v *Vault) GetDataLocation(owner client.Object) (string, error) {
	return ("/spi/" + owner.GetNamespace() + "/" + owner.GetName()), nil
}

func (v *Vault) Delete(owner client.Object) error {
	delete(v.storage(), client.ObjectKeyFromObject(owner))
	return nil
}

func (v *Vault) storage() map[client.ObjectKey]api.Token {
	if v._s == nil {
		v._s = map[client.ObjectKey]api.Token{}
	}

	return v._s
}
