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
	"net/http"

	"k8s.io/apimachinery/pkg/api/meta"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/remote-secret/api/v1beta1"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RemoteSecretOAuth", func() {

	When("RemoteSecret is created with the host label", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				RemoteSecrets: []*v1beta1.RemoteSecret{StandardRemoteSecret()},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.OAuthCapability = func() serviceprovider.OAuthCapability {
						return &ITest.Capabilities
					}
					ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
						return "https://someprovider.com", nil
					})
				},
			},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})
		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("adds OAuth URL annotation to RemoteSecret", func() {
			testSetup.settleWithCluster(false, func(g Gomega) {
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Annotations[controllers.OAuthUrlAnnotation]).To(Not(BeEmpty()))
			})
		})
		It("adds OAuth condition to status", func() {
			testSetup.settleWithCluster(false, func(g Gomega) {
				conditions := testSetup.InCluster.RemoteSecrets[0].Status.Conditions
				g.Expect(meta.IsStatusConditionPresentAndEqual(conditions, controllers.OAuthConditionType, metav1.ConditionTrue)).To(BeTrue())
			})
		})
	})

	When(" existing RemoteSecret has its host label removed", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				RemoteSecrets: []*v1beta1.RemoteSecret{StandardRemoteSecret()},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProvider.OAuthCapability = func() serviceprovider.OAuthCapability {
						return &ITest.Capabilities
					}
					ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
						return "https://someprovider.com", nil
					})
				},
			},
		}
		BeforeEach(func() {
			// ensure annotation is there
			testSetup.BeforeEach(func(g Gomega) {
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Annotations[controllers.OAuthUrlAnnotation]).To(Not(BeEmpty()))
			})
			// delete label
			Eventually(func(g Gomega) {
				rs := testSetup.InCluster.RemoteSecrets[0]
				delete(rs.Labels, api.RSServiceProviderHostLabel)
				g.Expect(ITest.Client.Update(ITest.Context, rs)).To(Succeed())
			}).Should(Succeed())
		})
		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("removes OAuth URL annotation from RemoteSecret", func() {
			testSetup.settleWithCluster(false, func(g Gomega) {
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Annotations[controllers.OAuthUrlAnnotation]).To(BeEmpty())
			})
		})

		It("removes OAuth condition from RemoteSecret status", func() {
			testSetup.settleWithCluster(false, func(g Gomega) {
				conditions := testSetup.InCluster.RemoteSecrets[0].Status.Conditions
				g.Expect(meta.FindStatusCondition(conditions, controllers.OAuthConditionType)).To(BeNil())
			})
		})
	})

	When("service provider doesn't support OAuth", func() {
		testSetup := TestSetup{
			ToCreate: TestObjects{
				RemoteSecrets: []*v1beta1.RemoteSecret{StandardRemoteSecret()},
			},
			Behavior: ITestBehavior{
				BeforeObjectsCreated: func() {
					ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
						return "https://someprovider.com", nil
					})
				},
			},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})
		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("adds OAuth not supported condition to RemoteSecret status", func() {
			testSetup.settleWithCluster(false, func(g Gomega) {
				oauthCondition := meta.FindStatusCondition(testSetup.InCluster.RemoteSecrets[0].Status.Conditions, controllers.OAuthConditionType)
				g.Expect(oauthCondition).NotTo(BeNil())
				g.Expect(oauthCondition.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(oauthCondition.Reason).To(Equal(string(controllers.OAuthConditionReasonNotSupported)))
			})
		})
	})

})

func StandardRemoteSecret() *v1beta1.RemoteSecret {
	return &v1beta1.RemoteSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    "default",
			GenerateName: "test-remotesecret",
			Labels: map[string]string{
				api.RSServiceProviderHostLabel: "someprovider.com",
			},
		},
		Spec: v1beta1.RemoteSecretSpec{
			Secret:  v1beta1.LinkableSecretSpec{},
			Targets: nil,
		},
	}
}
