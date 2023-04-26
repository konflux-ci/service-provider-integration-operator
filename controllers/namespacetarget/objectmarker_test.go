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

package namespacetarget

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var LinkAnnotation = "nein"
var ManagedByBindingLabel = "nein"

func TestNamespaceObjectMarker_IsManagedBy(t *testing.T) {
	m := NamespaceObjectMarker{}

	t.Run("not marked at all", func(t *testing.T) {
		obj := corev1.ConfigMap{}
		res, err := m.IsManagedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("just referenced", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/k",
				},
			},
		}
		res, err := m.IsManagedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("managed by other", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation:      "ns/k,ns/l",
					ManagingRemoteSecretNameAnnotation: "ns/l",
				},
			},
		}
		res, err := m.IsManagedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("managed", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation:      "ns/k",
					ManagingRemoteSecretNameAnnotation: "ns/k",
				},
			},
		}
		res, err := m.IsManagedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("managed by ours referenced by other in addition", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation:      "ns/k,ns/l",
					ManagingRemoteSecretNameAnnotation: "ns/k",
				},
			},
		}
		res, err := m.IsManagedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.True(t, res)
	})
}

func TestNamespaceObjectMarker_IsReferencedBy(t *testing.T) {
	m := NamespaceObjectMarker{}

	t.Run("not marked at all", func(t *testing.T) {
		obj := corev1.ConfigMap{}
		res, err := m.IsReferencedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("referenced", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/k",
				},
			},
		}
		res, err := m.IsReferencedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("referenced by other in addition", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/k,ns/l",
				},
			},
		}
		res, err := m.IsReferencedBy(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.True(t, res)
	})
}

func TestNamespaceObjectMarker_ListManagedOptions(t *testing.T) {
	m := NamespaceObjectMarker{}
	opts, err := m.ListManagedOptions(context.TODO(), client.ObjectKey{Name: "k"})
	assert.NoError(t, err)

	assert.Len(t, opts, 1)
	opt := opts[0]

	lopts := client.ListOptions{}
	opt.ApplyToList(&lopts)

	assert.True(t, lopts.LabelSelector.Matches(labels.Set{
		LinkedByRemoteSecretLabel: "true",
	}))
}

func TestNamespaceObjectMarker_ListReferencedOptions(t *testing.T) {
	m := NamespaceObjectMarker{}
	opts, err := m.ListReferencedOptions(context.TODO(), client.ObjectKey{Name: "k"})
	assert.NoError(t, err)

	assert.Len(t, opts, 1)
	opt := opts[0]

	lopts := client.ListOptions{}
	opt.ApplyToList(&lopts)

	assert.True(t, lopts.LabelSelector.Matches(labels.Set{
		LinkedByRemoteSecretLabel: "true",
	}))
}

func TestMarkManaged(t *testing.T) {
	m := NamespaceObjectMarker{}

	t.Run("mark empty", func(t *testing.T) {
		obj := corev1.ConfigMap{}
		changed, err := m.MarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[ManagingRemoteSecretNameAnnotation])
		assert.Equal(t, "ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 2)
		assert.True(t, changed)
	})

	t.Run("mark marked", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					ManagingRemoteSecretNameAnnotation: "ns/k",
					LinkedRemoteSecretsAnnotation:      "ns/k",
				},
			},
		}
		changed, err := m.MarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[ManagingRemoteSecretNameAnnotation])
		assert.Equal(t, "ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 2)
		assert.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("mark referenced", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/k",
				},
			},
		}
		changed, err := m.MarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[ManagingRemoteSecretNameAnnotation])
		assert.Equal(t, "ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 2)
		assert.True(t, changed)
	})

	t.Run("mark referenced by other", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/l",
				},
			},
		}
		changed, err := m.MarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[ManagingRemoteSecretNameAnnotation])
		assert.Equal(t, "ns/l,ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 2)
		assert.True(t, changed)
	})
}

