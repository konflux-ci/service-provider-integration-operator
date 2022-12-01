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

package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCreateClient(t *testing.T) {
	// the custom mapper is there to avoid the dynamic mapper used by the client by default. This is so that the
	// dynamic mapper doesn't try to discover the API that is just not there...
	cl, err := CreateClient(&rest.Config{}, client.Options{
		Mapper: meta.NewDefaultRESTMapper([]schema.GroupVersion{}),
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, cl.Scheme().AllKnownTypes())
}
