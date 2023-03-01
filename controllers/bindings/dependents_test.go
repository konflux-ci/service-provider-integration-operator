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
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/memorystorage"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDependentsSync(t *testing.T) {
	// Dependents.Sync just trivially calls serviceAccountsHandler.Sync, secretHandler.Sync
	// and serviceAccountsHandler.LinkToSecret.
	//
	// Not sure what and how to test there given the other three methods are covered by
	// unit tests.
}

func TestDependentsCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "default",
					Labels: map[string]string{
						ManagedByLabel: "binding",
					},
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sa-refed",
					Namespace: "default",
					Annotations: map[string]string{
						LinkAnnotation: "binding",
					},
				},
				Secrets: []corev1.ObjectReference{
					{
						Name: "secret",
					},
					{
						Name: "not-us",
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{
						Name: "secret",
					},
					{
						Name: "not-us",
					},
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sa-managed",
					Namespace: "default",
					Labels: map[string]string{
						ManagedByLabel: "binding",
					},
					Annotations: map[string]string{
						LinkAnnotation: "binding",
					},
				},
			},
		).
		Build()

	binding := &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding",
			Namespace: "default",
		},
	}

	h := DependentsHandler{
		Client:       cl,
		Binding:      binding,
		TokenStorage: &memorystorage.MemoryTokenStorage{},
	}

	assert.NoError(t, h.Cleanup(context.TODO()))

	s := &corev1.Secret{}
	sa := &corev1.ServiceAccount{}

	t.Run("deletes managed SAs", func(t *testing.T) {

		err := cl.Get(context.TODO(), client.ObjectKey{Name: "sa-managed", Namespace: "default"}, sa)
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("unlinks referenced SAs", func(t *testing.T) {
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "sa-refed", Namespace: "default"}, sa))
		assert.Len(t, sa.Secrets, 1)
		assert.Equal(t, "not-us", sa.Secrets[0].Name)
		assert.Len(t, sa.ImagePullSecrets, 1)
		assert.Equal(t, "not-us", sa.ImagePullSecrets[0].Name)
	})

	t.Run("deletes secrets", func(t *testing.T) {
		err := cl.Get(context.TODO(), client.ObjectKey{Name: "secret", Namespace: "default"}, s)
		assert.True(t, errors.IsNotFound(err))
	})
}