func TestNamespaceObjectMarker_MarkReferenced(t *testing.T) {
	m := NamespaceObjectMarker{}

	t.Run("not marked at all", func(t *testing.T) {
		obj := corev1.ConfigMap{}
		res, err := m.MarkReferenced(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 1)
		assert.True(t, res)
	})

	t.Run("referenced", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/k",
				},
			},
		}
		res, err := m.MarkReferenced(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 1)
		assert.False(t, res)
	})

	t.Run("referenced by other", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/l",
				},
			},
		}
		res, err := m.MarkReferenced(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/l,ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 1)
		assert.True(t, res)
	})
}

func TestNamespaceObjectMarker_UnmarkManaged(t *testing.T) {
	m := NamespaceObjectMarker{}

	t.Run("unmark empty", func(t *testing.T) {
		obj := corev1.ConfigMap{}
		changed, err := m.UnmarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.Empty(t, obj.Labels)
		assert.Empty(t, obj.Annotations)
		assert.False(t, changed)
	})

	t.Run("unmark managed", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					ManagingRemoteSecretNameAnnotation: "ns/k",
					LinkedRemoteSecretsAnnotation:      "ns/k",
				},
			},
		}
		changed, err := m.UnmarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 1)
		assert.True(t, changed)
	})

	t.Run("unmark with other references present", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					ManagingRemoteSecretNameAnnotation: "ns/k",
					LinkedRemoteSecretsAnnotation:      "ns/l,ns/k",
				},
			},
		}
		changed, err := m.UnmarkManaged(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/l,ns/k", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 1)
		assert.True(t, changed)
	})
}

func TestNamespaceObjectMarker_UnmarkReferenced(t *testing.T) {
	m := NamespaceObjectMarker{}

	t.Run("not marked at all", func(t *testing.T) {
		obj := corev1.ConfigMap{}
		res, err := m.UnmarkReferenced(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.Empty(t, obj.Labels)
		assert.Empty(t, obj.Annotations)
		assert.False(t, res)
	})

	t.Run("referenced", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/k",
				},
			},
		}
		res, err := m.UnmarkReferenced(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.Empty(t, obj.Labels)
		assert.Empty(t, obj.Annotations)
		assert.True(t, res)
	})

	t.Run("referenced by other in addition", func(t *testing.T) {
		obj := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					LinkedByRemoteSecretLabel: "true",
				},
				Annotations: map[string]string{
					LinkedRemoteSecretsAnnotation: "ns/l,ns/k,ns/m",
				},
			},
		}
		res, err := m.UnmarkReferenced(context.TODO(), client.ObjectKey{Name: "k", Namespace: "ns"}, &obj)

		assert.NoError(t, err)
		assert.Equal(t, "true", obj.Labels[LinkedByRemoteSecretLabel])
		assert.Equal(t, "ns/l,ns/m", obj.Annotations[LinkedRemoteSecretsAnnotation])
		assert.Len(t, obj.Labels, 1)
		assert.Len(t, obj.Annotations, 1)
		assert.True(t, res)
	})
}

func TestNamespaceObjectMarker_GetReferencingTargets(t *testing.T) {
	m := NamespaceObjectMarker{}

	obj := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Labels: map[string]string{
				LinkedByRemoteSecretLabel: "true",
			},
			Annotations: map[string]string{
				LinkedRemoteSecretsAnnotation: "ns/l,ns/k,ns/m",
			},
		},
	}
	res, err := m.GetReferencingTargets(context.TODO(), &obj)
	assert.NoError(t, err)
	assert.Len(t, res, 3)
	assert.Equal(t, "l", res[0].Name)
	assert.Equal(t, "ns", res[0].Namespace)
	assert.Equal(t, "k", res[1].Name)
	assert.Equal(t, "ns", res[1].Namespace)
	assert.Equal(t, "m", res[2].Name)
	assert.Equal(t, "ns", res[2].Namespace)
}
