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
		})
		AfterEach(func() {
			testSetup.AfterEach()
		})

		It("removes OAuth URL annotation from RemoteSecret", func() {
			// delete label
			Eventually(func(g Gomega) {
				rs := testSetup.InCluster.RemoteSecrets[0]
				delete(rs.Labels, api.RSServiceProviderHostLabel)
				g.Expect(ITest.Client.Update(ITest.Context, rs)).To(Succeed())
			}).Should(Succeed())

			// ensure annotation is deleted as well
			testSetup.settleWithCluster(false, func(g Gomega) {
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Annotations[controllers.OAuthUrlAnnotation]).To(BeEmpty())
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
