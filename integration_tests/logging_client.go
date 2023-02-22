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

//go:build !release

package integrationtests

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var fakeError error = errors.New("print the callstack of successful invocation")

// LoggingKubernetesClient is a wrapper aroung a Kubernetes client that is capable of logging the calls to the
// Kubernetes API. It is meant to be used only in tests and integration tests!
type LoggingKubernetesClient struct {
	Client             client.Client
	LogReads           bool
	LogWrites          bool
	IncludeStacktraces bool
}

type loggingKubernetesSubresourceWriter struct {
	scheme             *runtime.Scheme
	writer             client.SubResourceWriter
	includeStacktraces bool
}

// Create implements client.SubResourceWriter
func (w *loggingKubernetesSubresourceWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return logAround(func() error {
		return w.writer.Create(ctx, obj, subResource, opts...) //nolint:wrapcheck
	}, ctx, w.includeStacktraces, "Status.Create",
		func() []interface{} {
			return []interface{}{
				"key", client.ObjectKeyFromObject(obj),
				"kind", getKind(w.scheme, obj),
				"subResource", client.ObjectKeyFromObject(subResource),
				"subResourceKind", getKind(w.scheme, subResource),
			}
		})
}

// Patch implements client.SubResourceWriter
func (w *loggingKubernetesSubresourceWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return logAround(func() error {
		return w.writer.Patch(ctx, obj, patch, opts...) //nolint:wrapcheck
	}, ctx, w.includeStacktraces, "Status.Patch", func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", getKind(w.scheme, obj)}
	})
}

// Update implements client.SubResourceWriter
func (w *loggingKubernetesSubresourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return logAround(func() error {
		return w.writer.Update(ctx, obj, opts...) //nolint:wrapcheck
	}, ctx, w.includeStacktraces, "Status.Update", func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", getKind(w.scheme, obj)}
	})
}

type loggingSubresourceClient struct {
	scheme             *runtime.Scheme
	client             client.SubResourceClient
	logReads           bool
	logWrites          bool
	includeStacktraces bool
	subResource        string
}

// Get implements client.SubResourceClient
func (c *loggingSubresourceClient) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	if !c.logReads {
		return c.client.Get(ctx, obj, subResource, opts...) //nolint:wrapcheck
	}

	return logAround(func() error {
		return c.client.Get(ctx, obj, subResource, opts...) //nolint:wrapcheck
	}, ctx, false, c.methodName("Get"), func() []interface{} {
		return []interface{}{
			"key", client.ObjectKeyFromObject(obj),
			"kind", getKind(c.scheme, obj),
			"subResource", client.ObjectKeyFromObject(subResource),
			"subResourceKind", getKind(c.scheme, subResource),
		}
	})
}

// Create implements client.SubResourceClient
func (c *loggingSubresourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	if !c.logWrites {
		return c.client.Create(ctx, obj, subResource, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.client.Create(ctx, obj, subResource, opts...) //nolint:wrapcheck
	}, ctx, c.includeStacktraces, c.methodName("Create"),
		func() []interface{} {
			return []interface{}{
				"key", client.ObjectKeyFromObject(obj),
				"kind", getKind(c.scheme, obj),
				"subResource", client.ObjectKeyFromObject(subResource),
				"subResourceKind", getKind(c.scheme, subResource),
			}
		})
}

// Patch implements client.SubResourceClient
func (c *loggingSubresourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if !c.logWrites {
		return c.client.Patch(ctx, obj, patch, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.client.Patch(ctx, obj, patch, opts...) //nolint:wrapcheck
	}, ctx, c.includeStacktraces, c.methodName("Patch"), func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", getKind(c.scheme, obj)}
	})
}

// Update implements client.SubResourceClient
func (c *loggingSubresourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if !c.logWrites {
		return c.client.Update(ctx, obj, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.client.Update(ctx, obj, opts...) //nolint:wrapcheck
	}, ctx, c.includeStacktraces, c.methodName("Update"), func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", getKind(c.scheme, obj)}
	})
}

func (c *loggingSubresourceClient) methodName(methodName string) string {
	return fmt.Sprintf("SubResource(%s).%s", c.subResource, methodName)
}

// Get implements client.Client
func (c *LoggingKubernetesClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if !c.LogReads {
		return c.Client.Get(ctx, key, obj, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.Get(ctx, key, obj, opts...) //nolint:wrapcheck
	}, ctx, false, "Get", func() []interface{} {
		return []interface{}{"key", key, "kind", c.getKind(obj)}
	})
}

