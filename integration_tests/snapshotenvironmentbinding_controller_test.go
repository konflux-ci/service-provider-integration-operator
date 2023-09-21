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

var _ = Describe("SnapshotEnvironmentBinding", func() {

	Describe("Creates new target for RemoteSecret with local environment", func() {
		env := StandardLocalEnvironment("test-env")
		testSetup := TestSetup{
			ToCreate: TestObjects{
				Environments: []*v1alpha1.Environment{env},
			},
			Behavior: ITestBehavior{},
		}

		When("RemoteSecret has the environment label", func() {

			BeforeEach(func() {
				testSetup.BeforeEach(nil)
			})

			var _ = AfterEach(func() {
				testSetup.AfterEach()
			})

			It("sets the target", func() {
				rs := &rapi.RemoteSecret{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-remotesecret-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: rapi.RemoteSecretSpec{
						Secret: rapi.LinkableSecretSpec{
							Name: "test-remote-secret",
							Type: "Opaque",
						},
					},
				}
				Expect(ITest.Client.Create(ITest.Context, rs)).To(Succeed())
				// SEB must always be created after RS
				seb := &v1alpha1.SnapshotEnvironmentBinding{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-snapshotbinding-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: v1alpha1.SnapshotEnvironmentBindingSpec{
						Application: "test-app",
						Environment: env.Name,
						Components:  []v1alpha1.BindingComponent{},
					},
				}
				Expect(ITest.Client.Create(ITest.Context, seb)).To(Succeed())
				testSetup.ReconcileWithCluster(func(g Gomega) {
					g.Expect(testSetup.InCluster.RemoteSecrets[0].Spec.Targets[0].Namespace).To(Equal(env.Namespace))
				})
			})
		})

		When("RemoteSecrets has the environment annotations", func() {

			BeforeEach(func() {
				testSetup.BeforeEach(nil)
			})

			var _ = AfterEach(func() {
				testSetup.AfterEach()
			})

			It("sets the target", func() {
				rs := &rapi.RemoteSecret{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-remotesecret-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app"},
						Annotations:  map[string]string{"appstudio.redhat.com/environment": " env-foo,env-bar, " + env.Name},
					},
					Spec: rapi.RemoteSecretSpec{
						Secret: rapi.LinkableSecretSpec{
							Name: "test-remote-secret",
							Type: "Opaque",
						},
					},
				}
				Expect(ITest.Client.Create(ITest.Context, rs)).To(Succeed())
				// SEB must always be created after RS
				seb := &v1alpha1.SnapshotEnvironmentBinding{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-snapshotbinding-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: v1alpha1.SnapshotEnvironmentBindingSpec{
						Application: "test-app",
						Environment: env.Name,
						Components:  []v1alpha1.BindingComponent{},
					},
				}
				Expect(ITest.Client.Create(ITest.Context, seb)).To(Succeed())
				testSetup.ReconcileWithCluster(func(g Gomega) {
					g.Expect(testSetup.InCluster.RemoteSecrets[0].Spec.Targets[0].Namespace).To(Equal(env.Namespace))
				})
			})
		})
	})

	Describe("Creates new target for RemoteSecret with remote environment", func() {

		env := StandardRemoteEnvironment("test-remote-env")

		testSetup := TestSetup{
			ToCreate: TestObjects{
				RemoteSecrets: []*rapi.RemoteSecret{{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-remotesecret-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: rapi.RemoteSecretSpec{
						Secret: rapi.LinkableSecretSpec{
							Name: "test-remote-secret",
							Type: "Opaque",
						},
					},
				}},
				Environments: []*v1alpha1.Environment{env},
				SnapshotEnvBindings: []*v1alpha1.SnapshotEnvironmentBinding{{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-snapshotbinding-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: v1alpha1.SnapshotEnvironmentBindingSpec{
						Application: "test-app",
						Environment: env.Name,
						Components:  []v1alpha1.BindingComponent{},
					},
				}},
			},
			Behavior: ITestBehavior{},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("have the target set", func() {
			testSetup.ReconcileWithCluster(func(g Gomega) {
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Spec.Targets[0].Namespace).To(Equal(env.Spec.UnstableConfigurationFields.TargetNamespace))
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Spec.Targets[0].ApiUrl).To(Equal(env.Spec.UnstableConfigurationFields.APIURL))
				g.Expect(testSetup.InCluster.RemoteSecrets[0].Spec.Targets[0].ClusterCredentialsSecret).To(Equal(env.Spec.UnstableConfigurationFields.ClusterCredentialsSecret))
			})
		})

	})

	Describe("Removes the target for RemoteSecret with SEB removal ", func() {

		env := StandardRemoteEnvironment("test-remote-env")

		testSetup := TestSetup{
			ToCreate: TestObjects{
				RemoteSecrets: []*rapi.RemoteSecret{{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-remotesecret-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: rapi.RemoteSecretSpec{
						Secret: rapi.LinkableSecretSpec{
							Name: "test-remote-secret",
							Type: "Opaque",
						},
					},
				}},
				Environments: []*v1alpha1.Environment{env},
				SnapshotEnvBindings: []*v1alpha1.SnapshotEnvironmentBinding{{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "create-target-snapshotbinding-",
						Namespace:    env.Namespace,
						Labels:       map[string]string{"appstudio.redhat.com/application": "test-app", "appstudio.redhat.com/environment": env.Name},
					},
					Spec: v1alpha1.SnapshotEnvironmentBindingSpec{
						Application: "test-app",
						Environment: env.Name,
						Components:  []v1alpha1.BindingComponent{},
					},
				}},
			},
			Behavior: ITestBehavior{},
		}
		BeforeEach(func() {
			testSetup.BeforeEach(nil)
		})

		var _ = AfterEach(func() {
			testSetup.AfterEach()
		})

		It("have the target deleted", func() {
			//check target is there
			Expect(testSetup.InCluster.RemoteSecrets[0].Spec.Targets).ToNot(BeEmpty())
			// delete SEB
			Expect(ITest.Client.Delete(ITest.Context, testSetup.InCluster.SnapshotEnvBindings[0])).To(Succeed())
			// check target is gone
			Eventually(func(g Gomega) {
				rs := &rapi.RemoteSecret{}
				g.Expect(ITest.Client.Get(ITest.Context, types.NamespacedName{Name: testSetup.InCluster.RemoteSecrets[0].Name, Namespace: testSetup.InCluster.RemoteSecrets[0].Namespace}, rs)).To(Succeed())
				g.Expect(rs.Spec.Targets).To(BeEmpty())
			}).Should(Succeed())
		})
	})
})
