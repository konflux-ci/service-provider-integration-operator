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

package oauth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func LogErrorAndWriteResponse(ctx context.Context, w http.ResponseWriter, status int, msg string, err error) {
	log := log.FromContext(ctx)
	log.Error(err, msg)
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err = fmt.Fprintf(w, "%s: %s", msg, err.Error())
	if err != nil {
		log.Error(err, "error recording response error message")
	}
}

func LogDebugAndWriteResponse(ctx context.Context, w http.ResponseWriter, status int, msg string, keysAndValues ...interface{}) {
	log := log.FromContext(ctx)
	log.V(logs.DebugLevel).Info(msg, keysAndValues...)
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := fmt.Fprint(w, msg)
	if err != nil {
		log.Error(err, "error recording response error message")
	}
}

// AuditLogWithTokenInfo logs message related to particular SPIAccessToken into audit logger
func AuditLogWithTokenInfo(ctx context.Context, msg string, namespace string, token string, keysAndValues ...interface{}) {
	keysAndValues = append(keysAndValues, "namespace", namespace, "token", token)
	logs.AuditLog(ctx).Info(msg, keysAndValues...)
}
