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

var _ = Describe("Token lookup", func() {
	AfterEach(func() {
		bindings := &api.SPIAccessTokenBindingList{}
		Expect(ITest.Client.List(ITest.Context, bindings)).To(Succeed())

		for _, b := range bindings.Items {
			Expect(ITest.Client.Delete(ITest.Context, &b)).To(Succeed())
		}

		tokens := &api.SPIAccessTokenList{}
		Expect(ITest.Client.List(ITest.Context, tokens)).To(Succeed())

		for _, t := range tokens.Items {
			Expect(ITest.Client.Delete(ITest.Context, &t)).To(Succeed())
		}
	})

	Context("on GitHub", func() {
		Context("token based on SP URL", func() {
			BeforeEach(func() {
				Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessToken{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "token",
						Namespace: "default",
					},
					Spec: api.SPIAccessTokenSpec{
						ServiceProviderType: api.ServiceProviderTypeGitHub,
						ServiceProviderUrl:  "https://github.com",
						DataLocation:        "/over/the/rainbow",
						Permissions:         []api.Permission{},
					},
				})).To(Succeed())

				Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessTokenBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "matching",
						Namespace: "default",
					},
					Spec: api.SPIAccessTokenBindingSpec{
						RepoUrl:     "https://github.com/acme/acme",
						Permissions: []api.Permission{},
						Secret:      api.SecretSpec{},
					},
				})).To(Succeed())

				Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessTokenBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "not-matching",
						Namespace: "default",
					},
					Spec: api.SPIAccessTokenBindingSpec{
						RepoUrl:     "https://gitlab.com/somewhere-else",
						Permissions: []api.Permission{},
						Secret:      api.SecretSpec{},
					},
				})).To(Succeed())
			})

			It("is linked when matches", func() {
				Eventually(func(g Gomega) {
					binding := &api.SPIAccessTokenBinding{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "matching", Namespace: "default"}, binding)).To(Succeed())
					linkedToken := binding.Status.LinkedAccessTokenName
					g.Expect(linkedToken).To(Equal("token"))
				}).Should(Succeed())
			})

			It("is not linked when it doesn't match", func() {
				Eventually(func(g Gomega) {
					binding := &api.SPIAccessTokenBinding{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "not-matching", Namespace: "default"}, binding)).To(Succeed())
					linkedToken := binding.Status.LinkedAccessTokenName
					g.Expect(linkedToken).NotTo(Equal("token"))
				}).Should(Succeed())
			})
		})

		Context("finds token based on permissions", func() {
			BeforeEach(func() {

			})

			It("works", func() {
				Skip("Not implemented")
			})
		})

		Context("finds token based on scopes", func() {
			BeforeEach(func() {
				Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessToken{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "token",
						Namespace: "default",
					},
					Spec: api.SPIAccessTokenSpec{
						ServiceProviderType: api.ServiceProviderTypeGitHub,
						ServiceProviderUrl:  "https://github.com",
						DataLocation:        "/over/the/rainbow",
						Permissions:         []api.Permission{},
						TokenMetadata: &api.TokenMetadata{
							Scopes: []string{"read", "hug"},
						},
					},
				})).To(Succeed())

				Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessTokenBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "matching",
						Namespace: "default",
					},
					Spec: api.SPIAccessTokenBindingSpec{
						RepoUrl:     "https://github.com/acme/acme",
						Permissions: []api.Permission{},
						Secret:      api.SecretSpec{},
						Scopes:      []string{"read"},
					},
				})).To(Succeed())

				Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessTokenBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "not-matching",
						Namespace: "default",
					},
					Spec: api.SPIAccessTokenBindingSpec{
						RepoUrl:     "https://gitlab.com/somewhere-else",
						Permissions: []api.Permission{},
						Secret:      api.SecretSpec{},
						Scopes:      []string{"write"},
					},
				})).To(Succeed())
			})

			It("is linked when matches", func() {
				Eventually(func(g Gomega) {
					binding := &api.SPIAccessTokenBinding{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "matching", Namespace: "default"}, binding)).To(Succeed())
					linkedToken := binding.Status.LinkedAccessTokenName
					g.Expect(linkedToken).To(Equal("token"))
				}).Should(Succeed())
			})

			It("is not linked when it doesn't match", func() {
				Eventually(func(g Gomega) {
					binding := &api.SPIAccessTokenBinding{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "not-matching", Namespace: "default"}, binding)).To(Succeed())
					linkedToken := binding.Status.LinkedAccessTokenName
					g.Expect(linkedToken).NotTo(Equal("token"))
				}).Should(Succeed())
			})
		})
	})

	Context("on Quay", func() {
	})
})
