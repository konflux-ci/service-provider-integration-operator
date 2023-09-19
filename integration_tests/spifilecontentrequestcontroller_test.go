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
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
)

var _ = Describe("SPIFileContentRequest", func() {

	Describe("without SPIAccessToken or RemoteSecret", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("create-wo-token-data")},
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
		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("should have the status in error phase", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.FileContentRequests[0].Status.Phase).To(Equal(api.SPIFileContentRequestPhaseError))
				g.Expect(testSetup.InCluster.FileContentRequests[0].Status.ErrorMessage).To(ContainSubstring("no suitable credentials"))
			})
		})
	})

	Describe("file download fails", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("create-with-binding-in-error")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.LookupCredentialsImpl = func(ctx context.Context, c client.Client, matchable serviceprovider.Matchable) (*serviceprovider.Credentials, error) {
						return &serviceprovider.Credentials{Token: "abcd"}, nil
					}
					ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability {
						return serviceprovider.DownloadFileFunc(func(ctx context.Context, request api.SPIFileContentRequestSpec, credentials serviceprovider.Credentials, maxFileSizeLimit int) (string, error) {
							return "", fmt.Errorf("expected error")
						})
					}
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("should have the status in error phase", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				request := testSetup.InCluster.FileContentRequests[0]
				g.Expect(request.Status.Content).To(BeEmpty())
				g.Expect(request.Status.ContentEncoding).To(BeEmpty())
				g.Expect(request.Status.Phase).To(Equal(api.SPIFileContentRequestPhaseError))
				g.Expect(request.Status.ErrorMessage).To(ContainSubstring("expected error"))
			})
		})
	})

	Describe("provider doesn't support file download", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("no-download")},
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
				request := testSetup.InCluster.FileContentRequests[0]
				g.Expect(request.Status.Content).To(BeEmpty())
				g.Expect(request.Status.ContentEncoding).To(BeEmpty())
				g.Expect(request.Status.Phase).To(Equal(api.SPIFileContentRequestPhaseError))
				g.Expect(request.Status.ErrorMessage).To(ContainSubstring(serviceprovider.FileDownloadNotSupportedError{}.Error()))
			})
		})
	})

	Describe("request is removed", func() {
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

		It("should delete request by timeout", func() {
			ITest.OperatorConfiguration.FileContentRequestTtl = 500 * time.Millisecond
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.FileContentRequests).To(BeEmpty())
			})
		})
	})

	Describe("Credentials are available", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				FileContentRequests: []*api.SPIFileContentRequest{StandardFileRequest("credentials-ready")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability { return &ITest.Capabilities }
					ITest.TestServiceProvider.LookupCredentialsImpl =
						func(ctx context.Context, c client.Client, matchable serviceprovider.Matchable) (*serviceprovider.Credentials, error) {
							return &serviceprovider.Credentials{Token: "abcd"}, nil
						}
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("delivers the content", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				request := testSetup.InCluster.FileContentRequests[0]
				g.Expect(request.Status.Phase).To(Equal(api.SPIFileContentRequestPhaseDelivered))
				g.Expect(request.Status.ContentEncoding).To(Equal("base64"))
				g.Expect(request.Status.Content).To(Equal(base64.StdEncoding.EncodeToString([]byte("abcdefg"))))
			})
		})
	})
})
