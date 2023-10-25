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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAugmentConfiguration(t *testing.T) {
	cfg := rest.Config{}
	AugmentConfiguration(&cfg)

	assert.NotNil(t, cfg.AuthProvider)
	assert.Equal(t, authPluginName, cfg.AuthProvider.Name)
}

func TestWithAuthFromRequest(t *testing.T) {
	requestPerformed := false

	cfg := rest.Config{
		Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
			requestPerformed = true
			auth := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer kachny", auth)

			// because the client expects a protobuf message as a response and I can't be bothered to create one here
			// let's just return a 404. We don't care about the return value anyway
			return &http.Response{
				StatusCode: 404,
				Header:     http.Header{},
				Request:    r,
			}, nil
		}),
		Host: "over-the-rainbow",
	}

	AugmentConfiguration(&cfg)

	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	restMapper.Add(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}, meta.RESTScopeNamespace)

	cl, err := client.New(&cfg, client.Options{
		Scheme: scheme,
		Mapper: restMapper,
	})
	assert.NoError(t, err)

	req := &http.Request{
		Header: map[string][]string{"Authorization": {"Bearer kachny"}},
	}

	ctx, err := WithAuthFromRequestIntoContext(req, req.Context())
	assert.NoError(t, err)

	obj := corev1.ConfigMap{}
	_ = cl.Get(ctx, client.ObjectKey{Name: "name", Namespace: "ns"}, &obj)
	assert.True(t, requestPerformed)
}

// fakeRoundTrip casts a function into a http.RoundTripper
type fakeRoundTrip func(r *http.Request) (*http.Response, error)

var _ http.RoundTripper = fakeRoundTrip(nil)

func (f fakeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
