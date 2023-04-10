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
	"net/http"
	"strings"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"

	apiexv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func checkTokenNameInStatus(g Gomega, binding *api.SPIAccessTokenBinding, linkMatcher OmegaMatcher) {
	g.Expect(binding.Status.LinkedAccessTokenName).Should(linkMatcher)
	g.Expect(binding.Labels[controllers.SPIAccessTokenLinkLabel]).Should(linkMatcher)
}

var _ = Describe("SPIAccessTokenBinding", func() {
	Describe("Create binding", func() {
		var createdBinding *api.SPIAccessTokenBinding
		var createdToken *api.SPIAccessToken

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{
					StandardTestBinding("create-test"),
				},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.OAuthCapability = func() serviceprovider.OAuthCapability {
						return &ITest.Capabilities
					}
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			createdBinding = testSetup.InCluster.Bindings[0]
			createdToken = testSetup.InCluster.Tokens[0]
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("registering the finalizers", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.Bindings[0]
				g.Expect(binding.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-objects"))
			})
		})

		It("should link the token to the binding", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.Bindings[0]
				checkTokenNameInStatus(g, binding, Equal(createdToken.Name))
			})
		})

		It("should revert the updates to the linked token status", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.Bindings[0]
				checkTokenNameInStatus(g, binding, Equal(createdToken.Name))
			})

			Eventually(func(g Gomega) error {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

				binding.Status.LinkedAccessTokenName = "my random link name"
				return ITest.Client.Status().Update(ITest.Context, binding)
			}).Should(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.Bindings[0]
				checkTokenNameInStatus(g, binding, Not(Or(BeEmpty(), Equal("my random link name"))))
			})
		})

		It("should copy the OAuthUrl to the status and reflect the phase", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.Bindings[0]
				g.Expect(binding.Status.OAuthUrl).NotTo(BeEmpty())
				g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
				g.Expect(binding.Status.ErrorReason).To(BeEmpty())

				g.Expect(binding.Status.ErrorMessage).To(BeEmpty())
			})
		})

		It("have the upload URL set", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.Bindings[0]
				g.Expect(strings.HasSuffix(binding.Status.UploadUrl, "/token/"+createdToken.Namespace+"/"+createdToken.Name)).To(BeTrue())
			})
		})

		It("adds https scheme to repoUrl in binding", func() {
			createdBinding = &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "scheme-less-binding-",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					RepoUrl: "test",
				},
			}
			Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(createdBinding))
				g.Expect(binding.Spec.RepoUrl).To(Equal("https://test"))
				g.Expect(binding.Status.ErrorMessage).To(BeEmpty())
				g.Expect(binding.Status.ErrorReason).To(BeEmpty())
			})
		})

		It("results in an error due to invalid repoUrl", func() {
			createdBinding = &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "invalid-repourl-binding-",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					RepoUrl: ":://test",
				},
			}
			Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(createdBinding))
				g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
				g.Expect(binding.Status.ErrorMessage).To(Not(BeEmpty()))
				g.Expect(binding.Status.ErrorReason).To(Not(BeEmpty()))
			})
		})
	})

	Describe("Update binding", func() {
		var createdBinding *api.SPIAccessTokenBinding
		var createdToken *api.SPIAccessToken

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{
					StandardTestBinding("update-test"),
				},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					token := objects.Tokens[0]
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&token)
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			createdBinding = testSetup.InCluster.Bindings[0]
			createdToken = testSetup.InCluster.Tokens[0]
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("migrates old quay permission areas to new ones", func() {
			ITest.TestServiceProvider.GetTypeImpl = func() config.ServiceProviderType {
				return config.ServiceProviderTypeQuay
			}
			createdBinding = &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "invalid-quay-binding-",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					Permissions: api.Permissions{
						Required: []api.Permission{{
							Area: api.PermissionAreaRepository,
						}, {
							Area: api.PermissionAreaRepositoryMetadata,
						}},
					},
					RepoUrl: "test-provider://test",
				},
			}
			Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())
			testSetup.ReconcileWithCluster(func(g Gomega) {
				binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(createdBinding))
				g.Expect(binding.Spec.Permissions.Required[0].Area).To(Equal(api.PermissionAreaRegistry))
				g.Expect(binding.Spec.Permissions.Required[1].Area).To(Equal(api.PermissionAreaRegistryMetadata))
				g.Expect(binding.Status.ErrorMessage).To(BeEmpty())
				g.Expect(binding.Status.ErrorReason).To(BeEmpty())
			})
		})

		It("reverts updates to the linked token label", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				checkTokenNameInStatus(g, testSetup.InCluster.Bindings[0], Not(BeEmpty()))
			})

			Eventually(func(g Gomega) error {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
				binding.Labels[controllers.SPIAccessTokenLinkLabel] = "my_random_link_name"
				return ITest.Client.Update(ITest.Context, binding)
			}).Should(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				checkTokenNameInStatus(g, testSetup.InCluster.Bindings[0], Not(Or(BeEmpty(), Equal("my_random_link_name"))))
			})
		})

		It("reverts updates to the linked token in the status", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				checkTokenNameInStatus(g, testSetup.InCluster.Bindings[0], Not(BeEmpty()))
			})

			Eventually(func(g Gomega) error {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())

				binding.Status.LinkedAccessTokenName = "my random link name"
				return ITest.Client.Status().Update(ITest.Context, binding)
			}).Should(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				checkTokenNameInStatus(g, testSetup.InCluster.Bindings[0], Not(Or(BeEmpty(), Equal("my random link name"))))
			})
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

				ITest.TestServiceProvider.LookupTokensImpl = func(ctx context.Context, c client.Client, binding *api.SPIAccessTokenBinding) ([]api.SPIAccessToken, error) {
					if strings.HasSuffix(binding.Spec.RepoUrl, "test") {
						return []api.SPIAccessToken{*createdToken}, nil
					} else if strings.HasSuffix(binding.Spec.RepoUrl, "other") {
						return []api.SPIAccessToken{*otherToken}, nil
					} else {
						return nil, fmt.Errorf("request for invalid test token")
					}
				}

				testSetup.ReconcileWithCluster(nil)
			})

			It("changes the linked token, too", func() {
				Eventually(func(g Gomega) {
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), createdBinding)).To(Succeed())

					createdBinding.Spec.RepoUrl = "test-provider://other"
					g.Expect(ITest.Client.Update(ITest.Context, createdBinding)).To(Succeed())
				}).Should(Succeed())

				testSetup.ReconcileWithCluster(func(g Gomega) {
					createdBinding = testSetup.InCluster.Bindings[0]
					g.Expect(createdBinding.Status.LinkedAccessTokenName).To(Equal(otherToken.Name))
					g.Expect(createdBinding.Status.OAuthUrl).To(Equal(otherToken.Status.OAuthUrl))
					g.Expect(createdBinding.Status.UploadUrl).To(Equal(otherToken.Status.UploadUrl))
				})
			})
		})
	})

	Describe("Delete binding", func() {
		var createdBinding *api.SPIAccessTokenBinding
		var syncedSecret *corev1.Secret

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{
					StandardTestBinding("delete-test"),
				},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					token := objects.Tokens[0]

					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&token)

					err := ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
						AccessToken: "token",
					})
					Expect(err).NotTo(HaveOccurred())

					// now that the token is stored, we can simulate parsing its metadata from the SP
					ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "alois",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(func(g Gomega) {
				g.Expect(testSetup.InCluster.Bindings[0].Status.SyncedObjectRef.Name).NotTo(BeEmpty())
			})
			createdBinding = testSetup.InCluster.Bindings[0]

			syncedSecret = &corev1.Secret{}
			Expect(ITest.Client.Get(ITest.Context,
				client.ObjectKey{Name: createdBinding.Status.SyncedObjectRef.Name, Namespace: createdBinding.Namespace},
				syncedSecret)).To(Succeed())
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("should delete the synced secret", func() {
			Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(syncedSecret), &corev1.Secret{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})

		It("should delete binding by general timeout", func() {
			ITest.OperatorConfiguration.AccessTokenBindingTtl = 500 * time.Millisecond

			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Bindings).To(BeEmpty())
			})
		})

		It("should not delete binding with overridden timeout by general timeout", func() {
			ITest.OperatorConfiguration.AccessTokenBindingTtl = 500 * time.Millisecond

			Eventually(func(g Gomega) error {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdBinding), binding)).To(Succeed())
				binding.Spec.Lifetime = "-1"
				return ITest.Client.Update(ITest.Context, binding)
			}).Should(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Bindings).ToNot(BeEmpty())
			})
		})

	})

	Describe("Syncing", func() {
		binding := StandardTestBinding("sync-test")
		binding.Spec.Secret = api.SecretSpec{
			Name: "binding-secret",
			Type: corev1.SecretTypeBasicAuth,
		}
		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{
					binding,
				},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
					ITest.TestServiceProvider.MapTokenImpl = func(_ context.Context, _ *api.SPIAccessTokenBinding, token *api.SPIAccessToken, data *api.Token) (serviceprovider.AccessTokenMapper, error) {
						return serviceprovider.DefaultMapToken(token, data), nil
					}

				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		When("token is ready", func() {
			It("creates the secret with the data", func() {
				By("checking there is no secret")
				testSetup.ReconcileWithCluster(func(g Gomega) {
					Expect(testSetup.InCluster.Bindings[0].Status.SyncedObjectRef.Name).To(BeEmpty())
				})

				By("updating the token")
				err := ITest.TokenStorage.Store(ITest.Context, testSetup.InCluster.Tokens[0], &api.Token{
					AccessToken:  "access",
					RefreshToken: "refresh",
					TokenType:    "awesome",
				})
				Expect(err).NotTo(HaveOccurred())

				ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
					Username: "test",
					UserId:   "42",
				})

				By("waiting for the secret to be mentioned in the binding status")
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					g.Expect(binding.Status.SyncedObjectRef.Name).To(Equal("binding-secret"))
					secret := &corev1.Secret{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())
					g.Expect(string(secret.Data["password"])).To(Equal("access"))
				})
			})

			It("keeps the secret data valid", func() {
				By("checking there is no secret")
				testSetup.ReconcileWithCluster(func(g Gomega) {
					Expect(testSetup.InCluster.Bindings[0].Status.SyncedObjectRef.Name).To(BeEmpty())
				})

				By("updating the token")
				err := ITest.TokenStorage.Store(ITest.Context, testSetup.InCluster.Tokens[0], &api.Token{
					AccessToken:  "access",
					RefreshToken: "refresh",
					TokenType:    "awesome",
				})
				Expect(err).NotTo(HaveOccurred())

				ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
					Username: "test",
					UserId:   "42",
				})

				By("waiting for the secret to be mentioned in the binding status")
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					g.Expect(binding.Status.SyncedObjectRef.Name).To(Equal("binding-secret"))
					secret := &corev1.Secret{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())
					g.Expect(string(secret.Data["password"])).To(Equal("access"))
				})

				By("changing secret data")
				Eventually(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					secret := &corev1.Secret{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())
					secret.Data["password"] = []byte("wrong")
					g.Expect(ITest.Client.Update(ITest.Context, secret)).To(Succeed())
				}).Should(Succeed())

				By("waiting for the secret data to be reverted back to the correct values")
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					secret := &corev1.Secret{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())
					g.Expect(string(secret.Data["password"])).To(Equal("access"))
				})

				By("changing token data")
				err = ITest.TokenStorage.Store(ITest.Context, testSetup.InCluster.Tokens[0], &api.Token{
					AccessToken:  "access-new",
					RefreshToken: "refresh",
					TokenType:    "awesome",
				})
				Expect(err).NotTo(HaveOccurred())

				By("waiting for the secret data to be updated to the correct values")
				Eventually(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					secret := &corev1.Secret{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())
					g.Expect(string(secret.Data["password"])).To(Equal("access-new"))
				}).Should(Succeed())
			})
		})

		When("token is not ready", func() {
			It("doesn't create secret", func() {
				testSetup.ReconcileWithCluster(func(g Gomega) {
					Expect(testSetup.InCluster.Bindings[0].Status.SyncedObjectRef.Name).To(BeEmpty())
				})
			})
		})
	})

	Describe("Status updates", func() {
		var createdToken *api.SPIAccessToken
		var createdBinding *api.SPIAccessTokenBinding

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{StandardTestBinding("status-updates-test")},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
					ITest.TestServiceProvider.MapTokenImpl = func(_ context.Context, _ *api.SPIAccessTokenBinding, token *api.SPIAccessToken, data *api.Token) (serviceprovider.AccessTokenMapper, error) {
						return serviceprovider.DefaultMapToken(token, data), nil
					}
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			createdToken = testSetup.InCluster.Tokens[0]
			createdBinding = testSetup.InCluster.Bindings[0]
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		When("linked token is ready and secret not injected", func() {
			BeforeEach(func() {
				// We have the token in the ready state... let's not look it up during token matching
				ITest.TestServiceProvider.LookupTokensImpl = nil

				ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
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
				testSetup.ReconcileWithCluster(func(g Gomega) {
					g.Expect(testSetup.InCluster.Tokens[0].Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
				})
			})

			It("should end in error phase if linked token doesn't fit the requirements", func() {
				// This happens when the OAuth flow gives fewer perms than we requested
				// I.e. we link the token, the user goes through OAuth flow, but the token we get doesn't
				// give us all the required permissions (which will manifest in it not being looked up during
				// reconciliation for given binding).

				testSetup.ReconcileWithCluster(func(g Gomega) {
					g.Expect(testSetup.InCluster.Bindings[0].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
				})
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
				ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
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
				ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&createdToken)

				Expect(ITest.Client.Create(ITest.Context, testBinding)).To(Succeed())
			})

			It("replaces the linked token with a more precise lookup if available", func() {
				// first, let's check that we're linked to the original token
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(testBinding))
					g.Expect(binding.Status.LinkedAccessTokenName).To(Equal(createdToken.Name))
				})

				// now start returning the better token from the lookup
				ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&betterToken)

				// and check that the binding switches to the better token
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(testBinding))
					g.Expect(binding.Status.LinkedAccessTokenName).To(Equal(betterToken.Name))
				})
			})
		})

		When("linked token data disappears after successful sync", func() {
			var secret *corev1.Secret

			BeforeEach(func() {
				err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
					AccessToken: "access_token",
				})
				Expect(err).NotTo(HaveOccurred())

				ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
					Username:             "alois",
					UserId:               "42",
					Scopes:               []string{},
					ServiceProviderState: []byte("state"),
				})

				testSetup.ReconcileWithCluster(func(g Gomega) {
					g.Expect(testSetup.InCluster.Tokens[0].Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
					g.Expect(testSetup.InCluster.Bindings[0].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
					secret = &corev1.Secret{}
					Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: testSetup.InCluster.Bindings[0].Status.SyncedObjectRef.Name, Namespace: "default"}, secret)).To(Succeed())
				})
			})

			It("deletes the secret and flips back to awaiting phase when token data disappears", func() {
				Expect(ITest.TokenStorage.Delete(ITest.Context, createdToken)).To(Succeed())

				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
					g.Expect(binding.Status.SyncedObjectRef.Name).To(BeEmpty())

					err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(secret), &corev1.Secret{})
					g.Expect(err).To(HaveOccurred())
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				})
			})

			It("deletes the secret and flips back to awaiting phase when token is in awaiting state", func() {
				ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(nil)

				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					g.Expect(testSetup.InCluster.Tokens[0].Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
					g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseAwaitingTokenData))
					g.Expect(binding.Status.SyncedObjectRef.Name).To(BeEmpty())

					err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(secret), &corev1.Secret{})
					g.Expect(err).To(HaveOccurred())
					g.Expect(errors.IsNotFound(err)).To(BeTrue())
				})
			})
		})

		When("binding requires invalid scopes", func() {
			It("should flip to error state", func() {
				ITest.TestServiceProvider.ValidateImpl = func(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
					return serviceprovider.ValidationResult{
						ScopeValidation: []error{stderrors.New("nope")},
					}, nil
				}

				// now check that the binding flipped to the error state
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
					g.Expect(binding.Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorReasonUnsupportedPermissions))
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
				ITest.HostCredsServiceProvider.GetBaseUrlImpl = func() string {
					return "invalid://abc."
				}
				Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())

				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(createdBinding))
					g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
					g.Expect(binding.Status.ErrorMessage).To(Not(BeEmpty()))
					g.Expect(binding.Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorReasonUnknownServiceProviderType))
					g.Expect(binding.Status.LinkedAccessTokenName).To(BeEmpty())
				})
			})
		})

		When("binding lifetime is invalid", func() {
			It("should end in error phase and have an error message", func() {
				createdBinding = &api.SPIAccessTokenBinding{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "invalid-binding-",
						Namespace:    "default",
					},
					Spec: api.SPIAccessTokenBindingSpec{
						RepoUrl:  "test-provider://test",
						Lifetime: "2s",
					},
				}
				Expect(ITest.Client.Create(ITest.Context, createdBinding)).To(Succeed())

				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.GetBinding(client.ObjectKeyFromObject(createdBinding))
					g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
					g.Expect(binding.Status.ErrorMessage).To(Not(BeEmpty()))
					g.Expect(binding.Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorReasonInvalidLifetime))
					g.Expect(binding.Status.LinkedAccessTokenName).To(BeEmpty())
				})
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

				ITest.TestServiceProvider.LookupTokensImpl = nil
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
				Consistently(func(g Gomega) {
					tokens := &api.SPIAccessTokenList{}
					g.Expect(ITest.Client.List(ITest.Context, tokens)).Should(Succeed())
					// there should only be 1 token (the one created in the outer level). The change to the CRD makes every
					// attempt to create a new token and link it fail and clean up the freshly created token.
					// Because of the errors, we clean up but are left in a perpetual cycle of trying to create the linked
					// token and failing to link it and thus the new tokens are continuously appearing and disappearing.
					// Let's just check here that their number is not growing too much too quickly by this crude measure.
					g.Expect(len(tokens.Items)).To(BeNumerically("<", 5))
				}, "3s").Should(Succeed())
			})
		})
	})

	Describe("service account", func() {
		deleteSAs := func(g Gomega) {
			sas := &corev1.ServiceAccountList{}
			g.Expect(ITest.Client.List(ITest.Context, sas)).To(Succeed())

			deleted := []client.ObjectKey{}

			for _, sa := range sas.Items {
				err := ITest.Client.Delete(ITest.Context, &sa)

				if err != nil && !errors.IsNotFound(err) {
					g.Expect(err).To(Succeed())
				}
				deleted = append(deleted, client.ObjectKeyFromObject(&sa))
			}

			g.Eventually(func(gg Gomega) {
				sas := &corev1.ServiceAccountList{}
				gg.Expect(ITest.Client.List(ITest.Context, sas, client.InNamespace("default"))).To(Succeed())
				gg.Expect(sas.Items).To(BeEmpty())
			}).Should(Succeed())

			log.Log.Info("service accounts deleted", "test", ginkgo.CurrentGinkgoTestDescription().FullTestText, "deletedSAs", deleted)
		}

		Context("syncing", func() {
			testSetup := TestSetup{
				ToCreate: TestObjects{
					Bindings: []*api.SPIAccessTokenBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:    "default",
								GenerateName: "sa-token-sync-",
							},
							Spec: api.SPIAccessTokenBindingSpec{
								RepoUrl: "test-provider://test/",
								Secret: api.SecretSpec{
									// service account tokens are a corner case without much utility in SPI but we have supported them for a long time...
									Type: corev1.SecretTypeServiceAccountToken,
									Annotations: map[string]string{
										corev1.ServiceAccountNameKey: "sa-token",
									},
									LinkedTo: []api.SecretLink{
										{
											ServiceAccount: api.ServiceAccountLink{
												Reference: corev1.LocalObjectReference{
													Name: "sa-token",
												},
											},
										},
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:    "default",
								GenerateName: "sa-secret-sync-",
							},
							Spec: api.SPIAccessTokenBindingSpec{
								RepoUrl: "test-provider://test/",
								Secret: api.SecretSpec{
									Type: corev1.SecretTypeOpaque,
									LinkedTo: []api.SecretLink{
										{
											ServiceAccount: api.ServiceAccountLink{
												Managed: api.ManagedServiceAccountSpec{
													GenerateName: "sa-secret-sa-",
												},
											},
										},
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:    "default",
								GenerateName: "sa-image-pull-secret-sync-",
							},
							Spec: api.SPIAccessTokenBindingSpec{
								RepoUrl: "test-provider://test/",
								Secret: api.SecretSpec{
									Type: corev1.SecretTypeDockerConfigJson,
									LinkedTo: []api.SecretLink{
										{
											ServiceAccount: api.ServiceAccountLink{
												As: api.ServiceAccountLinkTypeImagePullSecret,
												Managed: api.ManagedServiceAccountSpec{
													GenerateName: "sa-image-pull-secret-sa-",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Behavior: ITestBehavior{
					BeforeObjectsCreated: func() {
						// wait until we can actually see the service account in the cluster
						Eventually(func(g Gomega) {
							g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "sa-token", Namespace: "default"}, &corev1.ServiceAccount{})).Should(Succeed())
						}).Should(Succeed())
					},
					AfterObjectsCreated: func(objects TestObjects) {
						// wait until we can actually see the service account in the cluster
						Eventually(func(g Gomega) {
							g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "sa-token", Namespace: "default"}, &corev1.ServiceAccount{})).Should(Succeed())
						}).Should(Succeed())

						err := ITest.TokenStorage.Store(ITest.Context, objects.Tokens[0], &api.Token{
							AccessToken:  "access",
							RefreshToken: "refresh",
							TokenType:    "awesome",
						})
						Expect(err).NotTo(HaveOccurred())
						ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
							Username:             "alois",
							UserId:               "42",
							Scopes:               []string{},
							ServiceProviderState: []byte("state"),
						})
						ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
					},
				},
			}

			BeforeEach(func() {
				Expect(ITest.Client.Create(ITest.Context, &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sa-token",
						Namespace: "default",
					},
				})).To(Succeed())

				// wait until we can actually see the service account in the cluster
				Eventually(func(g Gomega) {
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "sa-token", Namespace: "default"}, &corev1.ServiceAccount{})).Should(Succeed())
				}).Should(Succeed())

				testSetup.BeforeEach(nil)

			})

			AfterEach(func() {
				testSetup.AfterEach()
				deleteSAs(Default)
			})

			It("should persist service account name in status", func() {
				testSetup.ReconcileWithCluster(func(g Gomega) {
					for _, binding := range testSetup.InCluster.Bindings {
						g.Expect(binding.Status.ServiceAccountNames).NotTo(BeEmpty())

						sa := &corev1.ServiceAccount{}
						g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.ServiceAccountNames[0], Namespace: binding.Namespace}, sa)).To(Succeed())
					}
				})
			})

			It("should link the service account with service-account-token secret", func() {
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-token-sync", Namespace: "default"})[0]
					g.Expect(binding.Status.ServiceAccountNames).NotTo(BeEmpty())
				})
				binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-token-sync", Namespace: "default"})[0]
				sa := &corev1.ServiceAccount{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.ServiceAccountNames[0], Namespace: binding.Namespace}, sa)).To(Succeed())
				secret := &corev1.Secret{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())

				Expect(secret.Annotations[corev1.ServiceAccountNameKey]).To(Equal(sa.Name))
				Expect(sa.Secrets).NotTo(BeEmpty())
				Expect(sa.ImagePullSecrets).To(BeEmpty())
				Expect(sa.Secrets).To(ContainElement(corev1.ObjectReference{Name: secret.Name}))
			})

			It("should link the service account with docker-config-json secret as image pull secret", func() {
				testSetup.ReconcileWithCluster(func(g Gomega) {
					binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-token-sync", Namespace: "default"})[0]
					g.Expect(binding.Status.ServiceAccountNames).NotTo(BeEmpty())
				})
				binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-image-pull-secret-sync", Namespace: "default"})[0]
				sa := &corev1.ServiceAccount{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.ServiceAccountNames[0], Namespace: binding.Namespace}, sa)).To(Succeed())
				secret := &corev1.Secret{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())

				Expect(secret.Annotations[corev1.ServiceAccountNameKey]).To(BeEmpty())
				Expect(sa.Secrets).To(BeEmpty())
				Expect(sa.ImagePullSecrets).NotTo(BeEmpty())
				Expect(sa.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: secret.Name}))
			})

			It("is not deleted with the binding when unmanaged", func() {
				for _, b := range testSetup.InCluster.Bindings {
					Expect(ITest.Client.Delete(ITest.Context, b)).Should(Succeed())
				}

				Eventually(func(g Gomega) {
					bs := &api.SPIAccessTokenBindingList{}
					g.Expect(ITest.Client.List(ITest.Context, bs)).To(Succeed())
					g.Expect(bs.Items).To(BeEmpty())

					// TODO these are problematic, because we can leave dangling objects behind - see https://issues.redhat.com/browse/SVPI-210
					//secs := &corev1.SecretList{}
					//g.Expect(ITest.Client.List(ITest.Context, secs)).To(Succeed())
					//g.Expect(secs.Items).To(BeEmpty())
					//
					//sas := &corev1.ServiceAccountList{}
					//g.Expect(ITest.Client.List(ITest.Context, sas)).To(Succeed())
					//g.Expect(sas.Items).NotTo(BeEmpty())
				}).Should(Succeed())
			})

			It("should link the service account with an opaque secret", func() {
				binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-secret-sync", Namespace: "default"})[0]
				sa := &corev1.ServiceAccount{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.ServiceAccountNames[0], Namespace: binding.Namespace}, sa)).To(Succeed())
				secret := &corev1.Secret{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())

				Expect(secret.Annotations[corev1.ServiceAccountNameKey]).To(BeEmpty())
				Expect(sa.Secrets).To(HaveLen(1))
				Expect(sa.Secrets[0].Name).To(Equal(secret.Name))
			})

			It("should not link the service account with an opaque secret as image pull secret", func() {
				binding := &api.SPIAccessTokenBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    "default",
						GenerateName: "sa-invalid-sync-",
					},
					Spec: api.SPIAccessTokenBindingSpec{
						RepoUrl: "test-provider://test/",
						Secret: api.SecretSpec{
							Type: corev1.SecretTypeOpaque,
							LinkedTo: []api.SecretLink{
								{
									ServiceAccount: api.ServiceAccountLink{
										As: api.ServiceAccountLinkTypeImagePullSecret,
										Managed: api.ManagedServiceAccountSpec{
											GenerateName: "sa-secret-sa-",
										},
									},
								},
							},
						},
					},
				}

				Expect(ITest.Client.Create(ITest.Context, binding)).To(Succeed())

				Eventually(func(g Gomega) {
					b := &api.SPIAccessTokenBinding{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(binding), b)).To(Succeed())

					g.Expect(b.Status.ServiceAccountNames).To(BeEmpty())
					g.Expect(b.Status.SyncedObjectRef.Name).To(BeEmpty())
					g.Expect(b.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
					g.Expect(b.Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorReasonInconsistentSpec))
				}).Should(Succeed())
			})

			It("should let k8s API autofill service account secret data", func() {
				// pretend the Kubernetes API and fill in the data that is otherwise auto-filled by the kubernetes
				// control plane
				binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-token-sync", Namespace: "default"})[0]
				secret := &corev1.Secret{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())

				secret.Data["ca.crt"] = []byte("certificate")
				secret.Data["namespace"] = []byte("namespace")
				secret.Data["token"] = []byte("token")

				Expect(ITest.Client.Update(ITest.Context, secret)).To(Succeed())

				// from that point on, we should not see the operator remove the auto-filled data
				Consistently(func(g Gomega) {
					binding := testSetup.InCluster.GetBindingsByNamePrefix(client.ObjectKey{Name: "sa-token-sync", Namespace: "default"})[0]
					secret := &corev1.Secret{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.SyncedObjectRef.Name, Namespace: binding.Namespace}, secret)).To(Succeed())

					g.Expect(secret.Data).To(HaveKey("ca.crt"))
					g.Expect(secret.Data).To(HaveKey("token"))
					g.Expect(secret.Data).To(HaveKey("namespace"))
					g.Expect(secret.Data).To(HaveKey("extra"))
				}, "5s").Should(Succeed())
			})
		})

		Context("with pre-existing SAs", func() {
			var serviceAccount *corev1.ServiceAccount

			testSetup := TestSetup{
				ToCreate: TestObjects{
					Bindings: []*api.SPIAccessTokenBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:    "default",
								GenerateName: "sa-token-sync-",
							},
							Spec: api.SPIAccessTokenBindingSpec{
								RepoUrl: "test-provider://test/",
								Secret: api.SecretSpec{
									Type: corev1.SecretTypeBasicAuth,
									LinkedTo: []api.SecretLink{
										{
											ServiceAccount: api.ServiceAccountLink{
												Reference: corev1.LocalObjectReference{
													Name: "test-service-account",
												},
											},
										},
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:    "default",
								GenerateName: "sa-image-pull-secret-sync-",
							},
							Spec: api.SPIAccessTokenBindingSpec{
								RepoUrl: "test-provider://test/",
								Secret: api.SecretSpec{
									Type: corev1.SecretTypeDockerConfigJson,
									LinkedTo: []api.SecretLink{
										{
											ServiceAccount: api.ServiceAccountLink{
												As: api.ServiceAccountLinkTypeImagePullSecret,
												Reference: corev1.LocalObjectReference{
													Name: "test-service-account",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Behavior: ITestBehavior{
					AfterObjectsCreated: func(objects TestObjects) {
						err := ITest.TokenStorage.Store(ITest.Context, objects.Tokens[0], &api.Token{
							AccessToken:  "access",
							RefreshToken: "refresh",
							TokenType:    "awesome",
						})
						Expect(err).NotTo(HaveOccurred())
						ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
							Username:             "alois",
							UserId:               "42",
							Scopes:               []string{},
							ServiceProviderState: []byte("state"),
						})
						ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
					},
				},
			}

			BeforeEach(func() {
				serviceAccount = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service-account",
						Namespace: "default",
					},
				}
			})

			Context("without linked secrets", func() {
				BeforeEach(func() {
					Eventually(func(g Gomega) {
						g.Expect(ITest.Client.Create(ITest.Context, serviceAccount)).To(Succeed())
					}).Should(Succeed())

					testSetup.BeforeEach(func(g Gomega) {
						g.Expect(testSetup.InCluster.Bindings).To(HaveLen(2))
						g.Expect(testSetup.InCluster.Bindings[0].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
						g.Expect(testSetup.InCluster.Bindings[1].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
					})
				})

				AfterEach(func() {
					testSetup.AfterEach()
					deleteSAs(Default)
				})

				It("keeps the pre-existing links in SA secrets", func() {
					Eventually(func(g Gomega) {
						sa := &corev1.ServiceAccount{}
						Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(serviceAccount), sa)).To(Succeed())
						// TODO this can sometimes be 2 because of SVPI-210
						//Expect(sa.Secrets).To(HaveLen(1))
						Expect(sa.Secrets).NotTo(BeEmpty())
					}).Should(Succeed())
				})

				It("keeps the pre-existing links in SA image pull secrets", func() {
					Eventually(func(g Gomega) {
						sa := &corev1.ServiceAccount{}
						Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(serviceAccount), sa)).To(Succeed())
						// TODO this can sometimes be 2 because of SVPI-210
						//Expect(sa.ImagePullSecrets).To(HaveLen(1))
						Expect(sa.ImagePullSecrets).NotTo(BeEmpty())
					}).Should(Succeed())
				})

				It("is not deleted with the binding when unmanaged", func() {
					for _, b := range testSetup.InCluster.Bindings {
						Expect(ITest.Client.Delete(ITest.Context, b)).Should(Succeed())
					}

					Eventually(func(g Gomega) {
						bs := &api.SPIAccessTokenBindingList{}
						g.Expect(ITest.Client.List(ITest.Context, bs)).To(Succeed())
						g.Expect(bs.Items).To(BeEmpty())

						secs := &corev1.SecretList{}
						g.Expect(ITest.Client.List(ITest.Context, secs)).To(Succeed())
						g.Expect(secs.Items).To(BeEmpty())

						sas := &corev1.ServiceAccountList{}
						g.Expect(ITest.Client.List(ITest.Context, sas)).To(Succeed())
						g.Expect(sas.Items).NotTo(BeEmpty())
					}).Should(Succeed())
				})
			})

			Context("with linked secrets", func() {
				var linkedTokenSecret *corev1.Secret
				var linkedDockerConfigJsonSecret *corev1.Secret

				BeforeEach(func() {
					linkedTokenSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "linked-token-secret",
							Namespace: "default",
							Annotations: map[string]string{
								corev1.ServiceAccountNameKey: "test-service-account",
							},
						},
						Type: corev1.SecretTypeServiceAccountToken,
						Data: map[string][]byte{
							"extra": []byte("token"),
						},
					}

					linkedDockerConfigJsonSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "linked-docker-config-json-secret",
							Namespace: "default",
						},
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							".dockerconfigjson": []byte("{}"),
						},
					}

					Eventually(func(g Gomega) {
						g.Expect(ITest.Client.Create(ITest.Context, serviceAccount)).To(Succeed())
					}).Should(Succeed())

					Eventually(func(g Gomega) {
						g.Expect(ITest.Client.Create(ITest.Context, linkedTokenSecret)).To(Succeed())
						g.Expect(ITest.Client.Create(ITest.Context, linkedDockerConfigJsonSecret)).To(Succeed())
					}).Should(Succeed())

					serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: linkedTokenSecret.Name})
					serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, corev1.LocalObjectReference{Name: linkedDockerConfigJsonSecret.Name})

					Eventually(func(g Gomega) {
						g.Expect(ITest.Client.Update(ITest.Context, serviceAccount)).To(Succeed())
					}).Should(Succeed())

					Eventually(func(g Gomega) {
						sa := &corev1.ServiceAccount{}
						g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(serviceAccount), sa)).To(Succeed())
						g.Expect(sa.Secrets).To(HaveLen(1))
						g.Expect(sa.ImagePullSecrets).To(HaveLen(1))
					}).Should(Succeed())

					testSetup.BeforeEach(func(g Gomega) {
						g.Expect(testSetup.InCluster.Bindings).To(HaveLen(2))
						g.Expect(testSetup.InCluster.Bindings[0].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
						g.Expect(testSetup.InCluster.Bindings[1].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
					})
				})

				AfterEach(func() {
					testSetup.AfterEach()
					Expect(ITest.Client.Delete(ITest.Context, linkedTokenSecret)).To(Succeed())
					Expect(ITest.Client.Delete(ITest.Context, linkedDockerConfigJsonSecret)).To(Succeed())
					deleteSAs(Default)
				})

				It("keeps the pre-existing links in SA secrets", func() {
					Eventually(func(g Gomega) {
						sa := &corev1.ServiceAccount{}
						Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(serviceAccount), sa)).To(Succeed())
						// TODO we can have more than 2 due to https://issues.redhat.com/browse/SVPI-210
						//Expect(sa.Secrets).To(HaveLen(2))
						Expect(sa.Secrets).NotTo(BeEmpty())
					}).Should(Succeed())
				})

				It("keeps the pre-existing links in SA image pull secrets", func() {
					Eventually(func(g Gomega) {
						sa := &corev1.ServiceAccount{}
						Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(serviceAccount), sa)).To(Succeed())
						// TODO we can have more than 2 due to https://issues.redhat.com/browse/SVPI-210
						//Expect(sa.ImagePullSecrets).To(HaveLen(2))
						Expect(sa.ImagePullSecrets).NotTo(BeEmpty())
					}).Should(Succeed())
				})
			})
		})

		When("managed", func() {
			testSetup := TestSetup{
				ToCreate: TestObjects{
					Bindings: []*api.SPIAccessTokenBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:    "default",
								GenerateName: "binding-managed-sa-",
							},
							Spec: api.SPIAccessTokenBindingSpec{
								RepoUrl: "test-provider://test/",
								Secret: api.SecretSpec{
									Type: corev1.SecretTypeDockerConfigJson,
									LinkedTo: []api.SecretLink{
										{
											ServiceAccount: api.ServiceAccountLink{
												As: api.ServiceAccountLinkTypeImagePullSecret,
												Managed: api.ManagedServiceAccountSpec{
													GenerateName: "sa-managed-sa-",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Behavior: ITestBehavior{
					AfterObjectsCreated: func(objects TestObjects) {
						err := ITest.TokenStorage.Store(ITest.Context, objects.Tokens[0], &api.Token{
							AccessToken:  "access",
							RefreshToken: "refresh",
							TokenType:    "awesome",
						})
						Expect(err).NotTo(HaveOccurred())
						ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
							Username:             "alois",
							UserId:               "42",
							Scopes:               []string{},
							ServiceProviderState: []byte("state"),
						})
						ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
					},
				},
			}

			BeforeEach(func() {
				testSetup.BeforeEach(nil)
			})

			AfterEach(func() {
				testSetup.AfterEach()
				deleteSAs(Default)
			})

			It("is deleted with the binding", func() {
				Expect(ITest.Client.Delete(ITest.Context, testSetup.InCluster.Bindings[0])).Should(Succeed())

				Eventually(func(g Gomega) {
					bs := &api.SPIAccessTokenBindingList{}
					g.Expect(ITest.Client.List(ITest.Context, bs)).To(Succeed())
					g.Expect(bs.Items).To(BeEmpty())
				}).Should(Succeed())

				Eventually(func(g Gomega) {
					secs := &corev1.SecretList{}
					g.Expect(ITest.Client.List(ITest.Context, secs)).To(Succeed())
					g.Expect(secs.Items).To(BeEmpty())
				}).Should(Succeed())

				Eventually(func(g Gomega) {
					sas := &corev1.ServiceAccountList{}
					g.Expect(ITest.Client.List(ITest.Context, sas)).To(Succeed())
					g.Expect(sas.Items).To(BeEmpty())
				}).Should(Succeed())
			})
		})
	})

	Describe("service provider url is non-https when it's required", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "http-binding-",
							Namespace:    "default",
						},
						Spec: api.SPIAccessTokenBindingSpec{
							RepoUrl: "http://abc.foo/name/repo",
						},
					},
				},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.ValidationOptions = config.CustomValidationOptions{AllowInsecureURLs: false}
					ITest.TestServiceProvider.GetBaseUrlImpl = func() string {
						return "http://abc.foo"
					}
					ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
						return "http://abc.foo", nil
					})
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()

		})

		It("should end in error phase and have an error message", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Bindings[0].Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseError))
				g.Expect(testSetup.InCluster.Bindings[0].Status.ErrorMessage).To(Not(BeEmpty()))
				g.Expect(testSetup.InCluster.Bindings[0].Status.ErrorReason).To(Equal(api.SPIAccessTokenBindingErrorUnsupportedServiceProviderConfiguration))
				g.Expect(testSetup.InCluster.Bindings[0].Status.LinkedAccessTokenName).To(BeEmpty())
			})
		})
	})
})
