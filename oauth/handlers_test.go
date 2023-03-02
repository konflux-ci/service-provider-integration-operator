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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"

	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gorilla/mux"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logs.InitDevelLoggers()
	os.Exit(m.Run())
}

func TestOkHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(OkHandler)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCSPHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := CSPHandler(http.HandlerFunc(OkHandler))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	//Check header value ok
	assert.Equal(t, rr.Header().Get("Content-Security-Policy"), "default-src 'none'; style-src 'unsafe-inline'; img-src https://*.redhat.com; frame-ancestors 'none';")
}

func TestReadyCheckHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/ready", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(OkHandler)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCallbackSuccessHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/callback_success", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := CallbackSuccessHandler()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCallbackErrorHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", oauth.CallBackRoutePath+"?error=foo&error_description=bar", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := CallbackErrorHandler()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestUploaderOk(t *testing.T) {

	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Equal(t, "umbrella", tokenObjectName)
		assert.Equal(t, "jdoe", tokenObjectNamespace)
		assert.Equal(t, "42", data.AccessToken)
		assert.Empty(t, data.Username)
		return nil
	})

	req, err := http.NewRequest("POST", "/token/jdoe/umbrella", bytes.NewBuffer([]byte(`{"access_token": "42"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNoContent)
	}
}

func TestUploader_FailWithEmptyToken(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Fail(t, "This line should not be reached")
		return nil
	})

	req, err := http.NewRequest("POST", "/token/jdoe/umbrella", bytes.NewBuffer([]byte(`{"username": "jdoe"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	w := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "access token can't be omitted or empty" {
		t.Errorf("expected 'access token can't be omitted or empty' got '%v'", string(data))
	}
}

func TestUploader_FailWithProperResponse(t *testing.T) {
	uploaderNotFound := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Equal(t, "john", tokenObjectName)
		assert.Equal(t, "namespace", tokenObjectNamespace)
		return fmt.Errorf("mocking a missing SPIAccessToken: %w", errors.NewNotFound(schema.GroupResource{
			Group:    "testGroup",
			Resource: "testSPIAccessToken",
		}, tokenObjectName))
	})

	uploaderForbidden := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		return fmt.Errorf("mocking a token with forbidden access: %w", errors.NewForbidden(schema.GroupResource{
			Group:    "testGroup",
			Resource: "testSPIAccessToken",
		}, tokenObjectName, fmt.Errorf("unauthorized")))
	})

	uploaderUnauthorized := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		return fmt.Errorf("mocking an invalid token: %w", errors.NewUnauthorized("not a valid token"))
	})

	uploaderInternal := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		return fmt.Errorf("mocking internal unrelated error")
	})

	testResponse := func(uploader UploadFunc, statusCode int, errorMsg string) {
		req, err := http.NewRequest("POST", "/token/namespace/john", bytes.NewBuffer([]byte(`{"access_token": "2022"}`)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer macky")

		w := httptest.NewRecorder()
		var router = mux.NewRouter()
		router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

		router.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()

		assert.Equal(t, statusCode, res.StatusCode)

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, string(data), errorMsg)
	}
	testResponse(uploaderNotFound, http.StatusNotFound, "mocking a missing SPIAccessToken")
	testResponse(uploaderForbidden, http.StatusForbidden, "mocking a token with forbidden access")
	testResponse(uploaderUnauthorized, http.StatusUnauthorized, "mocking an invalid token")
	testResponse(uploaderInternal, http.StatusInternalServerError, "mocking internal unrelated error")
}

