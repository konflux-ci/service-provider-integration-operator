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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type nestingError struct {
	nested error
}

func (e *nestingError) Error() string {
	return ""
}

func (e *nestingError) Unwrap() error {
	return e.nested
}

func TestChecks(t *testing.T) {
	invalidAccessToken := &ServiceProviderHttpError{
		StatusCode: 401,
		Response:   "",
	}

	internalServerError := &ServiceProviderHttpError{
		StatusCode: 501,
		Response:   "",
	}

	nestedInvalidAccessToken := &nestingError{invalidAccessToken}
	nestedInternalServerError := &nestingError{internalServerError}

	assert.True(t, errors.Is(invalidAccessToken, invalidAccessToken))
	assert.True(t, errors.Is(nestedInvalidAccessToken, invalidAccessToken))
	assert.False(t, errors.Is(invalidAccessToken, internalServerError))
	assert.False(t, errors.Is(invalidAccessToken, fmt.Errorf("huh")))
	assert.True(t, IsServiceProviderHttpError(invalidAccessToken))
	assert.True(t, IsServiceProviderHttpError(internalServerError))
	assert.True(t, IsServiceProviderHttpError(nestedInvalidAccessToken))
	assert.False(t, IsServiceProviderHttpError(fmt.Errorf("huh")))
	assert.False(t, IsServiceProviderHttpError(nil))

	assert.True(t, IsServiceProviderHttpInvalidAccessToken(invalidAccessToken))
	assert.True(t, IsServiceProviderHttpInvalidAccessToken(nestedInvalidAccessToken))
	assert.False(t, IsServiceProviderHttpInvalidAccessToken(internalServerError))
	assert.False(t, IsServiceProviderHttpInvalidAccessToken(nestedInternalServerError))
	assert.False(t, IsServiceProviderHttpInvalidAccessToken(fmt.Errorf("huh")))

	assert.True(t, IsServiceProviderHttpInternalServerError(internalServerError))
	assert.True(t, IsServiceProviderHttpInternalServerError(nestedInternalServerError))
	assert.False(t, IsServiceProviderHttpInternalServerError(invalidAccessToken))
	assert.False(t, IsServiceProviderHttpInternalServerError(nestedInvalidAccessToken))
	assert.False(t, IsServiceProviderHttpInternalServerError(fmt.Errorf("huh")))
}

func TestConversion(t *testing.T) {
	invalidAccessToken := &ServiceProviderHttpError{
		StatusCode: 401,
		Response:   "an error",
	}

	test := func(t *testing.T, err *ServiceProviderHttpError) {
		assert.Equal(t, invalidAccessToken.StatusCode, err.StatusCode)
		assert.Equal(t, invalidAccessToken.Response, err.Response)
	}

	t.Run("direct", func(t *testing.T) {
		test(t, invalidAccessToken)
	})

	t.Run("nested", func(t *testing.T) {
		nesting := &nestingError{invalidAccessToken}
		extracted := &ServiceProviderHttpError{}

		assert.True(t, errors.As(nesting, &extracted))
		test(t, extracted)
	})
}

func TestFromHttpResponse(t *testing.T) {
	resp := http.Response{
		StatusCode: 401,
		Body:       io.NopCloser(strings.NewReader("an error")),
	}

	err := FromHttpResponse(&resp)
	assert.Equal(t, "invalid access token (http status 401): an error", err.Error())
}
