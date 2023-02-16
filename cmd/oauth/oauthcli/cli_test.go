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

package oauthcli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/alexflint/go-arg"
)

func TestK8sConfigParse(t *testing.T) {
	//given
	cmd := ""
	env := []string{"API_SERVER=http://localhost:9001", "API_SERVER_CA_PATH=/etc/ca.crt"}
	//then
	args := OAuthServiceCliArgs{}
	_, err := parseWithEnv(cmd, env, &args)
	//when
	if err != nil {
		t.Fatal(err)
	}
	if args.ApiServer != "http://localhost:9001" {
		t.Fatal("Unable to parse k8s api server url")
	}
	if args.ApiServerCAPath != "/etc/ca.crt" {
		t.Fatal("Unable to parse k8s ca path")
	}
}

func TestCorsConfigParse(t *testing.T) {
	//given
	cmd := ""
	env := []string{"ALLOWEDORIGINS=prod.acme.com"}
	//then
	args := OAuthServiceCliArgs{}
	_, err := parseWithEnv(cmd, env, &args)
	//when
	if err != nil {
		t.Fatal(err)
	}
	if args.AllowedOrigins != "prod.acme.com" {
		t.Fatal("Unable to parse CORS allowed origins")
	}
}

func parseWithEnv(cmdline string, env []string, dest interface{}) (*arg.Parser, error) {
	p, err := arg.NewParser(arg.Config{}, dest)
	if err != nil {
		return nil, err
	}

	// split the command line
	var parts []string
	if len(cmdline) > 0 {
		parts = strings.Split(cmdline, " ")
	}

	// split the environment vars
	for _, s := range env {
		pos := strings.Index(s, "=")
		if pos == -1 {
			return nil, fmt.Errorf("missing equals sign in %q", s)
		}
		err := os.Setenv(s[:pos], s[pos+1:])
		if err != nil {
			return nil, err
		}
	}

	// execute the parser
	return p, p.Parse(parts)
}
