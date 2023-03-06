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

package integrationtests

import (
	"encoding/base64"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SPIFileContentRequest", func() {

	Describe("Create without token data", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("create-wo-token-data")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability { return &ITest.Capabilities }
					ITest.TestServiceProvider.OAuthCapability = func() serviceprovider.OAuthCapability { return &ITest.Capabilities }
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("have the status awaiting set", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.FileContentRequests[0].Status.Phase == api.SPIFileContentRequestPhaseAwaitingTokenData).To(BeTrue())
			})
		})

		It("have the upload and OAUth URLs set", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				request := testSetup.InCluster.FileContentRequests[0]
				g.Expect(request.Status.TokenUploadUrl).NotTo(BeEmpty())
				g.Expect(request.Status.OAuthUrl).NotTo(BeEmpty())
			})
		})
	})

	Describe("With binding is in error", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("create-with-binding-in-error")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability { return &ITest.Capabilities }
				},
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "alois",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})
					err := ITest.TokenStorage.Store(ITest.Context, objects.Tokens[0], &api.Token{
						AccessToken: "token",
					})
					Expect(err).NotTo(HaveOccurred())
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(func(g Gomega) {
				g.Expect(testSetup.InCluster.Bindings[0].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
			})
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("sets request into error too", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				request := testSetup.InCluster.FileContentRequests[0]
				g.Expect(request.Status.Phase).To(Equal(api.SPIFileContentRequestPhaseError))
				g.Expect(request.Status.ErrorMessage).To(HavePrefix("linked binding is in error state"))
			})
		})

	})

	Describe("Request is removed", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("request-removal")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability { return &ITest.Capabilities }
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("it removes the binding, too", func() {
			// let's just double-check that we indeed have the binding
			testSetup.ReconcileWithCluster(func(g Gomega) {
				req := testSetup.InCluster.FileContentRequests[0]
				var linkedBinding *api.SPIAccessTokenBinding = nil
				for _, binding := range testSetup.InCluster.Bindings {
					if binding.GetLabels()[controllers.LinkedFileRequestLabel] == req.Name {
						linkedBinding = binding
					}
				}
				g.Expect(linkedBinding).NotTo(BeNil())
			})

			Expect(ITest.Client.Delete(ITest.Context, testSetup.InCluster.FileContentRequests[0])).To(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Bindings).To(BeEmpty())
			})
		})

		It("should delete request by timeout", func() {
			ITest.OperatorConfiguration.FileContentRequestTtl = 500 * time.Millisecond
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.FileContentRequests).To(BeEmpty())
			})
		})
	})

	Describe("With binding is ready", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("binding-ready")},
				Tokens:              []*api.SPIAccessToken{StandardTestToken("binding-ready")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability { return &ITest.Capabilities }
				},
				AfterObjectsCreated: func(objects TestObjects) {
					token := objects.GetTokensByNamePrefix(client.ObjectKey{Name: "binding-ready", Namespace: "default"})[0]

					ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "alois",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})
					err := ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
						AccessToken: "token",
					})
					Expect(err).NotTo(HaveOccurred())
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&token)
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("sets request into delivered, too", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				request := testSetup.InCluster.FileContentRequests[0]
				g.Expect(request.Status.Phase).To(Equal(api.SPIFileContentRequestPhaseDelivered))
				g.Expect(request.Status.Content).To(Equal(base64.StdEncoding.EncodeToString([]byte("abcdefg"))))
			})
		})
	})

	Describe("With binding is ready but provider doesn't support  downloads", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("binding-ready-no-download")},
				Tokens:              []*api.SPIAccessToken{StandardTestToken("binding-ready-no-download")},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					token := objects.GetTokensByNamePrefix(client.ObjectKey{Name: "binding-ready-no-download", Namespace: "default"})[0]

					ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "alois",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})
					err := ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
						AccessToken: "token",
					})
					Expect(err).NotTo(HaveOccurred())
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&token)
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("sets request into error, too", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.FileContentRequests[0].Status.Phase).To(Equal(api.SPIFileContentRequestPhaseError))
			})
		})
	})
})
