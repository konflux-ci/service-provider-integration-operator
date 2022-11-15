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

var _ = Describe("Create without token data", func() {
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		_, createdToken = createStandardPair("create-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
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

	It("have the upload URL set", func() {
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
			g.Expect(strings.HasSuffix(token.Status.UploadUrl, "/token/"+token.Namespace+"/"+token.Name)).To(BeTrue())
		}).Should(Succeed())
	})
})

var _ = Describe("Status", func() {
	var createdToken *api.SPIAccessToken
	var origBaseUrl string
	var origOAuthUrl string
	var origUploadUrl string

	triggerReconciliation := func() {
		Eventually(func(g Gomega) {
			// trigger the update of the token to force the reconciliation
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			currentToken.Annotations = map[string]string{"random-anno": string(uuid.NewUUID())}
			g.Expect(ITest.Client.Update(ITest.Context, currentToken)).To(Succeed())
		}).Should(Succeed())
	}

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		origBaseUrl = ITest.OperatorConfiguration.BaseUrl
		ITest.OperatorConfiguration.BaseUrl = "https://initial.base.url"
		ITest.TestServiceProvider.GetOauthEndpointImpl = func() string {
			return ITest.OperatorConfiguration.BaseUrl + "/test/oauth"
		}

		createdToken = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "status-update-test",
				Namespace:    "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderUrl: "test-provider://",
			},
		}

		Expect(ITest.Client.Create(ITest.Context, createdToken)).To(Succeed())

		Eventually(func(g Gomega) {
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
			origOAuthUrl = currentToken.Status.OAuthUrl
			origUploadUrl = currentToken.Status.UploadUrl
			g.Expect(origOAuthUrl).NotTo(BeEmpty())
			g.Expect(origUploadUrl).NotTo(BeEmpty())
		}).Should(Succeed())
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
		ITest.OperatorConfiguration.BaseUrl = origBaseUrl
	})

	It("updates the OAuth URL when the base url changes in the config", func() {
		ITest.OperatorConfiguration.BaseUrl = "https://completely-random-" + string(uuid.NewUUID())

		triggerReconciliation()

		Eventually(func(g Gomega) {
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			g.Expect(currentToken.Status.OAuthUrl).To(HavePrefix(ITest.OperatorConfiguration.BaseUrl))
			g.Expect(currentToken.Status.OAuthUrl).NotTo(Equal(origOAuthUrl))
		}).Should(Succeed())
	})

	It("updates the upload URL when the base url changes in the config", func() {
		ITest.OperatorConfiguration.BaseUrl = "https://completely-random-" + string(uuid.NewUUID())

		triggerReconciliation()

		Eventually(func(g Gomega) {
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			g.Expect(currentToken.Status.UploadUrl).To(HavePrefix(ITest.OperatorConfiguration.BaseUrl))
			g.Expect(currentToken.Status.UploadUrl).NotTo(Equal(origUploadUrl))
		}).Should(Succeed())
	})
})

var _ = Describe("Token data disappears", func() {
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		_, createdToken = createStandardPair("data-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		Expect(ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
			AccessToken: "access",
		})).To(Succeed())

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
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
	})

	It("flips token back to awaiting phase when data disappears", func() {
		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(nil)
		Expect(ITest.TokenStorage.Delete(ITest.Context, createdToken)).To(Succeed())

		Eventually(func(g Gomega) {
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
			g.Expect(currentToken.Status.TokenMetadata).To(BeNil())
		}).Should(Succeed())
	})
})

var _ = Describe("Delete token", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("delete-test")
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
	})

	AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
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
			return time.Now().Sub(createdBinding.CreationTimestamp.Time).Seconds() > ITest.OperatorConfiguration.DeletionGracePeriod.Seconds()+1
		}).Should(BeTrue())
		//flip back to awaiting
		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(nil)

		//delete binding
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())

		// and check that token eventually disappeared
		Eventually(func(g Gomega) {
			err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), &api.SPIAccessToken{})
			g.Expect(errors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed())
	})

	It("should delete the token by timeout", func() {
		orig := ITest.OperatorConfiguration.AccessTokenTtl
		ITest.OperatorConfiguration.AccessTokenTtl = 500 * time.Millisecond
		defer func() {
			ITest.OperatorConfiguration.AccessTokenTtl = orig
		}()

		//delete binding
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())

		// and check that token eventually disappeared
		Eventually(func(g Gomega) {
			err := ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), &api.SPIAccessToken{})
			if errors.IsNotFound(err) {
				return
			} else {
				//force reconciliation until timeout is passed
				token := &api.SPIAccessToken{}
				ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)
				token.Annotations = map[string]string{"foo": "bar"}
				ITest.Client.Update(ITest.Context, token)
			}
		}).Should(Succeed())
	})
})

