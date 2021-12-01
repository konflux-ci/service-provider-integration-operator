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
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// a helper function to test that the controller eventually updates the object such that the linked token name
// linked in the binding matches the provided criteria.
func testTokenNameInStatus(linkMatcher OmegaMatcher) {
	Eventually(func(g Gomega) bool {
		binding := &api.SPIAccessTokenBinding{}
		g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-binding", Namespace: "default"}, binding)).To(Succeed())

		cond := g.Expect(binding.Status.LinkedAccessTokenName).Should(linkMatcher) &&
			g.Expect(binding.Labels[api.LinkedAccessTokenLabel]).Should(linkMatcher)

		return cond
	}, 10).Should(BeTrue())
}

var _ = Describe("Create binding", func() {
	var _ = BeforeEach(func() {
		Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-binding",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl:     "https://github.com/acme/acme",
				Permissions: []api.Permission{api.PermissionWrite},
				Secret:      api.SecretSpec{},
			},
		})).To(Succeed())
	})

	var _ = AfterEach(func() {
		allBindings := api.SPIAccessTokenBindingList{}
		allTokens := api.SPIAccessTokenList{}

		Expect(ITest.Client.List(ITest.Context, &allBindings)).To(Succeed())
		Expect(ITest.Client.List(ITest.Context, &allTokens)).To(Succeed())

		for _, b := range allBindings.Items {
			Expect(ITest.Client.Delete(ITest.Context, &b)).To(Succeed())
		}

		for _, t := range allTokens.Items {
			Expect(ITest.Client.Delete(ITest.Context, &t)).To(Succeed())
		}
	})

	It("should link the token to the binding", func() {
		testTokenNameInStatus(Not(BeEmpty()))
	})

	It("updates to the linked token status should be reverted", func() {
		testTokenNameInStatus(Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-binding", Namespace: "default"}, binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(Not(Or(BeEmpty(), Equal("my random link name"))))
	})
})

var _ = Describe("Update binding", func() {
	var _ = BeforeEach(func() {
		Expect(ITest.Client.Create(ITest.Context, &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-binding",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl:     "https://github.com/acme/acme",
				Permissions: []api.Permission{api.PermissionWrite},
				Secret:      api.SecretSpec{},
			},
		})).To(Succeed())
	})

	var _ = AfterEach(func() {
		allBindings := api.SPIAccessTokenBindingList{}
		allTokens := api.SPIAccessTokenList{}

		Expect(ITest.Client.List(ITest.Context, &allBindings)).To(Succeed())
		Expect(ITest.Client.List(ITest.Context, &allTokens)).To(Succeed())

		for _, b := range allBindings.Items {
			Expect(ITest.Client.Delete(ITest.Context, &b)).To(Succeed())
		}

		for _, t := range allTokens.Items {
			Expect(ITest.Client.Delete(ITest.Context, &t)).To(Succeed())
		}
	})

	It("reverts updates to the linked token label", func() {
		testTokenNameInStatus(Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-binding", Namespace: "default"}, binding)).To(Succeed())
			binding.Labels[api.LinkedAccessTokenLabel] = "my_random_link_name"
			return ITest.Client.Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(Not(Or(BeEmpty(), Equal("my_random_link_name"))))
	})

	It("reverts updates to the linked token in the status", func() {
		testTokenNameInStatus(Not(BeEmpty()))

		Eventually(func(g Gomega) error {
			binding := &api.SPIAccessTokenBinding{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-binding", Namespace: "default"}, binding)).To(Succeed())

			binding.Status.LinkedAccessTokenName = "my random link name"
			return ITest.Client.Status().Update(ITest.Context, binding)
		}).Should(Succeed())

		testTokenNameInStatus(Not(Or(BeEmpty(), Equal("my random link name"))))
	})

	It("changes the token based on the changed repo", func() {
		// TODO implement this
		Skip("Not implemented yet")
	})

	It("changes the token based on the changed permissions", func() {
		// TODO implement this
		Skip("Not implemented yet")
	})
})

var _ = Describe("Delete binding", func() {
	It("should delete the synced secret", func() {
		// TODO implement
		Skip("Not implemented yet")
	})
})

var _ = Describe("Syncing", func() {
	When("token is ready", func() {
		// TODO implement
	})

	When("token is not ready", func() {
		It("doesn't create secret", func() {
			// TODO implement
			Skip("Not implemented yet")
		})
	})
})
