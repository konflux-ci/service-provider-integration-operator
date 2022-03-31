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

type ServiceProviderError int

const (
	InvalidAccessToken ServiceProviderError = iota
	InternalServerError
)

func (e ServiceProviderError) Error() string {
	switch e {
	case InvalidAccessToken:
		return "invalid access token"
	case InternalServerError:
		return "error in the service provider"
	}
	return "unknown service provider error"
}

func IsServiceProviderError(err error) bool {
	return err == InternalServerError || err == InvalidAccessToken
}

func FromHttpStatusCode(statusCode int) error {
	if statusCode >= 400 && statusCode < 500 {
		return InvalidAccessToken
	} else if statusCode >= 500 && statusCode < 600 {
		return InternalServerError
	}

	return nil
}
