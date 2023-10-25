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

package controllers

import (
	"errors"
	"reflect"

	"github.com/redhat-appstudio/remote-secret/pkg/logs"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var enqueueLog = log.Log.WithName("enqueue-logger")
var dataUpdateObjectExpected = errors.New("expected an SPIAccessTokenDataUpdate object")

func logReconciliationRequests(requests []ctrl.Request, requestKind string, cause client.Object, objectKind string) {
	if len(requests) > 0 {
		enqueueLog.V(logs.DebugLevel).Info("enqueueing reconciliation",
			"cause", client.ObjectKeyFromObject(cause),
			"causeKind", objectKind,
			"requests", requests,
			"requestKind", requestKind)
	}
}

// requestForDataUpdateOwner returns an array of reconcile requests. This array has at most 1 element that is the owner of the
// supplied data update object with the provided kind from the SPI Kubernetes API group.
func requestForDataUpdateOwner(dataUpdateObject client.Object, ownerKind string, logRequest bool) []reconcile.Request {
	update, ok := dataUpdateObject.(*api.SPIAccessTokenDataUpdate)
	if !ok {
		enqueueLog.Error(dataUpdateObjectExpected, dataUpdateObjectExpected.Error(), "actualType", reflect.TypeOf(dataUpdateObject).Name())
		return []reconcile.Request{}
	}

	if update.Spec.DataOwner.APIGroup == nil || *update.Spec.DataOwner.APIGroup != api.GroupVersion.Group || update.Spec.DataOwner.Kind != ownerKind {
		return []reconcile.Request{}
	}

	reqs := []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: dataUpdateObject.GetNamespace(),
				Name:      update.Spec.DataOwner.Name,
			},
		},
	}

	if logRequest {
		logReconciliationRequests(reqs, ownerKind, dataUpdateObject, "SPIAccessTokenDataUpdate")
	}
	return reqs
}
