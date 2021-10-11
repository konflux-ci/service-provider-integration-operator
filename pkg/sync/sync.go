//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package sync

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Syncer synchronized K8s objects with the cluster
type Syncer struct {
	client client.Client
	scheme *runtime.Scheme
}

func New(client client.Client, scheme *runtime.Scheme) Syncer {
	return Syncer{client: client, scheme: scheme}
}

// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
// Returns true if the object was created or updated, false if there was no change detected.
func (s *Syncer) Sync(ctx context.Context, owner client.Object, blueprint client.Object, diffOpts cmp.Option) (bool, client.Object, error) {
	lg := log.FromContext(ctx)
	actual, err := s.newWithSameKind(blueprint)
	if err != nil {
		lg.Error(err, "failed to create an empty object with GVK", "GVK", blueprint.GetObjectKind().GroupVersionKind())
		return false, nil, err
	}

	if err = s.client.Get(context.TODO(), client.ObjectKeyFromObject(blueprint), actual); err != nil {
		if !errors.IsNotFound(err) {
			lg.Error(err, "failed to read object to be synced", "ObjectKey", client.ObjectKeyFromObject(blueprint))
			return false, nil, err
		}
		actual = nil
	}

	if actual == nil {
		actual, err := s.create(ctx, owner, blueprint)
		if err != nil {
			return false, actual, err
		}

		return true, actual, nil
	}

	return s.update(ctx, owner, actual, blueprint, diffOpts)
}

// Delete deletes the supplied object from the cluster.
func (s *Syncer) Delete(ctx context.Context, object client.Object) error {
	lg := log.FromContext(ctx)
	var err error

	o, err := s.scheme.New(object.GetObjectKind().GroupVersionKind())
	if err != nil {
		lg.Error(err, "failed to create an empty object with GVK", "GVK", object.GetObjectKind().GroupVersionKind())
		return err
	}

	actual := o.(client.Object)

	if err = s.client.Get(ctx, client.ObjectKeyFromObject(object), actual); err == nil {
		err = s.client.Delete(ctx, actual)
	}

	if err != nil && !errors.IsNotFound(err) {
		lg.Error(err, "failed to delete object", "ObjectKey", client.ObjectKeyFromObject(object))
		return err
	}

	return nil
}

func (s *Syncer) newWithSameKind(blueprint client.Object) (client.Object, error) {
	o, err := s.scheme.New(blueprint.GetObjectKind().GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return o.(client.Object), nil
}

func (s *Syncer) create(ctx context.Context, owner client.Object, blueprint client.Object) (client.Object, error) {
	lg := log.FromContext(ctx)

	actual := blueprint.DeepCopyObject().(client.Object)

	err := controllerutil.SetControllerReference(owner, actual, s.scheme)
	if err != nil {
		lg.Error(err, "failed to set owner reference", "Owner", client.ObjectKeyFromObject(owner), "Object", client.ObjectKeyFromObject(blueprint))
		return nil, err
	}

	err = s.client.Create(ctx, actual)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			lg.Error(err, "failed to create object", "Object", client.ObjectKeyFromObject(blueprint))
			return nil, err
		}

		// ok, we got an already-exists error. So let's try to load the object into "actual".
		// if we fail this retry for whatever reason, just give up rather than retrying this in a loop...
		// the reconciliation loop will lead us here again in the next round.
		if err = s.client.Get(ctx, client.ObjectKeyFromObject(actual), actual); err != nil {
			lg.Error(err, "failed to read object that we failed to create", "Object", client.ObjectKeyFromObject(blueprint))
			return nil, err
		}
	}

	return actual, nil
}

func (s *Syncer) update(ctx context.Context, owner client.Object, actual client.Object, blueprint client.Object, diffOpts cmp.Option) (bool, client.Object, error) {
	lg := log.FromContext(ctx)
	diff := cmp.Diff(actual, blueprint, diffOpts)
	if len(diff) > 0 {
		// we need to handle labels and annotations specially in case the cluster admin has modified them.
		// if the current object in the cluster has the same annos/labels, they get overwritten with what's
		// in the blueprint. Any additional labels/annos on the object are kept though.
		targetLabels := map[string]string{}
		targetAnnos := map[string]string{}

		for k, v := range actual.GetAnnotations() {
			targetAnnos[k] = v
		}
		for k, v := range actual.GetLabels() {
			targetLabels[k] = v
		}

		for k, v := range blueprint.GetAnnotations() {
			targetAnnos[k] = v
		}
		for k, v := range blueprint.GetLabels() {
			targetLabels[k] = v
		}

		blueprint.SetAnnotations(targetAnnos)
		blueprint.SetLabels(targetLabels)

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			err := s.client.Delete(ctx, actual)
			if err != nil {
				lg.Error(err, "failed to delete object before re-creating it", "Object", client.ObjectKeyFromObject(actual))
				return false, actual, err
			}

			obj, err := s.create(ctx, owner, blueprint)
			if err != nil {
				lg.Error(err, "failed to create object", "Object", client.ObjectKeyFromObject(actual))
			}
			return false, obj, err
		} else {
			err := controllerutil.SetControllerReference(owner, blueprint, s.scheme)
			if err != nil {
				lg.Error(err, "failed to set owner reference", "Owner", client.ObjectKeyFromObject(owner), "Object", client.ObjectKeyFromObject(blueprint))
				return false, actual, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.SetResourceVersion(actual.GetResourceVersion())

			err = s.client.Update(ctx, blueprint)
			if err != nil {
				lg.Error(err, "failed to update object", "Object", client.ObjectKeyFromObject(blueprint))
				return false, actual, err
			}

			return true, blueprint, nil
		}
	}
	return false, actual, nil
}

func isUpdateUsingDeleteCreate(kind string) bool {
	// Routes are not able to update the host, so we just need to re-create them...
	// ingresses and services have been identified to needs this, too, for reasons that I don't know..
	return kind == "Service" || kind == "Ingress" || kind == "Route"
}
