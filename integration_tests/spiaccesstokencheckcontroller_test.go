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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SPIAccessCheck", func() {
	var createdCheck *api.SPIAccessCheck

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		ITest.TestServiceProvider.CheckRepositoryAccessImpl = func(ctx context.Context, c client.Client, check *api.SPIAccessCheck) (*api.SPIAccessCheckStatus, error) {
			return &api.SPIAccessCheckStatus{
				Accessible:      true,
				Accessibility:   api.SPIAccessCheckAccessibilityPublic,
				Type:            "git",
				ServiceProvider: "testProvider",
				ErrorReason:     "",
				ErrorMessage:    "",
			}, nil
		}

		createdCheck = &api.SPIAccessCheck{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    "default",
				GenerateName: "create-test-check",
			},
			Spec: api.SPIAccessCheckSpec{
				RepoUrl: "test-provider://acme",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdCheck)).To(Succeed())
	})

	It("status is updated", func() {
		Eventually(func(g Gomega) {
			check := &api.SPIAccessCheck{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdCheck), check)).To(Succeed())
			g.Expect(check).NotTo(BeNil())
			g.Expect(check.Status.ServiceProvider).To(BeEquivalentTo("testProvider"))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	AfterEach(func() {
		Eventually(func(g Gomega) {
			check := &api.SPIAccessCheck{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdCheck), check)).Error()
		})
	})
})
