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

package errors

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ServiceProviderHttpError struct {
	StatusCode int
	Response   string
}

func (e ServiceProviderHttpError) Error() string {
	var identification string
	if e.StatusCode >= 400 && e.StatusCode < 500 {
		identification = "invalid access token"
	} else if e.StatusCode > 500 && e.StatusCode < 600 {
		identification = "error in the service provider"
	} else {
		identification = "unknown service provider error"
	}

	return fmt.Sprintf("%s (http status %d): %s", identification, e.StatusCode, e.Response)
}

func IsServiceProviderHttpError(err error) bool {
	spe := &ServiceProviderHttpError{}
	return errors.As(err, &spe)
}

func IsServiceProviderHttpInvalidAccessToken(err error) bool {
	spe := &ServiceProviderHttpError{}
	if !errors.As(err, &spe) {
		return false
	}

	return spe.StatusCode >= 400 && spe.StatusCode < 500
}

func IsServiceProviderHttpInternalServerError(err error) bool {
	spe := &ServiceProviderHttpError{}
	if !errors.As(err, &spe) {
		return false
	}

	return spe.StatusCode >= 500 && spe.StatusCode < 600
}

// FromHttpResponse returns a non-nil error if the provided response has a status code >= 400 and < 600 (i.e. auth and
// internal server errors). Note that the body of the response is consumed when the erroneous status code is detected so
// that it is preserved in the returned error object.
func FromHttpResponse(response *http.Response) error {
	if response.StatusCode >= 400 && response.StatusCode < 600 {
		var body string
		bytes, err := io.ReadAll(response.Body)
		if err != nil {
			body = fmt.Sprintf("failed to read the error response from the service provider: %s", err.Error())
		} else {
			body = string(bytes)
		}

		return &ServiceProviderHttpError{
			StatusCode: response.StatusCode,
			Response:   body,
		}
	}

	return nil
}
