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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
	}).WithTimeout(10 * time.Second).Should(BeTrue())
}

var _ = Describe("Create binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()

		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    "default",
				GenerateName: "create-test-token",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "test-provider://acme",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		ITest.TestServiceProvider.GetOauthEndpointImpl = func() string {
			return "test-provider://acme"
		}

		createdBinding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "create-test-binding",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://acme/acme",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
	})

	AfterEach(func() {
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
	})

	It("should link the token to the binding", func() {
		testTokenNameInStatus(createdBinding, Equal(createdToken.Name))
	})

	It("should revert the updates to the linked token status", func() {
		testTokenNameInStatus(createdBinding, Equal(createdToken.Name))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my random link name"))))
	})

	It("should copy the OAuthUrl to the status and reflect the phase", func() {
		Eventually(func(g Gomega) {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			g.Expect(binding.Status.OAuthUrl).NotTo(BeEmpty())
			g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
			g.Expect(binding.Status.ErrorReason).To(BeEmpty())
			g.Expect(binding.Status.ErrorMessage).To(BeEmpty())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})
})

var _ = Describe("Update binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()

		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "update-test-token",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "test-provider://test",
			},
		}
		createdBinding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "update-test-binding",
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

	AfterEach(func() {
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
					ServiceProviderUrl: "test-provider://other",
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
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
				createdBinding.Spec.RepoUrl = "test-provider://test"
				g.Expect(ITest.Client.Update(ITest.Context, createdBinding)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(otherToken), otherToken)).To(Succeed())
				g.Expect(ITest.Client.Delete(ITest.Context, otherToken)).To(Succeed())
			}).Should(Succeed())

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
		ITest.TestServiceProvider.Reset()

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

		By("waiting for the token to get linked")
		createdToken = getLinkedToken(Default, createdBinding)

		err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
			AccessToken: "token",
		})
		Expect(err).NotTo(HaveOccurred())

		// now that the token is stored, we can simulate parsing its metadata from the SP
		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
			Username:             "alois",
			UserId:               "42",
			Scopes:               []string{},
			ServiceProviderState: []byte("state"),
		})

		Eventually(func(g Gomega) {
			By("waiting for the synced secret to appear")
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
			g.Expect(createdBinding.Status.SyncedObjectRef.Name).NotTo(BeEmpty())
			syncedSecret = &corev1.Secret{}
			g.Expect(ITest.Client.Get(ITest.Context,
				client.ObjectKey{Name: createdBinding.Status.SyncedObjectRef.Name, Namespace: createdBinding.Namespace},
				syncedSecret)).To(Succeed())
		}).WithTimeout(20 * time.Second).Should(Succeed())

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
			err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
				AccessToken:  "access",
				RefreshToken: "refresh",
				TokenType:    "awesome",
			})
			Expect(err).NotTo(HaveOccurred())

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

