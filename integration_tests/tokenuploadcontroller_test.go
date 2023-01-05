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
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("TokenUploadController", func() {
	Describe("Create with token", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{StandardTestToken("upload-token")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {

					ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "gena",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})

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

		It("updateи the SPIAccessToken status", func() {

			createSecret("test-token", testSetup.InCluster.Tokens[0].Name, "")
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Tokens[0].Status.Phase == api.SPIAccessTokenPhaseReady).To(BeTrue())
				g.Expect(testSetup.InCluster.Tokens[0].Status.TokenMetadata.UserId == "42").To(BeTrue())
				// The secret should be deleted
				g.Expect(ITest.Client.Get(ITest.Context, types.NamespacedName{Name: "test-token", Namespace: "default"}, &corev1.Secret{}).Error())
			})
		})
		It("failed with event as SPIAccessToken does not exist", func() {
			createSecret("test-token-err", "not-exists", "")
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(errorMessage("test-token-err")).
					Should(MatchRegexp("can not find SPI access token not-exists: SPIAccessToken.appstudio.redhat.com \"not-exists\" not found"))

			})
		})
		It("finds the SPIAccessToken by serviceProviderURL and updates the status", func() {

			createSecret("test-token", "", testSetup.InCluster.Tokens[0].Spec.ServiceProviderUrl)
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Tokens[0].Status.Phase == api.SPIAccessTokenPhaseReady).To(BeTrue())
				g.Expect(testSetup.InCluster.Tokens[0].Status.TokenMetadata.UserId == "42").To(BeTrue())
			})
		})
	})
})

func createSecret(name string, spiAccessTokenName string, serviceProviderURL string) {

	o := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			Labels: map[string]string{
				"spi.appstudio.redhat.com/upload-secret": "token",
			},
		},
		Type: "Opaque",
		StringData: map[string]string{
			"tokenData": "token-data",
		},
	}

	if spiAccessTokenName != "" {
		o.Labels["spi.appstudio.redhat.com/token-name"] = spiAccessTokenName
	} else if serviceProviderURL != "" {
		o.StringData["providerUrl"] = serviceProviderURL
	}

	Expect(ITest.Client.Create(ITest.Context, o)).To(Succeed())
}

func errorMessage(secretName string) string {
	event := &corev1.Event{}
	err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: secretName, Namespace: "default"}, event)
	if err != nil {
		Fail("Can't get the Event")
	}
	return event.Message
}
