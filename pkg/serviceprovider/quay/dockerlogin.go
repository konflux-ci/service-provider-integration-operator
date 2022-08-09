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

package quay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var loginResponseWithoutTokenError = errors.New("quay login response doesn't contain a token")
var unexpectedTokenFormatError = errors.New("unexpected token format in quay login response")

// DockerLogin performs docker login to quay using the provided username and password (that might be a robot account creds) and returns
// a JWT token that can be used as a bearer token in the subsequent requests to the docker API in quay.
// `repository` is in the form of `org/name`. If the provided credentials are invalid, an empty string is returned.
// An error is returned when the attempt to parse the login response fails or any other error during the login process.
func DockerLogin(ctx context.Context, cl *http.Client, repository string, username string, password string) (string, error) {
	debugLog := log.FromContext(ctx, "repository", repository).V(logs.DebugLevel)
	debugLog.Info("attempting docker login to quay")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://quay.io/v2/auth?service=quay.io&scope=repository:"+repository+":push,pull", nil)
	if err != nil {
		debugLog.Error(err, "failed to compose the quay login request")
		return "", fmt.Errorf("failed to compose the quay login request: %w", err)
	}

	userPass := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+userPass)

	res, err := cl.Do(req)
	if err != nil {
		debugLog.Error(err, "failed to perform the quay login request")
		return "", fmt.Errorf("failed to perform the quay login request: %w", err)
	}

	if res.StatusCode != 200 {
		defer func() {
			if err := res.Body.Close(); err != nil {
				debugLog.Error(err, "failed to close response while trying to login to quay.io")
			}
		}()

		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			debugLog.Error(err, "failed to read the body of response", "statusCode", res.StatusCode)
			return "", errors.WithMessage(err, "failed to read the body of non-200 response to login attempt")
		}

		msg := string(bytes)

		debugLog.Info("quay docker login attempt unsuccessful (without error)", "status", res.StatusCode, "response", msg)
		return "", nil
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		debugLog.Error(err, "failed to read the quay login response body")
		return "", fmt.Errorf("failed to read the quay login response body: %w", err)
	}

	resp := map[string]interface{}{}
	if err := json.Unmarshal(bytes, &resp); err != nil {
		debugLog.Error(err, "failed to unmarshal quay login response to JSON")
		return "", fmt.Errorf("failed to unmarshal quay login response to JSON: %w", err)
	}

	tokenObj, ok := resp["token"]
	if !ok {
		debugLog.Info("quay login response did not contain the expected 'token' field")
		return "", loginResponseWithoutTokenError
	}

	token, ok := tokenObj.(string)
	if !ok {
		debugLog.Info("quay login response 'token' field is expected to be a string")
		return "", unexpectedTokenFormatError
	}

	debugLog.Info("quay docker login attempt successful")

	return token, nil
}

// LoginTokenInfo is the output of the AnalyzeLoginToken function describing the information extracted from the JWT
// token obtained after a successful docker login from the DockerLogin function.
type LoginTokenInfo struct {
	Username     string
	Repositories map[string]LoginTokenRepositoryInfo
}

// LoginTokenRepositoryInfo represents the capabilities mentioned in the JWT docker login token for a certain
// repository.
type LoginTokenRepositoryInfo struct {
	Pushable bool
	Pullable bool
}

// quayClaims is the representation of the JWT docker login token returned from the docker login to Quay. This is only
// used internally by AnalyzeLoginToken to deserialize the token.
type quayClaims struct {
	jwt.RegisteredClaims
	Access []struct {
		Type    string   `json:"type"`
		Name    string   `json:"name"`
		Actions []string `json:"actions"`
	} `json:"access"`
	Context struct {
		Version         int               `json:"version"`
		EntityKind      string            `json:"entity_kind"`
		EntityReference string            `json:"entity_reference"`
		Kind            string            `json:"kind"`
		User            string            `json:"user"`
		OAuth           string            `json:"oauth"`
		ApostilleRoots  map[string]string `json:"com.apostille.roots"`
		ApostilleRoot   string            `json:"com.apostille.root"`
	} `json:"context"`
}

// AnalyzeLoginToken analyzes the JWT token obtained from the DockerLogin function to figure out the capabilities of
// token obtained for some repository.
func AnalyzeLoginToken(token string) (LoginTokenInfo, error) {
	tkn, _, err := jwt.NewParser().ParseUnverified(token, &quayClaims{})
	if err != nil {
		return LoginTokenInfo{}, fmt.Errorf("failed to parse Quay JWT token: %w", err)
	}

	claims := tkn.Claims.(*quayClaims)
	ret := LoginTokenInfo{}
	ret.Username = claims.Context.User
	ret.Repositories = make(map[string]LoginTokenRepositoryInfo, len(claims.Access))
	for _, access := range claims.Access {
		ret.Repositories[access.Name] = LoginTokenRepositoryInfo{
			Pushable: containsString(access.Actions, "push"),
			Pullable: containsString(access.Actions, "pull"),
		}
	}

	return ret, nil
}

func containsString(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}

	return false
}
