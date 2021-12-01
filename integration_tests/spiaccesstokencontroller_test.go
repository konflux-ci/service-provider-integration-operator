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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Auto-creation of token", func() {
	var _ = BeforeEach(func() {
		Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: api.ServiceProviderTypeGitHub,
				Permissions:         []api.Permission{api.PermissionRead},
				RawTokenData: &api.Token{
					AccessToken: "nazdar",
				},
			},
		})).To(Succeed())
	})

	var _ = AfterEach(func() {
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-token", Namespace: "default"}, token)).To(Succeed())
			g.Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
		}).Should(Succeed())
	})

	It("updates object with token location", func() {
		token := &api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-token", Namespace: "default"}, token)).To(Succeed())
		Expect(token.Spec.DataLocation).NotTo(BeEmpty())
	})

	It("removes the raw token from the object", func() {
		t := api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-token", Namespace: "default"}, &t)).To(Succeed())

		Expect(t.Spec.RawTokenData).To(BeNil())
		data, err := ITest.Vault.Get(&t)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())
		Expect(data.AccessToken).To(Equal("nazdar"))
	})

	It("allows token metadata to be updated", func() {
		// TODO implement
		Skip("Not implemented yet")
	})

	It("updates the token data with update", func() {
		t := api.SPIAccessToken{}
		Eventually(func() error {
			// we need to do this in the eventually loop because we might be stepping over each others' toes with the controller
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-token", Namespace: "default"}, &t)).To(Succeed())

			t.Spec.RawTokenData = &api.Token{
				AccessToken: "updated",
			}

			return ITest.Client.Update(ITest.Context, &t)
		}).Should(Succeed())

		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(&t), &t)).To(Succeed())

		Expect(t.Spec.RawTokenData).To(BeNil())
		data, err := ITest.Vault.Get(&t)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())
		Expect(data.AccessToken).To(Equal("updated"))
	})
})

var _ = Describe("Create without token data", func() {
	var _ = BeforeEach(func() {
		Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-token",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: api.ServiceProviderTypeGitHub,
				Permissions:         []api.Permission{api.PermissionRead},
			},
		})).To(Succeed())
	})

	var _ = AfterEach(func() {
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-token", Namespace: "default"}, token)).To(Succeed())
			g.Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
		}).Should(Succeed())
	})

	It("doesn't auto-create the token", func() {
		// TODO implement
		Skip("Not implemented yet")
	})

	It("and update with token data auto-creates the token", func() {
		// TODO implement
		Skip("Not implemented yet")
	})

	It("allows data location to be updated", func() {
		// TODO implement
		Skip("Not implemented yet")
	})

	It("allows token metadata to be updated", func() {
		// TODO implement
		Skip("Not implemented yet")
	})
})

var _ = Describe("Delete token", func() {
	It("cannot happen as long as there are active bindings", func() {
		// TODO implement
		Skip("Not implemented yet")
	})
})
