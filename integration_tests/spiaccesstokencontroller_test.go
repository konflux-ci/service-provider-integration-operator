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
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("Auto-creation of token", func() {
	var createdToken *api.SPIAccessToken

	var _ = BeforeEach(func() {
		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: api.ServiceProviderTypeGitHub,
				Permissions:         api.Permissions{},
				RawTokenData: &api.Token{
					AccessToken: "nazdar",
				},
			},
		}

		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), createdToken)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("updates object with token location", func() {
		token := &api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
		Expect(token.Spec.DataLocation).NotTo(BeEmpty())
	})

	It("removes the raw token from the object", func() {
		t := api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), &t)).To(Succeed())

		Expect(t.Spec.RawTokenData).To(BeNil())
		data, err := ITest.TokenStorage.Get(ITest.Context, &t)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())
		Expect(data.AccessToken).To(Equal("nazdar"))
	})

	It("updates the token data with update", func() {
		t := api.SPIAccessToken{}
		Eventually(func() error {
			// we need to do this in the eventually loop because we might be stepping over each others' toes with the controller
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), &t)).To(Succeed())

			t.Spec.RawTokenData = &api.Token{
				AccessToken: "updated",
			}

			return ITest.Client.Update(ITest.Context, &t)
		}).Should(Succeed())

		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(&t), &t)).To(Succeed())

		Expect(t.Spec.RawTokenData).To(BeNil())
		data, err := ITest.TokenStorage.Get(ITest.Context, &t)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())
		Expect(data.AccessToken).To(Equal("updated"))
	})
})

var _ = Describe("Create without token data", func() {
	var createdToken *api.SPIAccessToken

	var _ = BeforeEach(func() {
		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: api.ServiceProviderTypeGitHub,
				Permissions:         api.Permissions{},
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), createdToken)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("doesn't auto-create the token", func() {
		accessToken := &api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())

		tokenData, err := ITest.TokenStorage.Get(ITest.Context, accessToken)
		Expect(err).NotTo(HaveOccurred())
		Expect(tokenData).To(BeNil())
	})

	updateWithData := func() *api.SPIAccessToken {
		accessToken := &api.SPIAccessToken{}
		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())

			accessToken.Spec.RawTokenData = &api.Token{
				AccessToken:  "accessToken",
				TokenType:    "type",
				RefreshToken: "none",
				Expiry:       1,
			}

			g.Expect(ITest.Client.Update(ITest.Context, accessToken)).To(Succeed())
		}).Should(Succeed())
		return accessToken
	}

	It("and update with token data auto-creates the token", func() {
		accessToken := updateWithData()

		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())
			g.Expect(accessToken.Spec.DataLocation).NotTo(BeEmpty())
		}).Should(Succeed())

		tokenData, err := ITest.TokenStorage.Get(ITest.Context, accessToken)
		Expect(err).NotTo(HaveOccurred())
		Expect(tokenData).NotTo(BeNil())

		Expect(tokenData.AccessToken).To(Equal("accessToken"))
		Expect(tokenData.TokenType).To(Equal("type"))
		Expect(tokenData.RefreshToken).To(Equal("none"))
		Expect(tokenData.Expiry).To(Equal(uint64(1)))
	})

	It("allows data location to be updated", func() {
		accessToken := &api.SPIAccessToken{}

		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())
			g.Expect(accessToken.Spec.DataLocation).To(BeEmpty())

			accessToken.Spec.DataLocation = "over there"

			g.Expect(ITest.Client.Update(ITest.Context, accessToken)).To(Succeed())
		}).Should(Succeed())

		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())
		Expect(accessToken.Spec.DataLocation).To(Equal("over there"))

		accessToken = updateWithData()

		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())
			g.Expect(accessToken.Spec.DataLocation).NotTo(Equal("over there"))
		}).Should(Succeed())
	})

	It("allows token metadata to be updated", func() {

		accessToken := updateWithData()
		Expect(accessToken.Spec.TokenMetadata).To(BeNil())

		Eventually(func(g Gomega) {
			accessToken = &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())
			accessToken.Spec.TokenMetadata = &api.TokenMetadata{
				UserName: "the-user-that-belongs-to-the-update-test",
			}

			g.Expect(ITest.Client.Update(ITest.Context, accessToken)).To(Succeed())
		}).Should(Succeed())

		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())

		Expect(accessToken.Spec.TokenMetadata).NotTo(BeNil())
		Expect(accessToken.Spec.TokenMetadata.UserName).To(Equal("the-user-that-belongs-to-the-update-test"))
	})
})

var _ = Describe("Delete token", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken
	bindingDeleted := false

	BeforeEach(func() {
		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: "TestServiceProvider",
			},
		}
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
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
		Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())

		Expect(getLinkedToken(Default, createdBinding).UID).To(Equal(createdToken.UID))
	})

	AfterEach(func() {
		binding := &api.SPIAccessTokenBinding{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

		token := &api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())

		Expect(ITest.Client.Delete(ITest.Context, binding)).To(Succeed())

		if bindingDeleted {
			Eventually(func () error {
				return ITest.Client.Delete(ITest.Context, token)
			}).Should(Not(Succeed()))
		} else {
			Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
		}
	})

	It("cannot happen as long as there are linked bindings", func() {
		token := &api.SPIAccessToken{}

		Eventually(func(g Gomega) {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Labels[opconfig.SPIAccessTokenLinkLabel],
				Namespace: binding.Namespace}, token)).To(Succeed())
		}).Should(Succeed())

		// the delete request should succeed
		Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
		// but the resource should not get deleted because of a finalizer that checks for the present bindings
		time.Sleep(1 * time.Second)
		bindingDeleted = true
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(token), &api.SPIAccessToken{})).To(Succeed())
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
