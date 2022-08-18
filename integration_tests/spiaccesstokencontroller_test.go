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
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		log.Log.Info("Create without token data [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("create-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		log.Log.Info("<----- Create without token data [BeforeEach]")
	})

	var _ = AfterEach(func() {
		log.Log.Info("Create without token data [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Create without token data [AfterEach]")
	})

	It("sets up the finalizers", func() {
		log.Log.Info("Create without token data sets up the finalizers ----->")
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
			g.Expect(token.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-bindings"))
			g.Expect(token.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/token-storage"))
		}).Should(Succeed())
		log.Log.Info("<----- Create without token data sets up the finalizers")
	})

	It("doesn't auto-create the token data", func() {
		log.Log.Info("Create without token data doesn't auto-create the token data ----->")
		accessToken := &api.SPIAccessToken{}
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), accessToken)).To(Succeed())

		tokenData, err := ITest.TokenStorage.Get(ITest.Context, accessToken)
		Expect(err).NotTo(HaveOccurred())
		Expect(tokenData).To(BeNil())
		log.Log.Info("<----- Create without token data doesn't auto-create the token data")
	})

	It("have the upload URL set", func() {
		Eventually(func(g Gomega) {
			token := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
			g.Expect(strings.HasSuffix(token.Status.UploadUrl, "/token/"+token.Namespace+"/"+token.Name)).To(BeTrue())
		}).Should(Succeed())
	})
})

var _ = Describe("Token data disappears", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		log.Log.Info("Token data disappears [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
			Username:             "alois",
			UserId:               "42",
			Scopes:               []string{},
			ServiceProviderState: []byte("state"),
		})
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)

		createdBinding, createdToken = createStandardPair("data-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))

		Expect(ITest.TokenStorage.Store(ITest.Context, createdToken, &api.Token{
			AccessToken: "access",
		})).To(Succeed())

		Eventually(func(g Gomega) {
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
		}).Should(Succeed())
		log.Log.Info("<----- Token data disappears [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Token data disappears [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Token data disappears [AfterEach]")
	})

	It("flips token back to awaiting phase when data disappears", func() {
		log.Log.Info("Token data disappears flips token back to awaiting phase when data disappears ----->")
		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(nil)
		Expect(ITest.TokenStorage.Delete(ITest.Context, createdToken)).To(Succeed())

		Eventually(func(g Gomega) {
			currentToken := &api.SPIAccessToken{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), currentToken)).To(Succeed())
			g.Expect(currentToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
			g.Expect(currentToken.Status.TokenMetadata).To(BeNil())
		}).Should(Succeed())
		log.Log.Info("<----- Token data disappears flips token back to awaiting phase when data disappears")
	})
})

