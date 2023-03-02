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

package bindings

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateWithRetries(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	newCl := func(objs ...client.Object) FakeClient {
		return FakeClient{
			Client: fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build(),
		}
	}

	t.Run("nil attempt is success", func(t *testing.T) {
		cl := newCl()
		assert.NoError(t, updateWithRetries(1, context.TODO(), &cl, func() (client.Object, error) {
			return nil, nil
		}, "", ""))
	})
	t.Run("err is retry", func(t *testing.T) {
		attempt := 0
		cl := newCl()
		assert.NoError(t, updateWithRetries(3, context.TODO(), &cl, func() (client.Object, error) {
			defer func() {
				attempt += 1
			}()

			if attempt == 0 {
				return nil, errors.New("retry, pretty pls")
			} else {
				return nil, nil
			}
		}, "", ""))

		assert.Equal(t, 2, attempt)
	})
	t.Run("conflict in update is retry", func(t *testing.T) {
		attempt := 0
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "s",
				Namespace: "default",
			},
		}
		cl := newCl(s)

		assert.NoError(t, updateWithRetries(3, context.TODO(), &cl, func() (client.Object, error) {
			defer func() {
				attempt += 1
			}()

			if attempt == 0 {
				// this is the first attempt to GET our object to update.
				// We take advantage of this and set up the update that should happen to fail.
				cl.UpdateError = &FakeUpdateError{
					Reason: metav1.StatusReasonConflict,
				}
			} else {
				// on all other but the first attempt, the update should succeed
				cl.UpdateError = nil
			}

			return s, nil
		}, "", ""))

		assert.Equal(t, 2, attempt)
	})
	t.Run("error to update is error", func(t *testing.T) {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "s",
				Namespace: "default",
			},
		}
		cl := newCl(s)

		assert.Error(t, updateWithRetries(3, context.TODO(), &cl, func() (client.Object, error) {
			// this is the first attempt to GET our object to update.
			// We take advantage of this and set up the update that should happen to fail.
			cl.UpdateError = &FakeUpdateError{
				Reason: metav1.StatusReasonForbidden,
			}

			return s, nil
		}, "", ""))
	})
	t.Run("error to reget is error", func(t *testing.T) {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "s",
				Namespace: "default",
			},
		}
		cl := newCl(s)

		assert.Error(t, updateWithRetries(3, context.TODO(), &cl, func() (client.Object, error) {
			// this is the first attempt to GET our object to update.
			// We take advantage of this and set up the update used for retry that should happen to fail.
			cl.UpdateError = &FakeUpdateError{
				Reason: metav1.StatusReasonConflict,
			}

			// we also update the GET that happens after the conflicting update to fail, so that we trigger
			// the unrecoverable error.
			cl.GetError = &FakeUpdateError{
				Reason: metav1.StatusReasonForbidden,
			}

			return s, nil
		}, "", ""))
	})
	t.Run("maxing out retries is error", func(t *testing.T) {
		attempt := 0
		cl := newCl()

		assert.Error(t, updateWithRetries(3, context.TODO(), &cl, func() (client.Object, error) {
			defer func() {
				attempt += 1
			}()

			return nil, errors.New("i can't construct the object to update, sorry")
		}, "", ""))

		assert.Equal(t, 4, attempt)
	})
}

type FakeClient struct {
	client.Client

	GetError    error
	UpdateError error
}

func (c *FakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.GetError != nil {
		return c.GetError
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *FakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.UpdateError != nil {
		return c.UpdateError
	}

	return c.Client.Update(ctx, obj, opts...)
}

var _ (client.Client) = (*FakeClient)(nil)

type FakeUpdateError struct {
	Reason metav1.StatusReason
}

func (c *FakeUpdateError) Status() metav1.Status {
	return metav1.Status{
		Reason: c.Reason,
	}
}

func (*FakeUpdateError) Error() string {
	return "fake update conflict"
}

var _ (apierrors.APIStatus) = (*FakeUpdateError)(nil)
var _ (error) = (*FakeUpdateError)(nil)