var _ = Describe("Status updates", func() {
	var token *api.SPIAccessToken
	var binding *api.SPIAccessTokenBinding

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()

		token = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "status-updates-",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "test-provider://",
			},
		}

		binding = &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "status-updates-",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://acme/acme",
				Secret: api.SecretSpec{
					Type: corev1.SecretTypeBasicAuth,
				},
			},
		}

		Expect(ITest.Client.Create(ITest.Context, token)).To(Succeed())
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&token)
		Expect(ITest.Client.Create(ITest.Context, binding)).To(Succeed())

		Eventually(func(g Gomega) {
			currentBinding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(binding), currentBinding)).To(Succeed())
			g.Expect(currentBinding.Status.LinkedAccessTokenName).To(Equal(token.Name))
		}).Should(Succeed())
	})

	AfterEach(func() {
		Expect(ITest.Client.Delete(ITest.Context, binding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
	})

	When("linked token is ready and secret not injected", func() {
		BeforeEach(func() {
			ITest.TestServiceProvider.LookupTokenImpl = nil
			ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
				Username:             "alois",
				UserId:               "42",
				Scopes:               []string{},
				ServiceProviderState: []byte("state"),
			})

			err := ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
				AccessToken: "access_token",
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(token), currentToken)).To(Succeed())
				g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
			}).Should(Succeed())
		})

		AfterEach(func() {
			Expect(ITest.TokenStorage.Delete(ITest.Context, token)).To(Succeed())
			ITest.TestServiceProvider.LookupTokenImpl = nil
		})

		It("should end in error phase if linked token doesn't fit the requirements", func() {
			// This happens when the OAuth flow gives fewer perms than we requested
			// I.e. we link the token, the user goes through OAuth flow, but the token we get doesn't
			// give us all the required permissions (which will manifest in it not being looked up during
			// reconciliation for given binding).

			// We have the token in the ready state... let's not look it up during token matching
			ITest.TestServiceProvider.LookupTokenImpl = nil

			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(binding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
			}).Should(Succeed())
		})
	})

	When("linked token is not ready", func() {
		// we need to use a dedicated binding for this test so that the outer levels can clean up.
		// We will be linking this binding to a different token than the outer layers expect.
		var testBinding *api.SPIAccessTokenBinding
		var betterToken *api.SPIAccessToken

		BeforeEach(func() {
			betterToken = &api.SPIAccessToken{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "status-updates-better-",
					Namespace:    "default",
					Annotations: map[string]string{
						"dummy": "true",
					},
				},
				Spec: api.SPIAccessTokenSpec{
					ServiceProviderUrl: "test-provider://",
				},
			}

			testBinding = &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "status-updates-",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					RepoUrl: "test-provider://acme/acme",
					Secret: api.SecretSpec{
						Type: corev1.SecretTypeBasicAuth,
					},
				},
			}

			Expect(ITest.Client.Create(ITest.Context, betterToken)).To(Succeed())

			// we're trying to use the token defined by the outer layer first.
			// This token is not ready, so we should be in a situation that should
			// still enable swapping the token for a better fitting one.
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&token)
			Expect(ITest.Client.Create(ITest.Context, testBinding)).To(Succeed())
		})

		AfterEach(func() {
			Expect(ITest.Client.Delete(ITest.Context, testBinding)).To(Succeed())
			Expect(ITest.Client.Delete(ITest.Context, betterToken)).To(Succeed())
		})

		It("replaces the linked token with a more precise lookup if available", func() {
			// first, let's check that we're linked to the original token
			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(testBinding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.LinkedAccessTokenName).To(Equal(token.Name))
			}).Should(Succeed())

			// now start returning the better token from the lookup
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&betterToken)

			// now simulate that betterToken got changed and has become a better match
			// since we've set up the service provider above already, we only need to
			// update the object in cluster to force reconciliation
			Eventually(func(g Gomega) {
				tkn := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(betterToken), tkn)).To(Succeed())
				tkn.Annotations["dummy"] = "more_true"
				g.Expect(ITest.Client.Update(ITest.Context, tkn)).To(Succeed())
			}).Should(Succeed())

			// and check that the binding switches to the better token
			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(testBinding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.LinkedAccessTokenName).To(Equal(betterToken.Name))
			}).Should(Succeed())
		})
	})

	When("linked token data disappears after successful sync", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			err := ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
				AccessToken: "access_token",
			})
			Expect(err).NotTo(HaveOccurred())

			ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
				Username:             "alois",
				UserId:               "42",
				Scopes:               []string{},
				ServiceProviderState: []byte("state"),
			})

			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(token), currentToken)).To(Succeed())
				g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
			}).Should(Succeed())

			currentBinding := &api.SPIAccessTokenBinding{}
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(binding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
			}).Should(Succeed())

			secret = &corev1.Secret{}
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: currentBinding.Status.SyncedObjectRef.Name, Namespace: "default"}, secret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(ITest.TokenStorage.Delete(ITest.Context, token)).To(Succeed())
			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(token), currentToken)).To(Succeed())
			}).Should(Succeed())
		})

		It("deletes the secret and flips back to awaiting phase", func() {
			ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(nil)
			Expect(ITest.TokenStorage.Delete(ITest.Context, token)).To(Succeed())

			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(binding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
				g.Expect(currentBinding.Status.SyncedObjectRef.Name).To(BeEmpty())

				err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(secret), &corev1.Secret{})
				g.Expect(err).To(HaveOccurred())
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
	})
})
