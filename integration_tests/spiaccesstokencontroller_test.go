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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Create without token data", func() {
	var createdToken *api.SPIAccessToken

	var _ = BeforeEach(func() {
		ITest.TestServiceProvider.Reset()

		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "test-provider://",
				Permissions:        api.Permissions{},
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), createdToken)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("sets up the finalizers", func() {
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
			g.Expect(token.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-bindings"))
			g.Expect(token.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/token-storage"))
		}).Should(Succeed())
	})

	It("doesn't auto-create the token data", func() {
		accessToken := &api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())

		tokenData, err := ITest.TokenStorage.Get(ITest.Context, accessToken)
		Expect(err).NotTo(HaveOccurred())
		Expect(tokenData).To(BeNil())
	})
})

var _ = Describe("Delete token", func() {
	var createdToken *api.SPIAccessToken
	tokenDeleteInProgress := false

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()

		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "test-provider://",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
	})

	AfterEach(func() {
		token := &api.SPIAccessToken{}
		if tokenDeleteInProgress {
			err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)
			if err == nil {
				Eventually(func() error {
					return ITest.Client.Delete(ITest.Context, token)
				}).ShouldNot(Succeed())
			} else {
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		} else {
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
			Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
		}
	})

	When("there are linked bindings", func() {
		var createdBinding *api.SPIAccessTokenBinding

		BeforeEach(func() {
			ITest.TestServiceProvider.Reset()
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

			createdBinding = &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-binding",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					Permissions: api.Permissions{},
					RepoUrl:     "test-provider://",
				},
			}
			Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
			Expect(getLinkedToken(Default, createdBinding).UID).To(Equal(createdToken.UID))
		})

		AfterEach(func() {
			binding := &api.SPIAccessTokenBinding{}
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			Expect(ITest.Client.Delete(ITest.Context, binding)).To(Succeed())
		})

		It("doesn't happen", func() {
			token := &api.SPIAccessToken{}
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())

			// the delete request should succeed
			Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
			tokenDeleteInProgress = true
			// but the resource should not get deleted because of a finalizer that checks for the present bindings
			time.Sleep(1 * time.Second)
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(token), token)).To(Succeed())
			Expect(token.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-bindings"))
		})
	})

	It("deletes token data from storage", func() {
		// store the token data
		err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
			AccessToken: "42",
		})
		Expect(err).NotTo(HaveOccurred())

		// check that we can read the data from the storage
		data, err := ITest.TokenStorage.Get(ITest.Context, createdToken)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())

		// delete the token
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
			g.Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
			tokenDeleteInProgress = true
		}).Should(Succeed())

		// test that the data disappears, too
		Eventually(func(g Gomega) {
			data, err := ITest.TokenStorage.Get(ITest.Context, createdToken)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(data).To(BeNil())
		}).Should(Succeed())
	})
})

func getLinkedToken(g Gomega, binding *api.SPIAccessTokenBinding) *api.SPIAccessToken {
	token := &api.SPIAccessToken{}

	g.Eventually(func(g Gomega) {
		loadedBinding := &api.SPIAccessTokenBinding{}
		g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(binding), loadedBinding)).To(Succeed())
		g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: loadedBinding.Labels[opconfig.SPIAccessTokenLinkLabel],
			Namespace: binding.Namespace}, token)).To(Succeed())
	}).Should(Succeed())

	return token
}
