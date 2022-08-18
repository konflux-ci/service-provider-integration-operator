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

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"
	apiexv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	"k8s.io/apimachinery/pkg/api/errors"

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
	}).Should(BeTrue())
}

func createStandardPair(namePrefix string) (*api.SPIAccessTokenBinding, *api.SPIAccessToken) {

	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

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
		if binding.Status.LinkedAccessTokenName == "" {
			log.Log.Info("binding still without a linked token while creating the standard pair", "binding", binding)
		}
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
		log.Log.Info("Create binding [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("create-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
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
		log.Log.Info("<----- Create binding [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Create binding [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Create binding [AfterEach]")
	})

	It("should link the token to the binding", func() {
		log.Log.Info("Create binding should link the token to the binding ----->")
		testTokenNameInStatus(createdBinding, Equal(createdToken.Name))
		log.Log.Info("<----- Create binding should link the token to the binding")
	})

	It("should revert the updates to the linked token status", func() {
		log.Log.Info("Create binding should revert the updates to the linked token status ----->")
		testTokenNameInStatus(createdBinding, Equal(createdToken.Name))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my random link name"))))
		log.Log.Info("<----- Create binding should revert the updates to the linked token status")
	})

	It("should copy the OAuthUrl to the status and reflect the phase", func() {
		log.Log.Info("Create binding should copy the OAuthUrl to the status and reflect the phase ----->")
		Eventually(func(g Gomega) {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			g.Expect(binding.Status.OAuthUrl).NotTo(BeEmpty())
			g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
			g.Expect(binding.Status.ErrorReason).To(BeEmpty())
			g.Expect(binding.Status.ErrorMessage).To(BeEmpty())
		}).WithTimeout(10 * time.Second).Should(Succeed())
		log.Log.Info("<----- Create binding should copy the OAuthUrl to the status and reflect the phase")
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
		log.Log.Info("Update binding [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("update-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		log.Log.Info("<----- Update binding [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Update binding [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Update binding [AfterEach]")
	})

	It("reverts updates to the linked token label", func() {
		log.Log.Info("Update binding reverts updates to the linked token label ----->")
		testTokenNameInStatus(createdBinding, Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
			binding.Labels[config.SPIAccessTokenLinkLabel] = "my_random_link_name"
			return ITest.Client.Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my_random_link_name"))))
		log.Log.Info("<----- Update binding reverts updates to the linked token label")
	})

	It("reverts updates to the linked token in the status", func() {
		log.Log.Info("Update binding reverts updates to the linked token in the status ----->")
		testTokenNameInStatus(createdBinding, Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(createdBinding, Not(Or(BeEmpty(), Equal("my random link name"))))
		log.Log.Info("<----- Update binding reverts updates to the linked token in the status")
	})

	When("lookup changes the token", func() {
		var otherToken *api.SPIAccessToken

		BeforeEach(func() {
			log.Log.Info("Update binding when lookup changes the token [BeforeEach] ----->")
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

			ITest.TestServiceProvider.LookupTokenImpl = func(ctx context.Context, c client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
				if strings.HasSuffix(binding.Spec.RepoUrl, "test") {
					return []api.SPIAccessToken{*createdToken}, nil
				} else if strings.HasSuffix(binding.Spec.RepoUrl, "other") {
					return []api.SPIAccessToken{*otherToken}, nil
				} else {
					return nil, fmt.Errorf("request for invalid test token")
				}
			}
			log.Log.Info("<----- Update binding when lookup changes the token [BeforeEach]")
		})

		AfterEach(func() {
			log.Log.Info("Update binding when lookup changes the token [AfterEach] ----->")
			ITest.TestServiceProvider.Reset()
			log.Log.Info("<----- Update binding when lookup changes the token [AfterEach]")
		})

		It("changes the linked token, too", func() {
			log.Log.Info("Update binding when lookup changes the token changes the linked token, too ----->")
			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())

				createdBinding.Spec.RepoUrl = "test-provider://other"
				g.Expect(ITest.Client.Update(ITest.Context, createdBinding)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
				g.Expect(createdBinding.Status.LinkedAccessTokenName).To(Equal(otherToken.Name))
			}).Should(Succeed())
			log.Log.Info("<----- Update binding when lookup changes the token changes the linked token, too")
		})
	})
})

var _ = Describe("Delete binding", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken
	var syncedSecret *corev1.Secret

	BeforeEach(func() {
		log.Log.Info("Delete binding [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		createdBinding, createdToken = createStandardPair("delete-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))

		// Now that we have all the above setup, we can store the token data, which should trigger repeated reconciliation
		// of the token and in turn also of the binding.
		err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
			AccessToken: "token",
		})
		Expect(err).NotTo(HaveOccurred())

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
		log.Log.Info("<----- Delete binding [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Delete binding [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Delete binding [AfterEach]")
	})

	It("should delete the synced secret", func() {
		log.Log.Info("Delete binding should delete the synced secret ----->")
		// Note that automatic cleanup of owned objects doesn't seem to work in testenv, so we're just checking here
		// that the secret has its owner reference set correctly and actually try to delete it ourselves here in this
		// test.
		Expect(syncedSecret.OwnerReferences).NotTo(BeEmpty())
		Expect(syncedSecret.OwnerReferences[0].UID).To(Equal(createdBinding.UID))

		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, syncedSecret)).To(Succeed())
		log.Log.Info("<----- Delete binding should delete the synced secret")
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

	It("should delete the synced token in awaiting state", func() {
		log.Log.Info("Delete binding should delete the synced token in awaiting state ----->")
		Eventually(func(g Gomega) bool {
			return time.Now().Sub(createdBinding.CreationTimestamp.Time).Seconds() > controllers.NoLinkingBindingGracePeriodSeconds+1
		}).WithTimeout(controllers.GracePeriodSeconds * 10 * time.Second).Should(BeTrue())
		//flip back to awaiting
		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(nil)

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
		log.Log.Info("<----- Delete binding should delete the synced token in awaiting state")
	})
})

