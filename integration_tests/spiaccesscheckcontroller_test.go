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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SPIAccessCheck", func() {
	testSetup := TestSetup{
		ToCreate: TestObjects{
			Checks: []*api.SPIAccessCheck{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:    "default",
						GenerateName: "create-test-check",
					},
					Spec: api.SPIAccessCheckSpec{
						RepoUrl: "test-provider://acme",
					},
				},
			},
		},

		Behavior: ITestBehavior{
			BeforeObjectsCreated: func() {
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
			},
		},
	}

	BeforeEach(func() {
		testSetup.BeforeEach(nil)
	})

	It("status is updated", func() {
		Expect(testSetup.InCluster.Checks[0].Status.ServiceProvider).To(BeEquivalentTo("testProvider"))
	})

	AfterEach(func() {
		testSetup.AfterEach()
	})
})