var _ = Describe("Phase", func() {
	var createdToken *api.SPIAccessToken

	Context("with valid SP url", func() {
		BeforeEach(func() {
			ITest.TestServiceProvider.Reset()
			_, createdToken = createStandardPair("phase-test")
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		})

		AfterEach(func() {
			Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
			Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
		})

		It("defaults to AwaitingTokenData", func() {
			Eventually(func(g Gomega) {
				token := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
				g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
				g.Expect(token.Status.ErrorReason).To(BeEmpty())
				g.Expect(token.Status.ErrorMessage).To(BeEmpty())
			}).Should(Succeed())
		})

		When("metadata is persisted", func() {
			BeforeEach(func() {
				ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
					Username:             "user",
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
			})

			It("flips to ready", func() {
				Eventually(func(g Gomega) {
					token := &api.SPIAccessToken{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
					g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
					g.Expect(token.Status.ErrorReason).To(BeEmpty())
					g.Expect(token.Status.ErrorMessage).To(BeEmpty())
				}).Should(Succeed())
			})
		})

		When("metadata fails to persist due to invalid token", func() {
			It("flips to Invalid", func() {
				ITest.TestServiceProvider.PersistMetadataImpl = func(ctx context.Context, c client.Client, token *api.SPIAccessToken) error {
					return sperrors.ServiceProviderHttpError{StatusCode: 401, Response: "the token is invalid"}
				}

				Eventually(func(g Gomega) {
					token := &api.SPIAccessToken{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
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

				Eventually(func(g Gomega) {
					token := &api.SPIAccessToken{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
					g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseInvalid))
					g.Expect(token.Status.ErrorReason).To(Equal(api.SPIAccessTokenErrorReasonUnsupportedPermissions))
					g.Expect(token.Status.ErrorMessage).NotTo(BeEmpty())
				})
			})
		})
	})

	Context("returns common provider with random SP url", func() {
		var otherToken *api.SPIAccessToken
		BeforeEach(func() {
			ITest.TestServiceProvider.Reset()

			otherBinding := &api.SPIAccessTokenBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "host-binding-",
					Namespace:    "default",
				},
				Spec: api.SPIAccessTokenBindingSpec{
					RepoUrl: "not-test-provider://foo",
				},
			}
			ITest.TestServiceProvider.LookupTokenImpl = nil
			Expect(ITest.Client.Create(ITest.Context, otherBinding)).To(Succeed())

			Eventually(func(g Gomega) {
				binding := &api.SPIAccessTokenBinding{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(otherBinding), binding)).To(Succeed())
				g.Expect(binding.Status.LinkedAccessTokenName).NotTo(BeEmpty())
				otherBinding = binding
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				otherToken = &api.SPIAccessToken{}
				err := ITest.Client.Get(ITest.Context, client.ObjectKey{Name: otherBinding.Status.LinkedAccessTokenName, Namespace: otherBinding.Namespace}, otherToken)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(otherToken).NotTo(BeNil())
				g.Expect(otherToken.Status.Phase).NotTo(BeEmpty())
				g.Expect(otherToken.Spec.ServiceProviderUrl).To(Equal("not-test-provider://not-baseurl"))
			}).Should(Succeed())
		})

		It("defaults to AwaitingTokenData with common type", func() {
			Eventually(func(g Gomega) {
				g.Expect(otherToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
				g.Expect(otherToken.Status.ErrorReason).To(BeEmpty())
				g.Expect(otherToken.Status.ErrorMessage).To(BeEmpty())
				g.Expect(otherToken.Labels[api.ServiceProviderTypeLabel]).To(Equal("HostCredsServiceProvider"))
			}).Should(Succeed())
		})
	})
})
