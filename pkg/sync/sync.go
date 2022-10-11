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
	"fmt"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Syncer synchronized K8s objects with the cluster
type Syncer struct {
	client client.Client
}

func New(client client.Client) Syncer {
	return Syncer{client: client}
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

	key := client.ObjectKeyFromObject(blueprint)
	if err = s.client.Get(ctx, key, actual); err != nil {
		if !errors.IsNotFound(err) {
			lg.Error(err, "failed to read object to be synced", "ObjectKey", key)
			return false, nil, fmt.Errorf("error getting the object %+v: %w", key, err)
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

	actual, err := s.newWithSameKind(object)
	if err != nil {
		lg.Error(err, "failed to create an empty object with GVK", "GVK", object.GetObjectKind().GroupVersionKind())
		return err
	}

	key := client.ObjectKeyFromObject(object)
	if err = s.client.Get(ctx, key, actual); err == nil {
		err = s.client.Delete(ctx, actual)
	}

	if err != nil && !errors.IsNotFound(err) {
		lg.Error(err, "failed to delete object", "ObjectKey", client.ObjectKeyFromObject(object))
		return fmt.Errorf("error deleting the object %+v: %w", key, err)
	}

	return nil
}

func (s *Syncer) newWithSameKind(blueprint client.Object) (client.Object, error) {
	gvk := blueprint.GetObjectKind().GroupVersionKind()
	o, err := s.client.Scheme().New(gvk)
	if err != nil {
		return nil, fmt.Errorf("error constructing new object with GVK %+v: %w", gvk, err)
	}

	o.(client.Object).GetObjectKind().SetGroupVersionKind(blueprint.GetObjectKind().GroupVersionKind())
	return o.(client.Object), nil
}

func (s *Syncer) create(ctx context.Context, owner client.Object, blueprint client.Object) (client.Object, error) {
	lg := log.FromContext(ctx)

	actual := blueprint.DeepCopyObject().(client.Object)

	objectKey := client.ObjectKeyFromObject(blueprint)

	var err error
	if owner != nil {
		err = controllerutil.SetControllerReference(owner, actual, s.client.Scheme())
		if err != nil {
			lg.Error(err, "failed to set owner reference", "Owner", client.ObjectKeyFromObject(owner), "Object", objectKey)
			return nil, fmt.Errorf("error while setting the owner reference to %+v: %w", objectKey, err)
		}
	}

	err = s.client.Create(ctx, actual)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			lg.Error(err, "failed to create object", "Object", objectKey)
			return nil, fmt.Errorf("error while creating new object %+v with GVK %+v: %w", objectKey, blueprint.GetObjectKind().GroupVersionKind(), err)
		}

		// ok, we got an already-exists error. So let's try to load the object into "actual".
		// if we fail this retry for whatever reason, just give up rather than retrying this in a loop...
		// the reconciliation loop will lead us here again in the next round.
		if err = s.client.Get(ctx, client.ObjectKeyFromObject(actual), actual); err != nil {
			lg.Error(err, "failed to read object that we failed to create", "Object", objectKey)
			return nil, fmt.Errorf("error while reading the current state of the object %+v: %w", objectKey, err)
		}
	}

	// set the type meta again, because it disappears after client creates the object
	actual.GetObjectKind().SetGroupVersionKind(blueprint.GetObjectKind().GroupVersionKind())

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

		actualKey := client.ObjectKeyFromObject(actual)

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			err := s.client.Delete(ctx, actual)
			if err != nil {
				lg.Error(err, "failed to delete object before re-creating it", "Object", actualKey)
				return false, actual, fmt.Errorf("error while deleting the object %+v to recreate it: %w", actualKey, err)
			}

			obj, err := s.create(ctx, owner, blueprint)
			if err != nil {
				lg.Error(err, "failed to create object", "Object", client.ObjectKeyFromObject(actual))
			}
			return false, obj, err
		} else {
			blueprintKey := client.ObjectKeyFromObject(blueprint)
			if owner != nil {
				err := controllerutil.SetControllerReference(owner, blueprint, s.client.Scheme())
				if err != nil {
					lg.Error(err, "failed to set owner reference", "Owner", client.ObjectKeyFromObject(owner), "Object", blueprintKey)
					return false, actual, fmt.Errorf("error while setting controller reference of %+v before update: %w", blueprintKey, err)
				}
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.SetResourceVersion(actual.GetResourceVersion())

			err := s.client.Update(ctx, blueprint)
			if err != nil {
				lg.Error(err, "failed to update object", "Object", blueprintKey)
				return false, actual, fmt.Errorf("error while updating the object %+v: %w", blueprintKey, err)
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
