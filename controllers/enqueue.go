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
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var enqueueLog = log.Log.WithName("enqueue-logger")

func logReconciliationRequests(requests []ctrl.Request, requestKind string, cause client.Object, objectKind string) {
	if len(requests) > 0 {
		enqueueLog.V(logs.DebugLevel).Info("enqueueing reconciliation",
			"cause", client.ObjectKeyFromObject(cause),
			"causeKind", objectKind,
			"requests", requests,
			"requestKind", requestKind)
	}
}
