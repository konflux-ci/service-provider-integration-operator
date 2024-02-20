//
// Copyright 2023 Red Hat, Inc.
//
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

package contracts

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pact-foundation/pact-go/v2/provider"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	oauth "github.com/redhat-appstudio/service-provider-integration-operator/oauth"
)

func TestContracts(t *testing.T) {
	// Register fail handler and setup test environment (same as during unit tests)
	RegisterFailHandler(Fail)
	IT, ctx = oauth.StartTestEnv()

	// Create and setup Pact Verifier
	verifyRequest := createVerifier(t)

	// Run pact tests
	err := provider.NewVerifier().VerifyProvider(t, verifyRequest)
	if err != nil {
		t.Errorf("Error while verifying tests. \n %+v", err)
	}

	cleanUpNamespaces()

	IT.Cancel()

	err = IT.TestEnvironment.Stop()
	if err != nil {
		fmt.Println("Stopping failed")
		fmt.Printf("%+v", err)
		panic("Cleanup failed")
	}
}

func createVerifier(t *testing.T) provider.VerifyRequest {
	verifyRequest := provider.VerifyRequest{
		Provider:        "SPI",
		RequestTimeout:  60 * time.Second,
		ProviderBaseURL: IT.TestEnvironment.Config.Host,
		// Default selector should include environments, but as they are not in place yet, using just main branch
		ConsumerVersionSelectors:   []provider.Selector{&provider.ConsumerVersionSelector{Branch: "main"}},
		BrokerURL:                  os.Getenv("PACT_BROKER_URL"),
		PublishVerificationResults: false,
		EnablePending:              true,
		ProviderVersion:            "local",
		ProviderBranch:             "main",
	}

	// clean up test env before every test
	verifyRequest.BeforeEach = func() error {
		// workaround for https://github.com/pact-foundation/pact-go/issues/359
		if os.Getenv("SETUP") == "true" {
			return nil
		}
		cleanUpNamespaces()
		return nil
	}

	// setup credentials and publishing
	if os.Getenv("PR_CHECK") != "true" {
		if os.Getenv("PACT_BROKER_USERNAME") == "" {
			// To run Pact tests against local contract files, set LOCAL_PACT_FILES_FOLDER to the folder with pact jsons
			var pactDir, useLocalFiles = os.LookupEnv("LOCAL_PACT_FILES_FOLDER")
			if useLocalFiles {
				verifyRequest.BrokerPassword = ""
				verifyRequest.BrokerUsername = ""
				verifyRequest.BrokerURL = ""
				verifyRequest.PactFiles = []string{filepath.ToSlash(fmt.Sprintf("%s/HACdev-SPI.json", pactDir))}
				t.Log("Running tests locally. Verifying tests from local folder: ", pactDir)
			} else {
				t.Log("Running tests locally. Verifying against main branch, not pushing results to broker.")
				verifyRequest.ConsumerVersionSelectors = []provider.Selector{&provider.ConsumerVersionSelector{Branch: "main"}}
				// to test against changes in specific HAC-dev PR, use Tag:
				// verifyRequest.ConsumerVersionSelectors = []provider.Selector{&provider.ConsumerVersionSelector{Tag: "PR808", Latest: true}}
			}
		} else {
			t.Log("Running tests post-merge. Verifying against main branch and all environments. Pushing results to Pact broker with the branch \"main\".")
			verifyRequest.BrokerUsername = os.Getenv("PACT_BROKER_USERNAME")
			verifyRequest.BrokerPassword = os.Getenv("PACT_BROKER_PASSWORD")
			verifyRequest.ProviderBranch = os.Getenv("PROVIDER_BRANCH")
			verifyRequest.ProviderVersion = os.Getenv("COMMIT_SHA")
			verifyRequest.PublishVerificationResults = true
		}
	}

	// setup state handlers
	verifyRequest.StateHandlers = setupStateHandler()

	// Certificate magic - for the mocked service to be able to communicate with kube-apiserver & for authorization
	verifyRequest.CustomTLSConfig = createTlsConfig()

	return verifyRequest
}

func createTlsConfig() *tls.Config {
	caCertPool := x509.NewCertPool()

	// try to read cert data from config
	caCertPool.AppendCertsFromPEM(IT.TestEnvironment.Config.CAData)
	certs, err := tls.X509KeyPair(IT.TestEnvironment.Config.CertData, IT.TestEnvironment.Config.KeyData)
	if err != nil {
		fmt.Println("Could not read cert data from config:", err)
		fmt.Println("Trying to load the cert files themselves.")
		CAData, err := loadCertFromFile(IT.TestEnvironment.Config.CAFile)
		if err != nil {
			panic(err)
		}
		certPEM, err := ioutil.ReadFile(IT.TestEnvironment.Config.CertFile)
		if err != nil {
			fmt.Println("Error reading certificate file:", err)
			panic(err)
		}
		keyPEM, err := ioutil.ReadFile(IT.TestEnvironment.Config.KeyFile)
		if err != nil {
			fmt.Println("Error reading private key file:", err)
			panic(err)
		}

		caCertPool.AddCert(CAData)
		certs, err = tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			panic(err)
		}
	}
	return &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{certs},
	}
}

func loadCertFromFile(certFile string) (*x509.Certificate, error) {
	certPEM, err := ioutil.ReadFile(certFile)
	if err != nil {
		fmt.Println("Error reading certificate file:", err)
		return nil, err
	}

	// Parse the PEM-encoded certificate
	block, _ := pem.Decode(certPEM)
	if block == nil {
		fmt.Println("Failed to decode PEM block")
		return nil, err
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Println("Error parsing certificate:", err)
		return nil, err
	}

	return cert, nil
}