func TestDependentsRevertTo(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	// let's first define the state that should be reverted to...
	origState := func() []client.Object {
		return []client.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "default",
					Labels: map[string]string{
						ManagedByLabel: "binding",
					},
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sa-refed",
					Namespace: "default",
					Annotations: map[string]string{
						LinkAnnotation: "binding",
					},
				},
				Secrets: []corev1.ObjectReference{
					{
						Name: "secret",
					},
					{
						Name: "not-us",
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{
						Name: "secret",
					},
					{
						Name: "not-us",
					},
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sa-managed",
					Namespace: "default",
					Labels: map[string]string{
						ManagedByLabel: "binding",
					},
					Annotations: map[string]string{
						LinkAnnotation: "binding",
					},
				},
				Secrets: []corev1.ObjectReference{
					{
						Name: "secret",
					},
				},
			},
		}
	}

	newCl := func(objs ...client.Object) client.Client {
		return fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			Build()
	}

	t.Run("deletes the secret when there wasn't any originally", func(t *testing.T) {
		binding := &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding",
				Namespace: "default",
			},
		}

		// create the objects in the cluster as if the reconciler did it.
		cl := newCl(origState()...)

		h := DependentsHandler{
			Client:       cl,
			Binding:      binding,
			TokenStorage: &memorystorage.MemoryTokenStorage{},
		}

		// now create a checkpoint in the empty cluster
		cp, err := h.CheckPoint(context.TODO())
		assert.NoError(t, err)

		assert.NoError(t, h.RevertTo(context.TODO(), cp))

		// check that the cluster doesn't contain the secret and the managed service service account
		sl := &corev1.SecretList{}
		sal := &corev1.ServiceAccountList{}

		assert.NoError(t, cl.List(context.TODO(), sl))
		assert.Empty(t, sl.Items)

		assert.NoError(t, cl.List(context.TODO(), sal))
		assert.Len(t, sal.Items, 1)
		assert.Equal(t, sal.Items[0].Name, "sa-refed")
	})

	t.Run("reverts to using original link types", func(t *testing.T) {
		// set the binding as it exists before reconciliation, which reflects the origState of the objects in the cluster.
		binding := &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenBindingStatus{
				SyncedObjectRef: api.TargetObjectRef{
					Name: "secret",
				},
				ServiceAccountNames: []string{
					"sa-refed",
					"sa-managed",
				},
			},
		}

		cl := newCl(origState()...)

		h := DependentsHandler{
			Client:       cl,
			Binding:      binding,
			TokenStorage: &memorystorage.MemoryTokenStorage{},
		}

		cp, err := h.CheckPoint(context.TODO())
		assert.NoError(t, err)

		// ok, now, let's make the changes to the cluster so that we can later revert them...
		saRefed := &corev1.ServiceAccount{}
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "sa-refed", Namespace: "default"}, saRefed))
		saRefed.Secrets = []corev1.ObjectReference{
			{
				Name: "not-us",
			},
		}
		saRefed.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: "not-us",
			},
		}
		assert.NoError(t, cl.Update(context.TODO(), saRefed))

		// now we're ready to revert...
		assert.NoError(t, h.RevertTo(context.TODO(), cp))

		// load the sa-refed again and check that the changes are gone and it is back in its original state
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKey{Name: "sa-refed", Namespace: "default"}, saRefed))

		assert.Len(t, saRefed.Secrets, 2)
		assert.Len(t, saRefed.ImagePullSecrets, 2)
	})
	t.Run("deletes no longer linked managed SAs", func(t *testing.T) {
		// here, we're going to pretend that the reconciler created a new SA as a result of Sync - i.e that
		// the spec of the binding contains a new service account that was previously not mentioned in its status.
		// (i.e. the user updated the binding with a new SA link)

		// set the binding as it exists before reconciliation, which reflects the origState of the objects in the cluster.
		binding := &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenBindingStatus{
				SyncedObjectRef: api.TargetObjectRef{
					Name: "secret",
				},
				ServiceAccountNames: []string{
					"sa-refed",
					"sa-managed",
				},
			},
		}

		cl := newCl(origState()...)

		h := DependentsHandler{
			Client:       cl,
			Binding:      binding,
			TokenStorage: &memorystorage.MemoryTokenStorage{},
		}

		cp, err := h.CheckPoint(context.TODO())
		assert.NoError(t, err)

		// now let's pretend the sync created a new SA and linked it to the service account (which it then failed to update and so we're reverting)
		newSA := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sa-new",
				Namespace: "default",
				Labels: map[string]string{
					ManagedByLabel: "binding",
				},
				Annotations: map[string]string{
					LinkAnnotation: "binding",
				},
			},
			Secrets: []corev1.ObjectReference{
				{
					Name: "secret",
				},
			},
		}
		assert.NoError(t, cl.Create(context.TODO(), newSA))

		// now, let's revert...
		assert.NoError(t, h.RevertTo(context.TODO(), cp))

		// and check the SA is no longer there
		err = cl.Get(context.TODO(), client.ObjectKeyFromObject(newSA), newSA)
		assert.True(t, errors.IsNotFound(err))
	})
	t.Run("unannotates referenced SA if not linked anymore", func(t *testing.T) {
		// here, we're testing the situation where there is an attempt to link a new pre-existing SA but the update of the binding fails, so we
		// need to revert to the SA not being linked.

		// set the binding as it exists before reconciliation, which reflects the origState of the objects in the cluster.
		binding := &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding",
				Namespace: "default",
			},
			Status: api.SPIAccessTokenBindingStatus{
				SyncedObjectRef: api.TargetObjectRef{
					Name: "secret",
				},
				ServiceAccountNames: []string{
					"sa-refed",
					"sa-managed",
				},
			},
		}

		cl := newCl(origState()...)

		h := DependentsHandler{
			Client:       cl,
			Binding:      binding,
			TokenStorage: &memorystorage.MemoryTokenStorage{},
		}

		cp, err := h.CheckPoint(context.TODO())
		assert.NoError(t, err)

		// create the new SA that is newly linked to the binding's secret and is annotated as such. This is what the "reconciler" did before
		// it failed to update the binding.

		newRefedSA := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "newRefedSA",
				Namespace: "default",
				Annotations: map[string]string{
					LinkAnnotation: "binding",
				},
			},
			Secrets: []corev1.ObjectReference{
				{
					Name: "secret",
				},
			},
		}
		assert.NoError(t, cl.Create(context.TODO(), newRefedSA))

		binding.Status.ServiceAccountNames = append(binding.Status.ServiceAccountNames, "newRefedSA")

		// and now, we're ready to revert
		assert.NoError(t, h.RevertTo(context.TODO(), cp))

		// check that the new SA's are not linked anymore
		checkRefed := &corev1.ServiceAccount{}
		assert.NoError(t, cl.Get(context.TODO(), client.ObjectKeyFromObject(newRefedSA), checkRefed))

		assert.Empty(t, checkRefed.Labels)
		assert.Empty(t, checkRefed.Annotations)
	})

}
