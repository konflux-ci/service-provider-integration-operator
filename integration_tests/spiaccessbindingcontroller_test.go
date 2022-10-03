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
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"

	apiexv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
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
			g.Expect(binding.Labels[controllers.SPIAccessTokenLinkLabel]).Should(linkMatcher)

		return cond
	}).Should(BeTrue())
}

func createStandardPair(namePrefix string) (*api.SPIAccessTokenBinding, *api.SPIAccessToken) {

	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	ITest.TestServiceProvider.Reset()
	createdBinding = &api.SPIAccessTokenBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix + "-binding-",
			Namespace:    "default",
		},
		Spec: api.SPIAccessTokenBindingSpec{
			RepoUrl: "test-provider://test",
		},
	}
	Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
	Eventually(func(g Gomega) {
		binding := &api.SPIAccessTokenBinding{}
		g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
		g.Expect(binding.Status.LinkedAccessTokenName).NotTo(BeEmpty())

		createdBinding = binding
	}).Should(Succeed())
	Eventually(func(g Gomega) {
		createdToken = &api.SPIAccessToken{}
		err := ITest.Client.Get(ITest.Context, client.ObjectKey{Name: createdBinding.Status.LinkedAccessTokenName, Namespace: createdBinding.Namespace}, createdToken)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(createdToken).NotTo(BeNil())
		g.Expect(createdToken.Status.Phase).NotTo(BeEmpty())
	}).Should(Succeed())
	return createdBinding, createdToken
}

var _ = Describe("Create binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("create-test")
		ITest.TestServiceProvider.GetOauthEndpointImpl = func() string {
			return "test-provider://acme"
		}
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		//dummy update to cause reconciliation
		Eventually(func(g Gomega) {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			binding.Annotations = map[string]string{"foo": "bar"}
			g.Expect(ITest.Client.Update(ITest.Context, binding)).To(Succeed())
		}).Should(Succeed())
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
	})

	It("registering the finalizers", func() {
		Eventually(func(g Gomega) {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			g.Expect(binding.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-secrets"))
		}).Should(Succeed())
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

	It("have the upload URL set", func() {
		Eventually(func(g Gomega) {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			g.Expect(strings.HasSuffix(binding.Status.UploadUrl, "/token/"+createdToken.Namespace+"/"+createdToken.Name)).To(BeTrue())
		}).Should(Succeed())
	})
})

var _ = Describe("Update binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("update-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
	})

	It("reverts updates to the linked token label", func() {
		testTokenNameInStatus(createdBinding, Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			binding.Labels[controllers.SPIAccessTokenLinkLabel] = "my_random_link_name"
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
				g.Expect(createdBinding.Status.OAuthUrl).To(Equal(otherToken.Status.OAuthUrl))
				g.Expect(createdBinding.Status.UploadUrl).To(Equal(otherToken.Status.UploadUrl))
			}).Should(Succeed())
		})
	})
})

var _ = Describe("Delete binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken
	var syncedSecret *corev1.Secret

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("delete-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

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
			currentBinding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), currentBinding)).To(Succeed())
			g.Expect(currentBinding.Status.SyncedObjectRef.Name).NotTo(BeEmpty())
			syncedSecret = &corev1.Secret{}
			g.Expect(ITest.Client.Get(ITest.Context,
				client.ObjectKey{Name: currentBinding.Status.SyncedObjectRef.Name, Namespace: currentBinding.Namespace},
				syncedSecret)).To(Succeed())
		}).WithTimeout(20 * time.Second).Should(Succeed())

	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
	})

	It("should delete the synced secret", func() {
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		// and check that secret eventually disappeared
		Eventually(func(g Gomega) {
			err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(syncedSecret), &corev1.Secret{})
			if errors.IsNotFound(err) {
				return
			}
		}).Should(Succeed())
	})

	It("should delete binding by timeout", func() {
		orig := ITest.OperatorConfiguration.AccessTokenBindingTtl
		ITest.OperatorConfiguration.AccessTokenBindingTtl = 500 * time.Millisecond
		defer func() {
			ITest.OperatorConfiguration.AccessTokenBindingTtl = orig
		}()

		// and check that binding eventually disappeared
		Eventually(func(g Gomega) {
			err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), &api.SPIAccessToken{})
			if errors.IsNotFound(err) {
				return
			} else {
				//force reconciliation timeout is passed
				binding := &api.SPIAccessTokenBinding{}
				ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)
				binding.Annotations = map[string]string{"foo": "bar"}
				ITest.Client.Update(ITest.Context, binding)
			}
		}).Should(Succeed())
	})
})

