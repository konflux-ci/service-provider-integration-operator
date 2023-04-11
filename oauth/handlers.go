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
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-jose/go-jose/v3/json"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OkHandler is a Handler implementation that responds only with http.StatusOK.
// Typically, used for liveness and readiness probes
func OkHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// CallbackSuccessHandler is a Handler implementation that responds with HTML page
// This page is a landing page after successfully completing the OAuth flow.
// Resource file location is prefixed with `../` to be compatible with tests running locally.
func CallbackSuccessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../static/callback_success.html")
	})
}

// viewData structure is used to pass parameters during callback_error.html template processing.
type viewData struct {
	Title   string
	Message string
}

// CallbackErrorHandler is a Handler implementation that responds with HTML page
// This page is a landing page after unsuccessfully completing the OAuth flow.
// Resource file location is prefixed with `../` to be compatible with tests running locally.
func CallbackErrorHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		errorMsg := q.Get("error")
		errorDescription := q.Get("error_description")
		data := viewData{
			Title:   errorMsg,
			Message: errorDescription,
		}
		logs.AuditLog(r.Context()).Info("OAuth authentication flow failed.", "message", errorMsg, "description", errorDescription)
		tmpl, _ := template.ParseFiles("../static/callback_error.html")

		err := tmpl.Execute(w, data)
		if err == nil {
			w.WriteHeader(http.StatusOK)
		} else {
			log.FromContext(r.Context()).Error(err, "failed to process template")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("Error response returned to OAuth callback: %s. Message: %s ", errorMsg, errorDescription)))
		}
	})
}

// HandleUpload returns Handler implementation that is relied on provided TokenUploader to persist provided credentials
// for some concrete SPIAccessToken.
func HandleUpload(uploader TokenUploader) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, err := clientfactory.WithAuthFromRequestIntoContext(r, r.Context())
		if err != nil {
			LogErrorAndWriteResponse(r.Context(), w, http.StatusUnauthorized, "failed extract authorization information from headers", err)
			return
		}

		vars := mux.Vars(r)
		tokenObjectName := vars["name"]
		tokenObjectNamespace := vars["namespace"]

		if len(tokenObjectName) < 1 || len(tokenObjectNamespace) < 1 {
			LogDebugAndWriteResponse(r.Context(), w, http.StatusInternalServerError, "Incorrect service deployment. Token name and namespace can't be omitted or empty.")
			return
		}

		// We want to check token names are satisfying the general label value format,
		// since the SPI access token names are used as a label values in the linked objects.
		errs := validation.IsValidLabelValue(tokenObjectName)
		if len(errs) > 0 {
			LogDebugAndWriteResponse(r.Context(), w, http.StatusBadRequest, "Incorrect token name parameter. Must comply with common label value format. Details: "+strings.Join(errs, ";"))
			return
		}

		// Namespaces are required to be a valid RFC 1123 DNS labels
		errs = validation.IsDNS1123Label(tokenObjectNamespace)
		if len(errs) > 0 {
			LogDebugAndWriteResponse(r.Context(), w, http.StatusBadRequest, "Incorrect token namespace parameter. Must comply with RFC 1123 label format. Details: "+strings.Join(errs, ";"))
			return
		}

		data := &api.Token{}
		if err := json.NewDecoder(r.Body).Decode(data); err != nil {
			LogErrorAndWriteResponse(r.Context(), w, http.StatusBadRequest, "failed to decode request body as token JSON", err)
			return
		}

		if data.AccessToken == "" {
			LogDebugAndWriteResponse(r.Context(), w, http.StatusBadRequest, "access token can't be omitted or empty")
			return
		}

		if err := uploader.Upload(ctx, tokenObjectName, tokenObjectNamespace, data); err != nil {
			switch errors.ReasonForError(err) {
			case v1.StatusReasonNotFound:
				LogErrorAndWriteResponse(r.Context(), w, http.StatusNotFound, "specified SPIAccessToken does not exist", err)
			case v1.StatusReasonForbidden:
				LogErrorAndWriteResponse(r.Context(), w, http.StatusForbidden, "authorization token does not have permission to update SPIAccessToken", err)
			case v1.StatusReasonUnauthorized:
				LogErrorAndWriteResponse(r.Context(), w, http.StatusUnauthorized, "invalid authorization token, cannot update SPIAccessToken", err)
			default:
				LogErrorAndWriteResponse(r.Context(), w, http.StatusInternalServerError, "failed to upload the token", err)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// CSPHandler is a Handler that writes into response a CSP headers allowing inline styles, images from redhat domain, and denying everything else, including framing
func CSPHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src https://*.redhat.com; frame-ancestors 'none';")
		h.ServeHTTP(w, r)
	})
}

// BypassHandler is a Handler that redirects a request that has URL with certain prefix to a bypassHandler
// all remaining requests are redirected to mainHandler.
func BypassHandler(mainHandler http.Handler, bypassPathPrefixes []string, bypassHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, bypassPath := range bypassPathPrefixes {
			if strings.HasPrefix(r.URL.Path, bypassPath) {
				bypassHandler.ServeHTTP(w, r)
				return
			}
		}
		mainHandler.ServeHTTP(w, r)
	})
}

// MiddlewareHandler is a Handler that composed couple of different responsibilities.
// Like:
// - Service metrics
// - Request logging
// - CORS processing
func MiddlewareHandler(reg prometheus.Registerer, allowedOrigins []string, h http.Handler) http.Handler {

	middlewareHandler := HttpServiceInstrumentMetricHandler(reg,
		handlers.LoggingHandler(&zapio.Writer{Log: zap.L(), Level: zap.DebugLevel},
			handlers.CORS(handlers.AllowedOrigins(allowedOrigins),
				handlers.AllowCredentials(),
				handlers.AllowedHeaders([]string{
					"Accept",
					"Accept-Language",
					"Content-Language",
					"Origin",
					"Authorization"}))(h)))

	return BypassHandler(middlewareHandler, []string{"/health", "/ready"}, h)
}
