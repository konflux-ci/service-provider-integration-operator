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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/application-api/api/v1alpha1"
	rapi "github.com/redhat-appstudio/remote-secret/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Application", func() {

	Describe("Cleanups RemoteSecret on application removal", func() {

		var application v1alpha1.Application

		testSetup := TestSetup{
			ToCreate: TestObjects{
				RemoteSecrets: []*rapi.RemoteSecret{{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-remotesecret-",
						Namespace:    "default",
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": "myenv"},
					},
					Spec: rapi.RemoteSecretSpec{
						Secret: rapi.LinkableSecretSpec{
							Name: "test-remote-secret",
							Type: "Opaque",
						},
						Targets: []rapi.RemoteSecretTarget{{
							Namespace: "test-namespace",
							ApiUrl:    "https://test-target",
						}},
					},
				}},
			},
			Behavior: ITestBehavior{},
		}

		BeforeEach(func() {
			testSetup.BeforeEach(nil)
			application = v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "default",
				},
				Spec: v1alpha1.ApplicationSpec{
					DisplayName: "test-app",
				},
			}
			Expect(ITest.Client.Create(ITest.Context, &application)).To(Succeed())
			// reconcile the cluster after creation
			testSetup.ReconcileWithCluster(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("have the target cleared", func() {
			// delete application
			Expect(ITest.Client.Delete(ITest.Context, &application)).To(Succeed())
			// reconcile the cluster after deletion
			testSetup.ReconcileWithCluster(nil)
			// check remote secret is gone
			Eventually(func(g Gomega) {
				rs := &rapi.RemoteSecret{}
				g.Expect(ITest.Client.Get(ITest.Context, types.NamespacedName{Name: testSetup.InCluster.RemoteSecrets[0].Name, Namespace: testSetup.InCluster.RemoteSecrets[0].Namespace}, rs)).To(HaveField("ErrStatus.Code", int32(404)))
			}).Should(Succeed())
		})

	})
})
