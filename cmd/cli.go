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

package cmd

import (
	"crypto/tls"

	rcmd "github.com/redhat-appstudio/remote-secret/pkg/cmd"
)

// CommonCliArgs are the command line arguments and environment variable definitions understood by the configuration
// infrastructure shared between the operator and the oauth service.
type CommonCliArgs struct {
	rcmd.CommonCliArgs
	rcmd.LoggingCliArgs
	BaseUrl      string `arg:"--base-url, env" help:"The externally accessible URL on which the OAuth service is listening. This is used to construct manual-upload and OAuth URLs"`
	DisableHTTP2 bool   `arg:"--disable-http2, env" default:"true" help:"whether to disable access using the HTTP/2 protocol."`
}

var TLSConfigWithDisabledHTTP2 = &tls.Config{
	MinVersion: tls.VersionTLS12,
	NextProtos: []string{"http/1.1"},
}
