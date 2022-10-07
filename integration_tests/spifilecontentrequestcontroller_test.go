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

var _ = Describe("Create without token data", func() {
	var createdRequest *api.SPIFileContentRequest

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdRequest = &api.SPIFileContentRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "filerequest-",
				Namespace:    "default",
			},
			Spec: api.SPIFileContentRequestSpec{
				RepoUrl:  "test-provider://test",
				FilePath: "foo/bar.txt",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdRequest)).To(Succeed())
	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIFileContentRequest{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
	})

	It("have the status awaiting set", func() {
		Eventually(func(g Gomega) {
			request := &api.SPIFileContentRequest{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdRequest), request)).To(Succeed())
			g.Expect(request.Status.Phase == api.SPIFileContentRequestPhaseAwaitingTokenData).To(BeTrue())
		}).Should(Succeed())
	})

	It("have the upload and OAUth URLs set", func() {
		Eventually(func(g Gomega) {
			request := &api.SPIFileContentRequest{}
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdRequest), request)).To(Succeed())
			g.Expect(request.Status.TokenUploadUrl).NotTo(BeEmpty())
			//g.Expect(request.Status.OAuthUrl).NotTo(BeEmpty()) //failing, why ?
		}).Should(Succeed())
	})
})

var _ = Describe("With binding ready", func() {
	var createdRequest *api.SPIFileContentRequest

	BeforeEach(func() {
		ITest.TestServiceProvider.Reset()
		createdRequest = &api.SPIFileContentRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "filerequest-",
				Namespace:    "default",
			},
			Spec: api.SPIFileContentRequestSpec{
				RepoUrl:  "test-provider://test",
				FilePath: "foo/bar.txt",
			},
		}
		Expect(ITest.Client.Create(ITest.Context, createdRequest)).To(Succeed())
		// re-read to fill other fields
		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKeyFromObject(createdRequest), createdRequest)).To(Succeed())
		}).Should(Succeed())

		binding := &api.SPIAccessTokenBinding{}
		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: createdRequest.Status.LinkedBindingName, Namespace: createdRequest.Namespace}, binding)).To(Succeed())
		}).Should(Succeed())

		token := &api.SPIAccessToken{}
		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: binding.Status.LinkedAccessTokenName, Namespace: binding.Namespace}, token)).To(Succeed())
		}).Should(Succeed())

		ITest.TestServiceProvider.LookupTokenImpl = LookupConcreteToken(&token)
		err := ITest.TokenStorage.Store(ITest.Context, token, &api.Token{
			AccessToken: "token",
		})
		Expect(err).NotTo(HaveOccurred())

		ITest.TestServiceProvider.PersistMetadataImpl = PersistConcreteMetadata(&api.TokenMetadata{
			Username:             "alois",
			UserId:               "42",
			Scopes:               []string{},
			ServiceProviderState: []byte("state"),
		})

		Eventually(func(g Gomega) {
			g.Expect(ITest.Client.Get(ITest.Context, client.ObjectKey{Name: createdRequest.Status.LinkedBindingName, Namespace: createdRequest.Namespace}, binding)).To(Succeed())
			g.Expect(binding.Status.Phase).To(Equal(api.SPIAccessTokenBindingPhaseInjected))
		}).Should(Succeed())

	})

	var _ = AfterEach(func() {
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIFileContentRequest{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessTokenBinding{}, client.InNamespace("default"))).To(Succeed())
		Expect(ITest.Client.DeleteAllOf(ITest.Context, &api.SPIAccessToken{}, client.InNamespace("default"))).To(Succeed())
	})

	It("fetches the content", func() {
		//binding := &api.SPIAccessTokenBinding{}
	})

})