var _ = Describe("Syncing", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		log.Log.Info("Syncing [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("sync-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		createdBinding.Spec.Secret = api.SecretSpec{
			Name: "binding-secret",
			Type: corev1.SecretTypeBasicAuth,
		}
		log.Log.Info("<----- Syncing [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Syncing [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Syncing [AfterEach]")
	})

	When("token is ready", func() {
		It("creates the secret with the data", func() {
			log.Log.Info("Syncing when token is ready creates the secret with the data ----->")
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
			log.Log.Info("<----- Syncing when token is ready creates the secret with the data")
		})
	})

	When("token is not ready", func() {
		It("doesn't create secret", func() {
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())
			Expect(createdBinding.Status.SyncedObjectRef.Name).To(BeEmpty())
			log.Log.Info("<----- Syncing when token is not ready doesn't create secret")
		})
	})
})

var _ = Describe("Status updates", func() {
	var createdToken *api.SPIAccessToken
	var createdBinding *api.SPIAccessTokenBinding

	BeforeEach(func() {
		log.Log.Info("Status updates [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		createdBinding, createdToken = createStandardPair("status-updates-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
		log.Log.Info("<----- Status updates [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Status updates [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Status updates [AfterEach]")
	})

	When("linked token is ready and secret not injected", func() {
		BeforeEach(func() {
			log.Log.Info("Status updates when linked token is ready and secret not injected [BeforeEach] ----->")
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
			log.Log.Info("<----- Status updates when linked token is ready and secret not injected [BeforeEach]")
		})

		AfterEach(func() {
			log.Log.Info("Status updates when linked token is ready and secret not injected [AfterEach] ----->")
			ITest.TestServiceProvider.LookupTokenImpl = nil
			log.Log.Info("<----- Status updates when linked token is ready and secret not injected [AfterEach]")
		})

		It("should end in error phase if linked token doesn't fit the requirements", func() {
			log.Log.Info("Status updates when linked token is ready and secret not injected should end in error phase if linked token doesn't fit the requirements ----->")
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
			log.Log.Info("<----- Status updates when linked token is ready and secret not injected should end in error phase if linked token doesn't fit the requirements")
		})
	})

	When("linked token is not ready", func() {
		// we need to use a dedicated binding for this test so that the outer levels can clean up.
		// We will be linking this binding to a different token than the outer layers expect.
		var testBinding *api.SPIAccessTokenBinding
		var betterToken *api.SPIAccessToken

		BeforeEach(func() {
			log.Log.Info("Status updates when linked token is not ready [BeforeEach] ----->")
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
			log.Log.Info("<----- Status updates when linked token is not ready [BeforeEach]")
		})

		AfterEach(func() {
			log.Log.Info("Status updates when linked token is not ready [AfterEach] ----->")
			ITest.TestServiceProvider.LookupTokenImpl = nil
			Eventually(func(g Gomega) {
				currentBinding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(testBinding), currentBinding)).To(Succeed())
				g.Expect(ITest.Client.Delete(ITest.Context, currentBinding)).To(Succeed())
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(betterToken), currentToken)).To(Succeed())
				g.Expect(ITest.Client.Delete(ITest.Context, currentToken)).To(Succeed())
			}).Should(Succeed())
			log.Log.Info("<----- Status updates when linked token is not ready [AfterEach]")
		})

		It("replaces the linked token with a more precise lookup if available", func() {
			log.Log.Info("Status updates when linked token is not ready replaces the linked token with a more precise lookup if available ----->")
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
			log.Log.Info("<----- Status updates when linked token is not ready replaces the linked token with a more precise lookup if available")
		})
	})

	When("linked token data disappears after successful sync", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			log.Log.Info("Status updates when linked token data disappears after successful sync [BeforeEach] ----->")
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
			log.Log.Info("<----- Status updates when linked token data disappears after successful sync [BeforeEach]")
		})

		It("deletes the secret and flips back to awaiting phase", func() {
			log.Log.Info("Status updates when linked token data disappears after successful sync deletes the secret and flips back to awaiting phase ----->")
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
			log.Log.Info("<----- Status updates when linked token data disappears after successful sync deletes the secret and flips back to awaiting phase")
		})
	})

	When("binding requires invalid scopes", func() {
		It("should flip to error state", func() {
			log.Log.Info("Status updates when binding requires invalid scopes should flip to error state ----->")
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
			log.Log.Info("<----- Status updates when binding requires invalid scopes should flip to error state")
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
			log.Log.Info("Status updates when linking fails [BeforeEach] ----->")
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
			log.Log.Info("<----- Status updates when linking fails [BeforeEach]")
		})

		AfterEach(func() {
			log.Log.Info("Status updates when linking fails [AfterEach] ----->")
			// restore the CRD into its original state
			Eventually(func(g Gomega) {
				currentCRD := &apiexv1.CustomResourceDefinition{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(origCRD), currentCRD)).To(Succeed())
				currentCRD.Spec = origCRD.Spec
				g.Expect(ITest.Client.Update(ITest.Context, currentCRD)).To(Succeed())
			}).Should(Succeed())
			log.Log.Info("<----- Status updates when linking fails [BeforeEach]")
		})

		It("should not create a token", func() {
			log.Log.Info("Status updates when linking fails should not create a token ----->")
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
			log.Log.Info("<----- Status updates when linking fails should not create a token")
		})
	})
})
