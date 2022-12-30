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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TokenUploadController", func() {
	Describe("Create with token", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{StandardTestToken("upload-token")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					//ITest.TestServiceProvider.DownloadFileCapability = func() serviceprovider.DownloadFileCapability { return testCapability{} }
					//ITest.TestServiceProvider.GetOauthEndpointImpl = func() string {
					//	return "test-provider://test"
					//}
					//ITest.OperatorConfiguration.EnableTokenUpload = true
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			//ITest.OperatorConfiguration.EnableTokenUpload = true
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("update the token", func() {

			o := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test-token",
					Labels: map[string]string{
						"spi.appstudio.redhat.com/upload-secret": "token",
						"spi.appstudio.redhat.com/token-name":    testSetup.InCluster.Tokens[0].Name,
					},
				},
				Type: "Opaque",
				StringData: map[string]string{
					"tokenData": "token-data",
				},
			}

			Expect(ITest.Client.Create(ITest.Context, o)).To(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				// It does not work because SPIAccessTokenDataUpdate controller removes the SPIAccessTokenDataUpdate object
				// before SPIAccessToken controller tries to use it ("token data update already gone from the cluster")
				// so it has no token data to work with and can not move to SPIAccessTokenPhaseReady phase.
				g.Expect(testSetup.InCluster.Tokens[0].Status.Phase == api.SPIAccessTokenPhaseReady).To(BeTrue())
			})
		})
	})
})
