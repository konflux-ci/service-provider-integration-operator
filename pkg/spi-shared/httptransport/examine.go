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

package httptransport

import "net/http"

// RoundTripExaminer is interface enabling the implementors to examine the request and response during the roundtrip
// and potentially somehow modify them or turn them into error.
type RoundTripExaminer interface {
	Examine(req *http.Request, response *http.Response) error
}

// RoundTripExaminerFunc is a conversion of a function into RoundTripExaminer
type RoundTripExaminerFunc func(*http.Request, *http.Response) error

func (r RoundTripExaminerFunc) Examine(req *http.Request, res *http.Response) error {
	return r(req, res)
}

// ExaminingRoundTripper is an HTTP request round tripper that calls the Examiner after the request is made so that
// it can examine the request and response or turn it into an error.
type ExaminingRoundTripper struct {
	http.RoundTripper
	Examiner RoundTripExaminer
}

var _ http.RoundTripper = (*ExaminingRoundTripper)(nil)

func (r ExaminingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := r.RoundTripper.RoundTrip(req)
	if err != nil {
		return res, err //nolint:wrapcheck // the errors should be handled by the users of the HTTP client configured with this roundtripper
	}

	return res, r.Examiner.Examine(req, res) //nolint:wrapcheck // the errors should be handled by the users of the HTTP client configured with this roundtripper
}
