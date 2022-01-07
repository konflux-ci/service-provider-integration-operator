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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// a helper function to test that the controller eventually updates the object such that the linked token name
// linked in the binding matches the provided criteria.
func testTokenNameInStatus(createdBinding *api.SPIAccessTokenBinding, linkMatcher OmegaMatcher) {
	Eventually(func(g Gomega) bool {
		binding := &api.SPIAccessTokenBinding{}
		g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

		cond := g.Expect(binding.Status.LinkedAccessTokenName).Should(linkMatcher) &&
			g.Expect(binding.Labels[config.SPIAccessTokenLinkLabel]).Should(linkMatcher)

		return cond
	}, 10).Should(BeTrue())
}

var _ = Describe("Create binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	var _ = BeforeEach(func() {
		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    "default",
				GenerateName: "test-token",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: "TestServiceProvider",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		createdBinding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-binding",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://acme/acme",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("should link the token to the binding", func() {
		testTokenNameInStatus(createdBinding, Equal(createdToken.Name))
	})

	It("updates to the linked token status should be reverted", func() {
		testTokenNameInStatus(createdBinding, Equal(createdToken.Name))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my random link name"))))
	})
})

var _ = Describe("Update binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	var _ = BeforeEach(func() {
		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: "TestServiceProvider",
				ServiceProviderUrl:  "test-provider://test",
			},
		}
		createdBinding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-binding",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://test",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("reverts updates to the linked token label", func() {
		testTokenNameInStatus(createdBinding, Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			binding.Labels[config.SPIAccessTokenLinkLabel] = "my_random_link_name"
			return ITest.Client.Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my_random_link_name"))))
	})

	It("reverts updates to the linked token in the status", func() {
		testTokenNameInStatus(createdBinding, Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my random link name"))))
	})

	When("lookup changes the token", func() {
		var otherToken *api.SPIAccessToken

		BeforeEach(func() {
			otherToken = &api.SPIAccessToken{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "other-test-token",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenSpec{
					ServiceProviderUrl:  "test-provider://other",
					ServiceProviderType: "TestServiceProvider",
				},
			}

			Expect(ITest.Client.Create(ITest.Context, otherToken)).To(Succeed())

			ITest.TestServiceProvider.LookupTokenImpl = func(ctx context.Context, c client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
				if strings.HasSuffix(binding.Spec.RepoUrl, "test") {
					return createdToken, nil
				} else if strings.HasSuffix(binding.Spec.RepoUrl, "other") {
					return otherToken, nil
				} else {
					return nil, fmt.Errorf("request for invalid test token")
				}
			}
		})

		AfterEach(func() {
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
			createdBinding.Spec.RepoUrl = "test-provider://test"
			Expect(ITest.Client.Update(ITest.Context, createdBinding)).To(Succeed())

			Expect(ITest.Client.Delete(ITest.Context, otherToken)).To(Succeed())

			ITest.TestServiceProvider.Reset()
		})

		It("changes the linked token, too", func() {
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())

				createdBinding.Spec.RepoUrl = "test-provider://other"
				g.Expect(ITest.Client.Update(ITest.Context, createdBinding)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
				g.Expect(createdBinding.Status.LinkedAccessTokenName).To(Equal(otherToken.Name))
			}).Should(Succeed())
		})
	})
})

var _ = Describe("Delete binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken
	var syncedSecret *corev1.Secret
	var bindingDeleted bool

	BeforeEach(func() {
		createdBinding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-binding",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://acme/acme",
				Secret: api.SecretSpec{
					Type: corev1.SecretTypeBasicAuth,
				},
			},
		}

		By("creating binding")
		Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())

		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		Eventually(func(g Gomega) {
			By("waiting for the token to get linked")
			createdToken = getLinkedToken(g, createdBinding)

			createdToken.Spec.RawTokenData = &api.Token{
				AccessToken: "token",
			}

			By("updating the token with data")
			g.Expect(ITest.Client.Update(ITest.Context, createdToken)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			By("waiting for the synced secret to appear")
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
			g.Expect(createdBinding.Status.SyncedObjectRef.Name).NotTo(BeEmpty())
			syncedSecret = &corev1.Secret{}
			g.Expect(ITest.Client.Get(ITest.Context,
				client.ObjectKey{Name: createdBinding.Status.SyncedObjectRef.Name, Namespace: createdBinding.Namespace},
				syncedSecret)).To(Succeed())
		}).Should(Succeed())

		bindingDeleted = false
	})

	AfterEach(func() {
		ITest.TestServiceProvider.Reset()
		if !bindingDeleted {
			Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		}
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("should delete the synced secret", func() {
		// Note that automatic cleanup of owned objects doesn't seem to work in testenv, so we're just checking here
		// that the secret has its owner reference set correctly and actually try to delete it ourselves here in this
		// test.
		Expect(syncedSecret.OwnerReferences).NotTo(BeEmpty())
		Expect(syncedSecret.OwnerReferences[0].UID).To(Equal(createdBinding.UID))

		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		bindingDeleted = true

		Expect(ITest.Client.Delete(ITest.Context, syncedSecret)).To(Succeed())
	})
})

var _ = Describe("Syncing", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: "TestServiceProvider",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		createdBinding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-binding",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://acme/acme",
				Secret: api.SecretSpec{
					Name: "binding-secret",
					Type: corev1.SecretTypeBasicAuth,
				},
			},
		}

		Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
	})

	AfterEach(func() {
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	When("token is ready", func() {
		It("creates the secret with the data", func() {
			By("checking there is no secret")
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
			Expect(createdBinding.Status.SyncedObjectRef.Name).To(BeEmpty())

			By("updating the token")
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), createdToken)).To(Succeed())
				createdToken.Spec.RawTokenData = &api.Token{
					AccessToken:  "access",
					RefreshToken: "refresh",
					TokenType:    "awesome",
				}
				g.Expect(ITest.Client.Update(ITest.Context, createdToken)).To(Succeed())
			}).Should(Succeed())

			By("waiting for the secret to be mentioned in the binding status")
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
				g.Expect(createdBinding.Status.SyncedObjectRef.Name).To(Equal("binding-secret"))

				secret := &corev1.Secret{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: createdBinding.Status.SyncedObjectRef.Name, Namespace: createdBinding.Namespace}, secret)).To(Succeed())
				g.Expect(string(secret.Data["password"])).To(Equal("access"))
			})
		})
	})

	When("token is not ready", func() {
		It("doesn't create secret", func() {
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
			Expect(createdBinding.Status.SyncedObjectRef.Name).To(BeEmpty())
		})
	})
})
