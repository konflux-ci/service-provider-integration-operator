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

package clientfactory

import (
	"context"
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errClientInstanceNotSet = errors.New("client instance does not set")
)

// SingleInstanceClientFactory client instance holding factory impl, non-public and used in tests only
type singleInstanceClientFactory struct {
	client client.Client `json:"client,omitempty"`
}

func (c singleInstanceClientFactory) CreateClient(_ context.Context) (client.Client, error) {
	if c.client != nil {
		return c.client, nil
	} else {
		return nil, errClientInstanceNotSet
	}
}
