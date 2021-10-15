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

package sync

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	utilruntime.Must(corev1.AddToScheme(scheme))
}

func TestSyncCreates(t *testing.T) {

	preexisting := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
		},
	}

	new := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new",
			Namespace: "default",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(preexisting).Build()

	syncer := Syncer{client: cl, scheme: scheme}

	_, _, err := syncer.Sync(context.TODO(), preexisting, new, cmp.Options{})
	assert.NoError(t, err)

	synced := &corev1.Pod{}
	key := client.ObjectKey{Name: "new", Namespace: "default"}

	assert.NoError(t, cl.Get(context.TODO(), key, synced))

	assert.Equal(t, "new", synced.Name, "The synced object should have the expected name")
	assert.Len(t, synced.OwnerReferences, 1, "There should have been an owner reference set")
	assert.Equal(t, "preexisting", synced.OwnerReferences[0].Name, "Unexpected owner reference")
}

func TestSyncUpdates(t *testing.T) {
	preexisting := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name: "preexisting",
					Kind: "Pod",
				},
			},
		},
	}

	newOwner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "newOwner",
			Namespace: "default",
		},
	}

	update := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			Labels: map[string]string{
				"a": "b",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(preexisting).Build()

	syncer := Syncer{client: cl, scheme: scheme}

	_, _, err := syncer.Sync(context.TODO(), newOwner, update, cmp.Options{})
	assert.NoError(t, err)

	synced := &corev1.Pod{}
	key := client.ObjectKey{Name: "preexisting", Namespace: "default"}

	assert.NoError(t, cl.Get(context.TODO(), key, synced))

	assert.Equal(t, "preexisting", synced.Name, "The synced object should have the expected name")
	assert.Len(t, synced.OwnerReferences, 1, "There should have been an owner reference set")
	assert.Equal(t, "newOwner", synced.OwnerReferences[0].Name, "Unexpected owner reference")
	assert.NotEmpty(t, synced.GetLabels(), "There should have been labels on the synced object")
	assert.Equal(t, "b", synced.GetLabels()["a"], "Unexpected label")
}

func TestSyncKeepsAdditionalAnnosAndLabels(t *testing.T) {
	preexisting := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			Labels: map[string]string{
				"a": "x",
				"k": "v",
			},
			Annotations: map[string]string{
				"a": "x",
				"k": "v",
			},
		},
	}

	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "default",
		},
	}

	update := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			Labels: map[string]string{
				"a": "b",
				"c": "d",
			},
			Annotations: map[string]string{
				"a": "b",
				"c": "d",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(preexisting).Build()

	syncer := Syncer{client: cl, scheme: scheme}

	_, _, err := syncer.Sync(context.TODO(), owner, update, cmp.Options{})
	assert.NoError(t, err)

	synced := &corev1.Pod{}
	key := client.ObjectKey{Name: "preexisting", Namespace: "default"}

	assert.NoError(t, cl.Get(context.TODO(), key, synced))

	assert.Equal(t, "preexisting", synced.Name, "The synced object should have the expected name")

	expectedValues := map[string]string{
		"a": "b",
		"k": "v",
		"c": "d",
	}

	assert.Equal(t, expectedValues, synced.Labels, "Unexpected labels on the synced object")
	assert.Equal(t, expectedValues, synced.Annotations, "Unexpected annotations on the synced object")
}
