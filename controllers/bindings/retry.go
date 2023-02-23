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
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// updateWithRetries uses a function that obtains the object to try and update in the cluster and does so repeatedly if the update fails (each time asking the function to get the object to update). If the function
// returns nil, the procedure is interrupted and considered a success (because we can't but succeed when trying to update nothing).
// `retryOnConflictMessage` is a debug log message to log before a retry, nonConflictErrorMessage is an error message to use if the update yielded other error types than a conflict (a `: %w` is automatically added to it).
// The function returns an error obtained from the last retry or nil if it succeeded.
func updateWithRetries(retries uint64, ctx context.Context, cl client.Client, objectToUpdate func() (client.Object, error), retryOnConflictMessage string, nonConflictErrorMessage string) error {
	debugLog := log.FromContext(ctx).V(logs.DebugLevel)

	err := backoff.Retry(func() error {
		obj, err := objectToUpdate()
		if err != nil {
			return err
		}
		if obj == nil {
			return nil
		}

		if err = cl.Update(ctx, obj); err != nil {
			if errors.IsConflict(err) {
				// try to reload the object. If that fails, we have to give up...
				if gerr := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj); gerr == nil {
					debugLog.Info(retryOnConflictMessage, "obj", client.ObjectKeyFromObject(obj))
					return err //nolint:wrapcheck // this will cause a retry or bubble up as a returned error from Retry...
				} else {
					debugLog.Error(gerr, "failed to re-get the object during update retry", "obj", client.ObjectKeyFromObject(obj))
					// fall-through to the return of the permanent error.
				}
			}

			// permanent error interrupts the Retry even if there are still some attempts left.
			return backoff.Permanent(fmt.Errorf("%s: %w", nonConflictErrorMessage, err)) //nolint:wrapcheck // This is an "indication error" to the Backoff framework that is not exposed further.
		}

		return nil
	}, backoff.WithContext(backoff.WithMaxRetries(backoff.NewExponentialBackOff(), retries), ctx))

	if err != nil {
		return fmt.Errorf("failed to update the object after %d retries: %w", retries, err)
	}

	return nil
}
