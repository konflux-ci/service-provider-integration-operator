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

package serviceprovider

import "net/http"

// serviceProviderProbe is a simple function that can determine whether a URL can be handled by a certain service
// provider.
type serviceProviderProbe interface {
	// Probe returns the base url of the service provider, if the provided URL can be handled by that provider or
	// an empty string if it cannot. The provided http client can be used to perform requests against the URL if needed.
	Probe(cl *http.Client, url string) (string, error)
}
