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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"

	sperrors "github.com/redhat-appstudio/service-provider-integration-operator/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SPIAccessToken", func() {

	Describe("Create without token data", func() {
		var createdToken *api.SPIAccessToken

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{
					StandardTestToken("create-test"),
				},
			},
			Behavior: ITestBehavior{
				// We need to set OAuth capability prior to the first reconciliation. Alternatively, it can be set in AfterObjectsCreated
				// and checked in testSetup.BeforeEach post-condition, but that kind of duplicates the OAuth URL test case itself.
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.OAuthCapability = func() serviceprovider.OAuthCapability {
						return &ITest.Capabilities
					}
				},
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			createdToken = testSetup.InCluster.Tokens[0]
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("sets up the finalizers", func() {
			Expect(createdToken.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-bindings"))
			Expect(createdToken.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/token-storage"))
		})

		It("doesn't auto-create the token data", func() {
			tokenData, err := ITest.TokenStorage.Get(ITest.Context, createdToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(tokenData).To(BeNil())
		})

		It("have the upload URL set", func() {
			Expect(strings.HasSuffix(createdToken.Status.UploadUrl, "/token/"+createdToken.Namespace+"/"+createdToken.Name)).To(BeTrue())
		})

		It("have the oauth URL set", func() {
			Expect(createdToken.Status.OAuthUrl).ToNot(BeEmpty())
		})
	})

	Describe("No OAuth Capability service provider", func() {
		var createdToken *api.SPIAccessToken

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{
					StandardTestToken("create-test"),
				},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			createdToken = testSetup.InCluster.Tokens[0]
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("does not have the oauth URL set", func() {
			Expect(createdToken.Status.OAuthUrl).To(BeEmpty())
		})
	})

	Describe("Status", func() {
		var createdToken *api.SPIAccessToken
		var origOAuthUrl string
		var origUploadUrl string

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{
					StandardTestToken("status-update-test"),
				}},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.OperatorConfiguration.BaseUrl = "https://initial.base.url"
					ITest.TestServiceProvider.OAuthCapability = func() serviceprovider.OAuthCapability {
						return &ITest.Capabilities
					}
				},
			},
		}
		BeforeEach(func() {
			// we don't have to have any special postCondition here, because testSetup guarantees the reconciliation to
			// happen at least once, at which point the status should be filled in and the phase already established
			testSetup.BeforeEach(nil)
			Expect(testSetup.InCluster.Tokens).To(HaveLen(1))
			createdToken = testSetup.InCluster.Tokens[0]

			Expect(createdToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
			origOAuthUrl = createdToken.Status.OAuthUrl
			origUploadUrl = createdToken.Status.UploadUrl
			Expect(origOAuthUrl).NotTo(BeEmpty())
			Expect(origUploadUrl).NotTo(BeEmpty())
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("updates the OAuth URL when the base url changes in the config", func() {
			ITest.OperatorConfiguration.BaseUrl = "https://completely-random-" + string(uuid.NewUUID())

			TriggerReconciliation(createdToken)

			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
				g.Expect(currentToken.Status.OAuthUrl).To(HavePrefix(ITest.OperatorConfiguration.BaseUrl))
				g.Expect(currentToken.Status.OAuthUrl).NotTo(Equal(origOAuthUrl))
			}).Should(Succeed())
		})

		It("updates the upload URL when the base url changes in the config", func() {
			ITest.OperatorConfiguration.BaseUrl = "https://completely-random-" + string(uuid.NewUUID())

			TriggerReconciliation(createdToken)

			Eventually(func(g Gomega) {
				currentToken := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
				g.Expect(currentToken.Status.UploadUrl).To(HavePrefix(ITest.OperatorConfiguration.BaseUrl))
				g.Expect(currentToken.Status.UploadUrl).NotTo(Equal(origUploadUrl))
			}).Should(Succeed())
		})
	})

	Describe("Token data disappears", func() {
		var createdToken *api.SPIAccessToken

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Tokens: []*api.SPIAccessToken{
					StandardTestToken("data-test"),
				},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					token := objects.Tokens[0]
					Expect(ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
						AccessToken: "access",
					})).To(Succeed())
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
				g.Expect(testSetup.InCluster.Tokens[0].Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
			})

			createdToken = testSetup.InCluster.Tokens[0]
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("flips token back to awaiting phase when data disappears", func() {
			ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(nil)
			Expect(ITest.TokenStorage.Delete(ITest.Context, createdToken)).To(Succeed())

			testSetup.ReconcileWithCluster(func(g Gomega) {
				currentToken := testSetup.InCluster.Tokens[0]
				g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
				g.Expect(currentToken.Status.TokenMetadata).To(BeNil())
			})
		})
	})

	Describe("Delete token", func() {
		var createdBinding *api.SPIAccessTokenBinding
		var createdToken *api.SPIAccessToken

		testSetup := TestSetup{
			ToCreate: TestObjects{
				Bindings: []*api.SPIAccessTokenBinding{StandardTestBinding("delete-test")},
			},
			Behavior: ITestBehavior{
				AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			Expect(testSetup.InCluster.Tokens).To(HaveLen(1))
			Expect(testSetup.InCluster.Bindings).To(HaveLen(1))
			createdBinding = testSetup.InCluster.Bindings[0]
			createdToken = testSetup.InCluster.Tokens[0]
		})

		AfterEach(func() {
			testSetup.AfterEach()
		})

		When("there are linked bindings", func() {
			It("doesn't happen", func() {
				token := &api.SPIAccessToken{}
				Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())

				// the delete request should succeed
				Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
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
			}).Should(Succeed())

			// test that the data disappears, too
			Eventually(func(g Gomega) {
				data, err := ITest.TokenStorage.Get(ITest.Context, createdToken)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(data).To(BeNil())
			}).Should(Succeed())
		})

		It("should delete the synced token in awaiting state", func() {
			origDeletionGracePeriod := ITest.OperatorConfiguration.DeletionGracePeriod
			ITest.OperatorConfiguration.DeletionGracePeriod = 1 * time.Second
			defer func() {
				ITest.OperatorConfiguration.DeletionGracePeriod = origDeletionGracePeriod
			}()

			Eventually(func(g Gomega) bool {
				return time.Now().Sub(createdBinding.CreationTimestamp.Time).Seconds() > 1
			}).Should(BeTrue())
			//flip back to awaiting
			ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(nil)
			testSetup.ReconcileWithCluster(nil)

			//delete binding
			Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())

			// and check that token eventually disappeared
			Eventually(func(g Gomega) {
				err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), &api.SPIAccessToken{})
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})

		It("should delete the token by timeout", func() {
			ITest.OperatorConfiguration.AccessTokenTtl = 500 * time.Millisecond

			//delete binding
			Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())

			// and check that token eventually disappeared
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.Tokens).To(BeEmpty())
			})
		})
	})

	Describe("Phase", func() {

		Context("with valid SP url", func() {
			var createdToken *api.SPIAccessToken
			testSetup := TestSetup{
				ToCreate: TestObjects{Tokens: []*api.SPIAccessToken{StandardTestToken("phase-test")}},
				Behavior: ITestBehavior{AfterObjectsCreated: func(objects TestObjects) {
					ITest.TestServiceProvider.LookupTokensImpl = serviceprovider.LookupConcreteToken(&objects.Tokens[0])
				}},
			}
			BeforeEach(func() {
				testSetup.BeforeEach(nil)
				createdToken = testSetup.InCluster.Tokens[0]
			})

			AfterEach(func() {
				testSetup.AfterEach()
			})

			It("defaults to AwaitingTokenData", func() {
				Expect(createdToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
				Expect(createdToken.Status.ErrorReason).To(BeEmpty())
				Expect(createdToken.Status.ErrorMessage).To(BeEmpty())
			})

			When("metadata is persisted", func() {
				It("flips to ready", func() {
					ITest.TestServiceProvider.PersistMetadataImpl = serviceprovider.PersistConcreteMetadata(&api.TokenMetadata{
						Username:             "user",
						UserId:               "42",
						Scopes:               []string{},
						ServiceProviderState: []byte("state"),
					})

					err := ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
						AccessToken: "access_token",
					})
					Expect(err).NotTo(HaveOccurred())

					testSetup.ReconcileWithCluster(func(g Gomega) {
						token := testSetup.InCluster.Tokens[0]
						g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
						g.Expect(token.Status.ErrorReason).To(BeEmpty())
						g.Expect(token.Status.ErrorMessage).To(BeEmpty())
					})
				})
			})

			When("metadata fails to persist due to invalid token", func() {
				It("flips to Invalid", func() {
					ITest.TestServiceProvider.PersistMetadataImpl = func(ctx context.Context, c client.Client, token *api.SPIAccessToken) error {
						return &sperrors.ServiceProviderHttpError{StatusCode: 401, Response: "the token is invalid"}
					}

					testSetup.ReconcileWithCluster(func(g Gomega) {
						token := testSetup.InCluster.Tokens[0]
						g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseInvalid))
						g.Expect(token.Status.ErrorReason).To(Equal(api.SPIAccessTokenErrorReasonMetadataFailure))
						g.Expect(token.Status.ErrorMessage).NotTo(BeEmpty())
					})
				})
			})

			When("service provider doesn't support some permissions", func() {
				It("flips to Invalid", func() {
					ITest.TestServiceProvider.ValidateImpl = func(ctx context.Context, validated serviceprovider.Validated) (serviceprovider.ValidationResult, error) {
						return serviceprovider.ValidationResult{
							ScopeValidation: []error{stderrors.New("nah")},
						}, nil
					}

					testSetup.ReconcileWithCluster(func(g Gomega) {
						token := testSetup.InCluster.Tokens[0]
						g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseInvalid))
						g.Expect(token.Status.ErrorReason).To(Equal(api.SPIAccessTokenErrorReasonUnsupportedPermissions))
						g.Expect(token.Status.ErrorMessage).NotTo(BeEmpty())
					})
				})
			})
		})

		Context("returns common provider with random SP url", func() {
			var createdToken *api.SPIAccessToken
			testSetup := TestSetup{
				ToCreate: TestObjects{Bindings: []*api.SPIAccessTokenBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "host-binding-",
							Namespace:    "default",
						},
						Spec: api.SPIAccessTokenBindingSpec{
							RepoUrl: "not-test-provider://foo",
						},
					}}},
			}
			BeforeEach(func() {
				testSetup.BeforeEach(func(g Gomega) {
					binding := testSetup.InCluster.Bindings[0]
					token := testSetup.InCluster.Tokens[0]

					g.Expect(binding.Status.LinkedAccessTokenName).To(Equal(token.Name))
					g.Expect(token.Status.Phase).NotTo(BeEmpty())
					g.Expect(token.Spec.ServiceProviderUrl).To(Equal("not-test-provider://not-baseurl"))
				})

				createdToken = testSetup.InCluster.Tokens[0]
			})

			AfterEach(func() {
				testSetup.AfterEach()
			})

			It("defaults to AwaitingTokenData with common type", func() {
				Eventually(func(g Gomega) {
					g.Expect(createdToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
					g.Expect(createdToken.Status.ErrorReason).To(BeEmpty())
					g.Expect(createdToken.Status.ErrorMessage).To(BeEmpty())
					g.Expect(createdToken.Labels[api.ServiceProviderTypeLabel]).To(Equal(string(ITest.HostCredsServiceProvider.GetTypeImpl().Name)))
				}).Should(Succeed())
			})
		})
	})
})