func TestUploader_FailWithoutAuthorization(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Fail(t, "This line should not be reached")
		return nil
	})

	req, err := http.NewRequest("POST", "/token/jdoe/umbrella", bytes.NewBuffer([]byte(`{"access_token": "42"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Del("Authorization")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	w := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusUnauthorized)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var expected = "failed extract authorization information from headers: no bearer token found"
	if string(data) != expected {
		t.Errorf("expected '"+expected+"' got '%v'", string(data))
	}
}

func TestUploader_FailJsonParse(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Fail(t, "This line should not be reached")
		return nil
	})

	req, err := http.NewRequest("POST", "/token/jdoe/umbrella", bytes.NewBuffer([]byte(`this is not a json`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var expected = "failed to decode request body as token JSON: invalid character 'h' in literal true (expecting 'r')"
	if string(data) != expected {
		t.Errorf("expected '"+expected+"' got '%v'", string(data))
	}
}

func TestUploader_FailNamespaceParamValidation(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Fail(t, "This line should not be reached")
		return nil
	})

	req, err := http.NewRequest("POST", "/token/jdoe:baudot/umbrella", bytes.NewBuffer([]byte(`{"access_token": "42"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var expected = "Incorrect token namespace parameter. Must comply with RFC 1123 label format. Details: a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')"
	if string(data) != expected {
		t.Errorf("expected '"+expected+"' got '%v'", string(data))
	}
}

func TestUploader_FailTokenNameParamValidation(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {
		assert.Fail(t, "This line should not be reached")
		return nil
	})

	req, err := http.NewRequest("POST", "/token/jdoe/umbrella:bad", bytes.NewBuffer([]byte(`{"access_token": "42"}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var expected = "Incorrect token name parameter. Must comply with common label value format. Details: a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"
	if string(data) != expected {
		t.Errorf("expected '"+expected+"' got '%v'", string(data))
	}
}

func TestUploader_FailUploaderError(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {

		return fmt.Errorf("failed to store the token data into storage")
	})

	req, err := http.NewRequest("POST", "/token/jdoe/umbrella", bytes.NewBuffer([]byte(`{"access_token": "42"}`)))

	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token/{namespace}/{name}").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var expected = "failed to upload the token: failed to store the token data into storage"
	if string(data) != expected {
		t.Errorf("expected '"+expected+"' got '%v'", string(data))
	}
}

func TestUploader_FailIncorrectHandlerConfiguration(t *testing.T) {
	uploader := UploadFunc(func(ctx context.Context, tokenObjectName string, tokenObjectNamespace string, data *api.Token) error {

		return fmt.Errorf("failed to store the token data into storage")
	})

	req, err := http.NewRequest("POST", "/token", bytes.NewBuffer([]byte(`{"access_token": "42"}`)))

	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer kachny")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	var router = mux.NewRouter()
	router.NewRoute().Path("/token").HandlerFunc(HandleUpload(uploader)).Methods("POST")

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	router.ServeHTTP(rr, req)
	res := rr.Result()
	defer res.Body.Close()
	// Check the status code is what we expect.

	if status := res.StatusCode; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var expected = "Incorrect service deployment. Token name and namespace can't be omitted or empty."
	if string(data) != expected {
		t.Errorf("expected '"+expected+"' got '%v'", string(data))
	}
}

func TestMiddlewareHandlerCorsPart(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", oauth.CallBackRoutePath+"?error=foo&error_description=bar", nil)
	req.Header.Set("Origin", "https://prod.foo.redhat.com")
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := MiddlewareHandler(prometheus.NewRegistry(), []string{"https://console.dev.redhat.com", "https://prod.foo.redhat.com"}, http.HandlerFunc(OkHandler))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the status code is what we expect.
	if allowOrigin := rr.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "https://prod.foo.redhat.com" {
		t.Errorf("handler returned wrong header \"Access-Control-Allow-Origin\": got %v want %v",
			allowOrigin, "prod.foo.redhat.com")
	}

}

func TestMiddlewareHandlerCors(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("OPTIONS", oauth.AuthenticateRoutePath+"?state=eyJhbGciO", nil)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "c")
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Origin", "https://file-retriever-server-service-spi-system.apps.cluster-flmv6.flmv6.sandbox1324.opentlc.com")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://file-retriever-server-service-spi-system.apps.cluster-flmv6.flmv6.sandbox1324.opentlc.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := MiddlewareHandler(prometheus.NewRegistry(), []string{"https://file-retriever-server-service-spi-system.apps.cluster-flmv6.flmv6.sandbox1324.opentlc.com", "http:://acme.com"}, http.HandlerFunc(OkHandler))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the status code is what we expect.
	if allowOrigin := rr.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "https://file-retriever-server-service-spi-system.apps.cluster-flmv6.flmv6.sandbox1324.opentlc.com" {
		t.Errorf("handler returned wrong header \"Access-Control-Allow-Origin\": got %v want %v",
			allowOrigin, "https://file-retriever-server-service-spi-system.apps.cluster-flmv6.flmv6.sandbox1324.opentlc.com")
	}

}

// Simple counter server
type Counter struct {
	mu sync.Mutex // protects n
	n  int
}

func (ctr *Counter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctr.mu.Lock()
	defer ctr.mu.Unlock()
	ctr.n++
}
func TestBypassHandlerFollowBypass(t *testing.T) {
	//given
	mainHandler := new(Counter)
	bypassHandler := new(Counter)
	testHandler := BypassHandler(mainHandler, []string{"/path1", "/path2"}, bypassHandler)
	req, err := http.NewRequest("GET", "/path2", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	//when
	testHandler.ServeHTTP(rr, req)
	//then
	assert.Equal(t, 0, mainHandler.n)
	assert.Equal(t, 1, bypassHandler.n)
}

func TestBypassHandlerNotFollowBypass(t *testing.T) {
	//given
	mainHandler := new(Counter)
	bypassHandler := new(Counter)
	testHandler := BypassHandler(mainHandler, []string{"/path1", "/path2"}, bypassHandler)
	req, err := http.NewRequest("POST", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	//when
	testHandler.ServeHTTP(rr, req)
	//then
	assert.Equal(t, 1, mainHandler.n)
	assert.Equal(t, 0, bypassHandler.n)
}