var _ = Describe("Syncing", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("sync-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		createdBinding.Spec.Secret = api.SecretSpec{
			Name: "binding-secret",
			Type: corev1.SecretTypeBasicAuth,
		}
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
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

		It("keeps the secret data valid", func() {
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

			By("changing secret data")
			Eventually(func(g Gomega) {
				secret := &corev1.Secret{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: createdBinding.Status.SyncedObjectRef.Name, Namespace: createdBinding.Namespace}, secret)).To(Succeed())
				secret.Data["password"] = []byte("wrong")
				g.Expect(ITest.Client.Update(ITest.Context, secret)).To(Succeed())
			})

			By("waiting for the secret data to be reverted back to the correct values")
			Eventually(func(g Gomega) {
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
	var createdToken *api.SPIAccessToken
	var createdBinding *api.SPIAccessTokenBinding

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("status-updates-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
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

			err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
				AccessToken: "access_token",
			})
			Expect(err).NotTo(HaveOccurred())

			//force reconciliation
			Eventually(func(g Gomega) {
				token := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
				token.Annotations = map[string]string{"foo": "bar"}
				g.Expect(ITest.Client.Update(ITest.Context, token)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
				g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
			}).Should(Succeed())
		})

		AfterEach(func() {
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
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), currentBinding)).To(Succeed())
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
			ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
				Username:             "alois",
				UserId:               "42",
				Scopes:               []string{},
				ServiceProviderState: []byte("state"),
			})

			err := ITest.TokenStorage.Store(ITest.Context, betterToken, &api.Token{
				AccessToken: "access_token",
			})
			Expect(err).NotTo(HaveOccurred())

			// we're trying to use the token defined by the outer layer first.
			// This token is not ready, so we should be in a situation that should
			// still enable swapping the token for a better fitting one.
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
			Expect(ITest.Client.Create(ITest.Context, testBinding)).To(Succeed())
		})

		AfterEach(func() {
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		})

		It("replaces the linked token with a more precise lookup if available", func() {
			// first, let's check that we're linked to the original token
			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(testBinding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.LinkedAccessTokenName).To(Equal(createdToken.Name))
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
			err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
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
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
				g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
			}).Should(Succeed())

			currentBinding := &api.SPIAccessTokenBinding{}
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
			}).Should(Succeed())

			secret = &corev1.Secret{}
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: currentBinding.Status.SyncedObjectRef.Name, Namespace: "default"}, secret)).To(Succeed())
		})

		It("deletes the secret and flips back to awaiting phase", func() {
			ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(nil)
			Expect(ITest.TokenStorage.Delete(ITest.Context, createdToken)).To(Succeed())

			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), currentBinding)).To(Succeed())
				g.Expect(currentBinding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
				g.Expect(currentBinding.Status.SyncedObjectRef.Name).To(BeEmpty())

				err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(secret), &corev1.Secret{})
				g.Expect(err).To(HaveOccurred())
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
	})

	When("binding requires invalid scopes", func() {
		It("should flip to error state", func() {
			ITest.TestServiceProvider.ValidateImpl = func(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
				return serviceprovider.ValidationResult{
					ScopeValidation: []error{stderrors.New("nope")},
				}, nil
			}

			// update the binding to force reconciliation after we change the impl of the validation
			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), currentBinding)).To(Succeed())
				currentBinding.Annotations = map[string]string{"just-an-annotation": "to force reconciliation"}
				g.Expect(ITest.Client.Update(ITest.Context, currentBinding)).To(Succeed())
			}).Should(Succeed())

			// now check that the binding flipped to the error state
			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), currentBinding)).To(Succeed())

				g.Expect(currentBinding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
				g.Expect(currentBinding.Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorReasonUnsupportedPermissions))
			})
		})
	})

	When("service provider url is invalid", func() {
		It("should end in error phase and have an error message", func() {
			createdBinding = &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "invalid-binding-",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					RepoUrl: "invalid://abc./name/repo",
				},
			}
			previousBaseImpl := ITest.HostCredsServiceProvider.GetBaseUrlImpl
			ITest.HostCredsServiceProvider.GetBaseUrlImpl = func() string {
				return "invalid://abc."
			}
			Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())

			//dummy update to cause reconciliation
			Eventually(func(g Gomega) {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
				binding.Annotations = map[string]string{"foo": "bar"}
				g.Expect(ITest.Client.Update(ITest.Context, binding)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
				g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
				g.Expect(binding.Status.ErrorMessage).To(Not(BeEmpty()))
				g.Expect(binding.Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType))
				g.Expect(binding.Status.LinkedAccessTokenName).To(BeEmpty())
			}).Should(Succeed())
			ITest.HostCredsServiceProvider.GetBaseUrlImpl = previousBaseImpl
		})
	})

	When("linking fails", func() {
		// This simulates a situation where the CRDs and the code is out-of-sync and any updates to the binding status
		// fail.
		origCRD := &apiexv1.CustomResourceDefinition{}
		testBinding := &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "link-failing-",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl: "test-provider://acme/acme",
				Secret: api.SecretSpec{
					Type: corev1.SecretTypeBasicAuth,
				},
			},
		}

		BeforeEach(func() {
			// we need to modify the cluster somehow so that updating the object status fails
			// we will add a required property to the status schema so that the status update fails
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "spiaccesstokenbindings.appstudio.redhat.com"}, origCRD)).To(Succeed())
				updatedCRD := origCRD.DeepCopy()
				status := updatedCRD.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"]
				status.Properties["__test"] = apiexv1.JSONSchemaProps{
					Type: "string",
				}
				requiredStatusProps := status.Required
				requiredStatusProps = append(requiredStatusProps, "__test")
				status.Required = requiredStatusProps
				updatedCRD.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"] = status

				g.Expect(ITest.Client.Update(ITest.Context, updatedCRD)).To(Succeed())
			}).Should(Succeed())

			ITest.TestServiceProvider.LookupTokenImpl = nil
			Expect(ITest.Client.Create(ITest.Context, testBinding)).Should(Succeed())
		})

		AfterEach(func() {
			// restore the CRD into its original state
			Eventually(func(g Gomega) {
				currentCRD := &apiexv1.CustomResourceDefinition{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(origCRD), currentCRD)).To(Succeed())
				currentCRD.Spec = origCRD.Spec
				g.Expect(ITest.Client.Update(ITest.Context, currentCRD)).To(Succeed())
			}).Should(Succeed())
		})

		It("should not create a token", func() {
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
				g.Expect(createdBinding.Status.LinkedAccessTokenName).To(Equal(createdToken.Name))
			}).Should(Succeed())

			Consistently(func(g Gomega) {
				tokens := &api.SPIAccessTokenList{}
				g.Expect(ITest.Client.List(ITest.Context, tokens)).Should(Succeed())
				// there should only be 1 token (the one created in the outer level). The change to the CRD makes every
				// attempt to create a new token and link it fail and clean up the freshly created token.
				// Because of the errors, we clean up but are left in a perpetual cycle of trying to create the linked
				// token and failing to link it and thus the new tokens are continuously appearing and disappearing.
				// Let's just check here that their number is not growing too much too quickly by this crude measure.
				g.Expect(len(tokens.Items)).To(BeNumerically("<", 5))
			}, "10s").Should(Succeed())
		})
	})
})
