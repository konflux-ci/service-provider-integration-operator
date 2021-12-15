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

package storage

import (
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Storage struct {
	// let's fake it for now
	_s map[client.ObjectKey]api.Token
}

func (v *Storage) Store(owner client.Object, token *api.Token) (string, error) {
	v.storage()[client.ObjectKeyFromObject(owner)] = *token
	return v.GetDataLocation(owner)
}

func (v *Storage) Get(owner client.Object) (*api.Token, error) {
	key := client.ObjectKeyFromObject(owner)
	val, ok := v.storage()[key]
	if !ok {
		return nil, nil
	}
	return val.DeepCopy(), nil
}

func (v *Storage) GetDataLocation(owner client.Object) (string, error) {
	return "/spi/" + owner.GetNamespace() + "/" + owner.GetName(), nil
}

func (v *Storage) Delete(owner client.Object) error {
	delete(v.storage(), client.ObjectKeyFromObject(owner))
	return nil
}

func (v *Storage) storage() map[client.ObjectKey]api.Token {
	if v._s == nil {
		v._s = map[client.ObjectKey]api.Token{}
	}

	return v._s
}
