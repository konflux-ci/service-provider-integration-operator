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
	"errors"
	"net/http"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/alexedwards/scs/v2"
)

type Authenticator struct {
	K8sClient      AuthenticatingClient
	SessionManager *scs.SessionManager
}

var (
	noTokenFoundError = errors.New("no token associated with the given session or provided as a `k8s_token` query parameter")
)

func (a Authenticator) tokenReview(token string, req *http.Request) (bool, error) {
	//TODO not working. temporary disabled.
	//review := v1.TokenReview{
	//	Spec: v1.TokenReviewSpec{
	//		Token: token,
	//	},
	//}
	//
	//ctx := WithAuthIntoContext(token, req.Context())
	//
	//if err := a.K8sClient.Create(ctx, &review); err != nil {
	//	zap.L().Error("token review error", zap.Error(err))
	//	return false, err
	//}
	//
	//zap.L().Debug("token review result", zap.Stringer("review", &review))
	//return review.Status.Authenticated, nil
	return true, nil
}
func (a *Authenticator) GetToken(r *http.Request) (string, error) {
	lg := log.FromContext(r.Context())
	defer logs.TimeTrack(lg, time.Now(), "/GetToken")

	token := r.URL.Query().Get("k8s_token")
	if token == "" {
		token = a.SessionManager.GetString(r.Context(), "k8s_token")
	} else {
		lg.V(logs.DebugLevel).Info("persisting token that was provided by `k8_token` query parameter to the session")
		a.SessionManager.Put(r.Context(), "k8s_token", token)
	}

	if token == "" {
		return "", noTokenFoundError
	}
	return token, nil
}

func (a Authenticator) Login(w http.ResponseWriter, r *http.Request) {
	lg := log.FromContext(r.Context())
	defer logs.TimeTrack(lg, time.Now(), "/Login")

	token := r.FormValue("k8s_token")

	if token == "" {
		token = ExtractTokenFromAuthorizationHeader(r.Header.Get("Authorization"))
	}

	if token == "" {
		LogDebugAndWriteResponse(r.Context(), w, http.StatusUnauthorized, "failed extract authorization info either from headers or form parameters")
		return
	}
	hasAccess, err := a.tokenReview(token, r)
	if err != nil {
		LogErrorAndWriteResponse(r.Context(), w, http.StatusUnauthorized, "failed to determine if the authenticated user has access", err)
		lg.Error(err, "The token is incorrect or the SPI OAuth service is not configured properly "+
			"and the API_SERVER environment variable points it to the incorrect Kubernetes API server. "+
			"If SPI is running with Devsandbox Proxy or KCP, make sure this env var points to the Kubernetes API proxy,"+
			" otherwise unset this variable. See more https://github.com/redhat-appstudio/infra-deployments/pull/264")
		return
	}

	if !hasAccess {
		LogDebugAndWriteResponse(r.Context(), w, http.StatusUnauthorized, "authenticating the request in Kubernetes unsuccessful")
		AuditLog(r.Context()).Info("unsuccessful authentication with Kubernetes token occurred") //more details will be logged after real TokenReview will be in action
		return
	}

	a.SessionManager.Put(r.Context(), "k8s_token", token)
	AuditLog(r.Context()).Info("successful authentication with Kubernetes token")
	w.WriteHeader(http.StatusOK)
}

func NewAuthenticator(sessionManager *scs.SessionManager, cl AuthenticatingClient) *Authenticator {
	return &Authenticator{
		K8sClient:      cl,
		SessionManager: sessionManager,
	}
}