// List implements client.Client
func (c *LoggingKubernetesClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if !c.LogReads {
		return c.Client.List(ctx, list, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.List(ctx, list, opts...) //nolint:wrapcheck
	}, ctx, false, "List", func() []interface{} {
		return []interface{}{"kind", c.getKind(list)}
	})
}

// Create implements client.Client
func (c *LoggingKubernetesClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if !c.LogWrites {
		return c.Client.Create(ctx, obj, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.Create(ctx, obj, opts...) //nolint:wrapcheck
	}, ctx, c.IncludeStacktraces, "Create", func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", c.getKind(obj)}
	})
}

// Delete implements client.Client
func (c *LoggingKubernetesClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if !c.LogWrites {
		return c.Client.Delete(ctx, obj, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.Delete(ctx, obj, opts...) //nolint:wrapcheck
	}, ctx, c.IncludeStacktraces, "Delete", func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", c.getKind(obj)}
	})
}

// DeleteAllOf implements client.Client
func (c *LoggingKubernetesClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if !c.LogWrites {
		return c.Client.DeleteAllOf(ctx, obj, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.DeleteAllOf(ctx, obj, opts...) //nolint:wrapcheck
	}, ctx, c.IncludeStacktraces, "DeleteAllOf", func() []interface{} {
		return []interface{}{"kind", c.getKind(obj)}
	})
}

// Patch implements client.Client
func (c *LoggingKubernetesClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if !c.LogWrites {
		return c.Client.Patch(ctx, obj, patch, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.Patch(ctx, obj, patch, opts...) //nolint:wrapcheck
	}, ctx, c.IncludeStacktraces, "Patch", func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", c.getKind(obj)}
	})
}

// Update implements client.Client
func (c *LoggingKubernetesClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if !c.LogWrites {
		return c.Client.Update(ctx, obj, opts...) //nolint:wrapcheck
	}
	return logAround(func() error {
		return c.Client.Update(ctx, obj, opts...) //nolint:wrapcheck
	}, ctx, c.IncludeStacktraces, "Update", func() []interface{} {
		return []interface{}{"key", client.ObjectKeyFromObject(obj), "kind", c.getKind(obj)}
	})
}

// Status implements client.Client
func (c *LoggingKubernetesClient) Status() client.SubResourceWriter {
	if !c.LogWrites {
		return c.Client.Status()
	}
	return &loggingKubernetesSubresourceWriter{
		scheme:             c.Client.Scheme(),
		writer:             c.Client.Status(),
		includeStacktraces: c.IncludeStacktraces,
	}
}

// SubResource implements client.Client
func (c *LoggingKubernetesClient) SubResource(subResource string) client.SubResourceClient {
	return &loggingSubresourceClient{
		scheme:             c.Client.Scheme(),
		client:             c.Client.SubResource(subResource),
		includeStacktraces: c.IncludeStacktraces,
		subResource:        subResource,
	}
}

// RESTMapper implements client.Client
func (c *LoggingKubernetesClient) RESTMapper() meta.RESTMapper {
	return c.Client.RESTMapper()
}

// Scheme implements client.Client
func (c *LoggingKubernetesClient) Scheme() *runtime.Scheme {
	return c.Client.Scheme()
}

var _ (client.Client) = (*LoggingKubernetesClient)(nil)

func (c *LoggingKubernetesClient) getKind(obj runtime.Object) string {
	return getKind(c.Client.Scheme(), obj)
}

func getKind(scheme *runtime.Scheme, obj runtime.Object) string {
	var kind = "<unknown>"
	kinds, _, err := scheme.ObjectKinds(obj)
	if len(kinds) > 0 && err == nil {
		kind = kinds[0].Kind
	}

	return kind
}

func logAround(fn func() error, ctx context.Context, includeStacktraces bool, methodName string, keysVals func() []interface{}) error {
	id := uuid.NewUUID()

	args := keysVals()

	lg := log.FromContext(ctx)

	lg.WithValues("inv_id", id).WithValues(args...).Info(fmt.Sprintf(">> %s", methodName))

	err := fn()

	// evaluate the arguments again so that we can gather more info in case pointers are involved
	args = keysVals()
	lg = lg.WithValues("inv_id", id).WithValues(args...)

	if err != nil || includeStacktraces {
		e := err
		if e == nil {
			e = fakeError
		}
		lg.Error(e, fmt.Sprintf("<< %s", methodName))
	} else {
		lg.Info(fmt.Sprintf("<< %s", methodName))
	}

	return err
}
