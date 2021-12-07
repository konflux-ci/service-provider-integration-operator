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
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTICE: most of the calls to the K8s client in the tests are wrapped in Eventually() because
// the controller intervenes and updates the objects while we're executing the tests. This is
// the expected behavior but it makes our lives a little more complex here in the integration tests.

var _ = Describe("RBAC Enforcement", func() {
	BeforeEach(func() {
		// Give the noPrivsUser the ability to create SPIAccessTokenBindings, but not read SPIAccessTokens
		Expect(ITest.Client.Create(ITest.Context, &rbac.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "access-token-bindings",
				Namespace: "default",
			},
			Rules: []rbac.PolicyRule{
				{
					Verbs:     []string{"*"},
					APIGroups: []string{api.GroupVersion.Group},
					Resources: []string{"spiaccesstokenbindings"},
				},
			},
		})).To(Succeed())
		Expect(ITest.Client.Create(ITest.Context, &rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-user-roles",
				Namespace: "default",
			},
			RoleRef: rbac.RoleRef{
				Kind: "Role",
				Name: "access-token-bindings",
			},
			Subjects: []rbac.Subject{
				{
					Kind: "User",
					Name: "test-user",
				},
			},
		})).To(Succeed())
	})

	AfterEach(func() {
		rb := &rbac.RoleBinding{}
		r := &rbac.Role{}

		Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "test-user-roles", Namespace: "default"}, rb)).To(Succeed())
		Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: "access-token-bindings", Namespace: "default"}, r)).To(Succeed())

		Expect(ITest.Client.Delete(ITest.Context, rb)).To(Succeed())
		Expect(ITest.Client.Delete(ITest.Context, r)).To(Succeed())
	})

	It("fails to create SPIAccessTokenBinding if user doesn't have perms to SPIAccessToken", func() {
		err := ITest.NoPrivsClient.Create(ITest.Context, &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "should-not-exist",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl:     "https://github.com/acme/bikes",
				Permissions: api.Permissions{},
			},
		})

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("not authorized to operate on SPIAccessTokenBindings"))
	})
})
