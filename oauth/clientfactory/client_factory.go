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
	"fmt"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClientFactory interface {
	CreateClient(ctx context.Context) (client.Client, error)
}

func doCreateClient(cfg *rest.Config, opts client.Options) (client.Client, error) {
	cl, err := client.New(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create a kubernetes client: %w", err)
	}
	return cl, nil
}
