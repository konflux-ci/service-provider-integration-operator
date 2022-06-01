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
	invalidAccessToken := &ServiceProviderError{
		StatusCode: 401,
		Response:   "",
	}

	internalServerError := &ServiceProviderError{
		StatusCode: 501,
		Response:   "",
	}

	nestedInvalidAccessToken := &nestingError{invalidAccessToken}
	nestedInternalServerError := &nestingError{internalServerError}

	assert.True(t, errors.Is(invalidAccessToken, invalidAccessToken))
	assert.True(t, errors.Is(nestedInvalidAccessToken, invalidAccessToken))
	assert.False(t, errors.Is(invalidAccessToken, internalServerError))
	assert.False(t, errors.Is(invalidAccessToken, fmt.Errorf("huh")))
	assert.True(t, IsServiceProviderError(invalidAccessToken))
	assert.True(t, IsServiceProviderError(internalServerError))
	assert.True(t, IsServiceProviderError(nestedInvalidAccessToken))
	assert.False(t, IsServiceProviderError(fmt.Errorf("huh")))
	assert.False(t, IsServiceProviderError(nil))

	assert.True(t, IsInvalidAccessToken(invalidAccessToken))
	assert.True(t, IsInvalidAccessToken(nestedInvalidAccessToken))
	assert.False(t, IsInvalidAccessToken(internalServerError))
	assert.False(t, IsInvalidAccessToken(nestedInternalServerError))
	assert.False(t, IsInvalidAccessToken(fmt.Errorf("huh")))

	assert.True(t, IsInternalServerError(internalServerError))
	assert.True(t, IsInternalServerError(nestedInternalServerError))
	assert.False(t, IsInternalServerError(invalidAccessToken))
	assert.False(t, IsInternalServerError(nestedInvalidAccessToken))
	assert.False(t, IsInternalServerError(fmt.Errorf("huh")))
}

func TestConversion(t *testing.T) {
	invalidAccessToken := &ServiceProviderError{
		StatusCode: 401,
		Response:   "an error",
	}

	test := func(t *testing.T, err *ServiceProviderError) {
		assert.Equal(t, invalidAccessToken.StatusCode, err.StatusCode)
		assert.Equal(t, invalidAccessToken.Response, err.Response)
	}

	t.Run("direct", func(t *testing.T) {
		test(t, invalidAccessToken)
	})

	t.Run("nested", func(t *testing.T) {
		nesting := &nestingError{invalidAccessToken}
		extracted := &ServiceProviderError{}

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