var _ = Describe("Delete token", func() {
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	BeforeEach(func() {
		log.Log.Info("Delete token [BeforeEach] ----->")
		ITest.TestServiceProvider.Reset()
		createdBinding, createdToken = createStandardPair("delete-test")
		By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
		log.Log.Info("<----- Delete token [BeforeEach]")
	})

	AfterEach(func() {
		log.Log.Info("Delete token [AfterEach] ----->")
		Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
		log.Log.Info("<----- Delete token [AfterEach]")
	})

	When("there are linked bindings", func() {
		It("doesn't happen", func() {
			log.Log.Info("Delete token when there are linked bindings doesn't happen ----->")
			token := &api.SPIAccessToken{}
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())

			// the delete request should succeed
			Expect(ITest.Client.Delete(ITest.Context, token)).To(Succeed())
			// but the resource should not get deleted because of a finalizer that checks for the present bindings
			time.Sleep(1 * time.Second)
			Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(token), token)).To(Succeed())
			Expect(token.ObjectMeta.Finalizers).To(ContainElement("spi.appstudio.redhat.com/linked-bindings"))
			log.Log.Info("<----- Delete token when there are linked bindings doesn't happen")
		})
	})

	It("deletes token data from storage", func() {
		log.Log.Info("Delete token deletes token data from storage ----->")
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
		log.Log.Info("<----- Delete token deletes token data from storage")
	})

	It("should delete the synced token in awaiting state", func() {
		Eventually(func(g Gomega) bool {
			return time.Now().Sub(createdBinding.CreationTimestamp.Time).Seconds() > controllers.GracePeriodSeconds+1
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
	var createdBinding *api.SPIAccessTokenBinding
	var createdToken *api.SPIAccessToken

	Context("with valid SP url", func() {
		BeforeEach(func() {
			log.Log.Info("Phase with valid SP url [BeforeEach] ----->")
			ITest.TestServiceProvider.Reset()
			createdBinding, createdToken = createStandardPair("phase-test")
			By(fmt.Sprintf("testing on token '%s' and binding '%s'", createdToken.Name, createdBinding.Name))
			ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&createdToken)
			log.Log.Info("<----- Phase with valid SP url [BeforeEach]")
		})

		AfterEach(func() {
			log.Log.Info("Phase with valid SP url [AfterEach] ----->")
			Expect(ITest.Client.Delete(ITest.Context, createdBinding)).To(Succeed())
			Expect(ITest.Client.Delete(ITest.Context, createdToken)).To(Succeed())
			log.Log.Info("<----- Phase with valid SP url [AfterEach]")
		})

		It("defaults to AwaitingTokenData", func() {
			log.Log.Info("Phase with valid SP url defaults to AwaitingTokenData ----->")
			Eventually(func(g Gomega) {
				token := &api.SPIAccessToken{}
				g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
				g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
				g.Expect(token.Status.ErrorReason).To(BeEmpty())
				g.Expect(token.Status.ErrorMessage).To(BeEmpty())
			}).Should(Succeed())
			log.Log.Info("<----- Phase with valid SP url defaults to AwaitingTokenData")
		})

		When("metadata is persisted", func() {
			BeforeEach(func() {
				log.Log.Info("Phase with valid SP url when metadata is persisted [BeforeEach] ----->")
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
				log.Log.Info("<----- Phase with valid SP url when metadata is persisted [BeforeEach]")
			})

			It("flips to ready", func() {
				log.Log.Info("Phase with valid SP url when metadata is persisted flips to ready ----->")
				Eventually(func(g Gomega) {
					token := &api.SPIAccessToken{}
					g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdToken), token)).To(Succeed())
					g.Expect(token.Status.Phase).To(Equal(api.SPIAccessTokenPhaseReady))
					g.Expect(token.Status.ErrorReason).To(BeEmpty())
					g.Expect(token.Status.ErrorMessage).To(BeEmpty())
				}).Should(Succeed())
				log.Log.Info("<----- Phase with valid SP url when metadata is persisted flips to ready")
			})
		})

		When("metadata fails to persist due to invalid token", func() {
			It("flips to Invalid", func() {
				log.Log.Info("Phase with valid SP url when metadata fails to persist due to invalid token flips to Invalid ----->")
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
			log.Log.Info("<----- Phase with valid SP url when metadata fails to persist due to invalid token flips to Invalid")
		})

		When("service provider doesn't support some permissions", func() {
			It("flips to Invalid", func() {
				log.Log.Info("Phase with valid SP url when service provider doesn't support some permissions flips to Invalid ----->")
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
				log.Log.Info("<----- Phase with valid SP url when service provider doesn't support some permissions flips to Invalid")
			})
		})
	})

	Context("returns common provider with random SP url", func() {
		var otherToken *api.SPIAccessToken
		BeforeEach(func() {
			log.Log.Info("Phase returns common provider with random SP url [BeforeEach] ----->")
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
				g.Expect(otherToken.Spec.ServiceProviderUrl).To(Equal("not-test-provider://"))
			}).Should(Succeed())
			log.Log.Info("<----- Phase returns common provider with random SP url [BeforeEach]")
		})

		It("defaults to AwaitingTokenData with common type", func() {
			log.Log.Info("Phase returns common provider with random SP url defaults to AwaitingTokenData with common type ----->")
			Eventually(func(g Gomega) {
				g.Expect(otherToken.Status.Phase).To(Equal(api.SPIAccessTokenPhaseAwaitingTokenData))
				g.Expect(otherToken.Status.ErrorReason).To(BeEmpty())
				g.Expect(otherToken.Status.ErrorMessage).To(BeEmpty())
				g.Expect(otherToken.Labels[api.ServiceProviderTypeLabel]).To(Equal("HostCredsServiceProvider"))
			}).Should(Succeed())
			log.Log.Info("<----- Phase returns common provider with random SP url defaults to AwaitingTokenData with common type")
		})
	})
})
