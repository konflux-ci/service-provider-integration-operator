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

//nolint:wrapcheck
package integrationtests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("TokenUploadController", func() {
	Describe("Upload token", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{StandardTestToken("upload-token")},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {

					ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "gena",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})

					ITest.OperatorConfiguration.EnableTokenUpload = true
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			for i := range testSetup.InCluster.Tokens {
				testSetup.InCluster.Tokens[i].Status.Phase = api.SPIAccessTokenPhaseAwaitingTokenData
			}

		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
			Expect(ITest.Client.DeleteAllOf(ITest.Context, &corev1.Secret{}, client.InNamespace("default"))).Should(Succeed())
		})

		It("updates existed SPIAccessToken's status", func() {
			accessToken := api.SPIAccessToken{}
			Expect(testSetup.InCluster.Tokens[0].Status.Phase).ToNot(Equal(api.SPIAccessTokenPhaseReady))
			createSecret("test-token", testSetup.InCluster.Tokens[0].Name, "")

			Eventually(func() error {
				// Token added to Storage...
				_, err := ITest.TokenStorage.Get(ITest.Context, &accessToken)

				return err
			}).Should(Succeed(), "Token was not added to Storage")

			Eventually(func() bool {
				// SPIAccessToken Updated
				err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: testSetup.InCluster.Tokens[0].Name, Namespace: "default"}, &accessToken)
				if err != nil {
					return false
				}

				return accessToken.Status.TokenMetadata.UserId == "42" && accessToken.Status.Phase == api.SPIAccessTokenPhaseReady
			}).Should(BeTrue(), "SPIAccessToken was not updated")

			Eventually(func() error {
				// And the secret deleted eventually
				err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: "test-token", Namespace: "default"}, &corev1.Secret{})

				return err
			}).ShouldNot(Succeed(), "Secret was not deleted eventually")
		})
		It("creates new SPIAccessToken and updates its status", func() {
			spiTokenName := "new-spitoken"
			accessToken := api.SPIAccessToken{}
			Expect(ITest.Client.Get(ITest.Context, types.NamespacedName{Name: spiTokenName, Namespace: "default"}, &accessToken)).ShouldNot(Succeed())
			createSecret("test-token2", spiTokenName, testSetup.InCluster.Tokens[0].Spec.ServiceProviderUrl)

			Eventually(func() error {
				// Token added to Storage...
				_, err := ITest.TokenStorage.Get(ITest.Context, &accessToken)

				return err
			}).Should(Succeed(), "Token was not added to Storage")

			Eventually(func() bool {
				// And new SPIAccessToken created and moved to Ready state...
				err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: spiTokenName, Namespace: "default"}, &accessToken)
				if err != nil {
					return false
				}

				return accessToken.Name == spiTokenName && accessToken.Status.Phase == api.SPIAccessTokenPhaseReady
			}).Should(BeTrue(), "SPIAccessToken was not created and moved to Ready state")

			Eventually(func() error {
				// And the secret deleted eventually
				err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: "test-token2", Namespace: "default"}, &corev1.Secret{})

				return err
			}).ShouldNot(Succeed(), "Secret was not deleted eventually")
		})
		It("fails creating SPIAccessToken b/c SPIAccessToken name is invalid", func() {
			accessToken := api.SPIAccessToken{}

			createSecret("secret", "my-spi-access-token_2023_03_02__15_37_28", "")
			Eventually(ITest.Client.Get).WithArguments(ITest.Context, types.NamespacedName{Name: "not-existed-spitoken", Namespace: "default"}, &accessToken).ShouldNot(Succeed())
		})
		It("leaves non-annotated secrets alone", func() {
			accessToken := api.SPIAccessToken{}
			Expect(testSetup.InCluster.Tokens[0].Status.Phase).ToNot(Equal(api.SPIAccessTokenPhaseReady))
			createSecret("test-token", testSetup.InCluster.Tokens[0].Name, "")
			noTouchSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-touching",
					Namespace: "default",
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"ryans": "privates",
				},
			}
			Expect(ITest.Client.Create(ITest.Context, noTouchSecret)).To(Succeed())

			Eventually(func() error {
				// Token added to Storage...
				_, err := ITest.TokenStorage.Get(ITest.Context, &accessToken)

				return err
			}).Should(Succeed(), "Token was not added to Storage")

			Eventually(func() bool {
				// SPIAccessToken Updated
				err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: testSetup.InCluster.Tokens[0].Name, Namespace: "default"}, &accessToken)
				if err != nil {
					return false
				}

				return accessToken.Status.TokenMetadata.UserId == "42" && accessToken.Status.Phase == api.SPIAccessTokenPhaseReady
			}).Should(BeTrue(), "SPIAccessToken was not created and moved to Ready state")

			Eventually(func() error {
				// And the secret deleted eventually
				err := ITest.Client.Get(ITest.Context, types.NamespacedName{Name: "test-token", Namespace: "default"}, &corev1.Secret{})

				return err
			}).ShouldNot(Succeed(), "Secret was not deleted eventually")
			Consistently(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, types.NamespacedName{Name: "no-touching", Namespace: "default"}, &corev1.Secret{})).To(Succeed())
			}, "1s").Should(Succeed())
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
		o.StringData["spiTokenName"] = spiAccessTokenName
	}
	if serviceProviderURL != "" {
		o.StringData["providerUrl"] = serviceProviderURL
	}

	Expect(ITest.Client.Create(ITest.Context, o)).To(Succeed())
}
