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

// Probe is a simple function that can determine whether a URL can be handled by a certain service
// provider.
type Probe interface {
	// Examine returns the base url of the service provider, if the provided URL can be handled by that provider or
	// an empty string if it cannot. The provided http client can be used to perform requests against the URL if needed.
	Examine(cl *http.Client, url string) (string, error)
}

// ProbeFunc provides the Probe implementation for compatible functions
type ProbeFunc func(*http.Client, string) (string, error)

// Constructor is able to produce a new service provider instance using data from the provided Factory and
// the base URL of the service provider.
type Constructor interface {
	// Construct creates a new instance of service provider
	Construct(factory *Factory, baseUrl string) (ServiceProvider, error)
}

// ConstructorFunc converts a compatible function into the Constructor interface
type ConstructorFunc func(factory *Factory, baseUrl string) (ServiceProvider, error)

// Initializer is struct that contains all necessary data to initialize a service provider instance from a URL using
// a Factory.
type Initializer struct {
	Probe                        Probe
	Constructor                  Constructor
	SupportsManualUploadOnlyMode bool
}

// implementation guards
var _ Probe = ProbeFunc(nil)
var _ Constructor = ConstructorFunc(nil)

func (p ProbeFunc) Examine(cl *http.Client, url string) (string, error) {
	return p(cl, url)
}

func (c ConstructorFunc) Construct(factory *Factory, baseUrl string) (ServiceProvider, error) {
	return c(factory, baseUrl)
}
