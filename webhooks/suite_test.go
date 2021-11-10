/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	rbac "k8s.io/api/rbac/v1"

	//+kubebuilder:scaffold:imports
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/vault"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cl client.Client
var noPrivsClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var vlt *vault.Vault

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhook Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("Consumes raw token", func() {
	BeforeEach(func() {
		Expect(cl.Create(ctx, &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "token",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: "Github",
				Permissions:         []api.Permission{api.PermissionRead, api.PermissionWrite},
				ServiceProviderUrl:  "https://github.com",
				RawTokenData: &api.Token{
					AccessToken:  "access",
					TokenType:    "Bearer",
					RefreshToken: "refresh",
				},
			},
		})).To(Succeed())
	})

	AfterEach(func() {
		t := api.SPIAccessToken{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: "token", Namespace: "default"}, &t)).To(Succeed())
		Expect(cl.Delete(ctx, &t)).To(Succeed())
	})

	It("finds no raw token after create", func() {
		t := api.SPIAccessToken{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: "token", Namespace: "default"}, &t)).To(Succeed())

		Expect(t.Spec.RawTokenData).To(BeNil())
		data, err := vlt.Get(&t)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())
		Expect(data.AccessToken).To(Equal("access"))
	})

	It("finds no raw token after update", func() {
		t := api.SPIAccessToken{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: "token", Namespace: "default"}, &t)).To(Succeed())

		t.Spec.RawTokenData = &api.Token{
			AccessToken: "updated",
		}

		Expect(cl.Update(ctx, &t)).To(Succeed())
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(&t), &t)).To(Succeed())

		Expect(t.Spec.RawTokenData).To(BeNil())
		data, err := vlt.Get(&t)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeNil())
		Expect(data.AccessToken).To(Equal("updated"))
	})
})

var _ = Describe("RBAC Enforcement", func() {
	BeforeEach(func() {
		// Give the noPrivsUser the ability to create SPIAccessTokenBindings, but not read SPIAccessTokens
		Expect(cl.Create(ctx, &rbac.Role{
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
		Expect(cl.Create(ctx, &rbac.RoleBinding{
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

		Expect(cl.Get(ctx, client.ObjectKey{Name: "test-user-roles", Namespace: "default"}, rb)).To(Succeed())
		Expect(cl.Get(ctx, client.ObjectKey{Name: "access-token-bindings", Namespace: "default"}, r)).To(Succeed())

		Expect(cl.Delete(ctx, rb)).To(Succeed())
		Expect(cl.Delete(ctx, r)).To(Succeed())
	})

	It("fails to create SPIAccessTokenBinding if user doesn't have perms to SPIAccessToken", func() {
		err := noPrivsClient.Create(ctx, &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "should-not-exist",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl:     "https://github.com/acme/bikes",
				Permissions: []api.Permission{api.PermissionRead},
			},
		})

		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("not authorized to operate on SPIAccessTokenBindings"))
	})
})

var _ = Describe("Update protections", func() {
	BeforeEach(func() {
		Expect(cl.Create(ctx, &api.SPIAccessTokenBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-binding",
				Namespace: "default",
			},
			Spec: api.SPIAccessTokenBindingSpec{
				RepoUrl:     "https://github.com/acme/repo",
				Permissions: []api.Permission{api.PermissionRead},
			},
		})).To(Succeed())
	})

	AfterEach(func() {
		allBindings := api.SPIAccessTokenBindingList{}
		allTokens := api.SPIAccessTokenList{}

		Expect(cl.List(ctx, &allBindings)).To(Succeed())
		Expect(cl.List(ctx, &allTokens)).To(Succeed())

		for _, b := range allBindings.Items {
			Expect(cl.Delete(ctx, &b)).To(Succeed())
		}

		for _, t := range allTokens.Items {
			Expect(cl.Delete(ctx, &t)).To(Succeed())
		}
	})

	It("should protect the linking label", func() {
		binding := &api.SPIAccessTokenBinding{}

		Expect(cl.Get(ctx, client.ObjectKey{Name: "test-binding", Namespace: "default"}, binding)).To(Succeed())

		Expect(binding.Labels[config.SPIAccessTokenLinkLabel]).NotTo(BeEmpty())

		binding.Labels = map[string]string{}
		Expect(cl.Update(ctx, binding)).To(Succeed())

		Expect(binding.Labels[config.SPIAccessTokenLinkLabel]).NotTo(BeEmpty())
	})
})

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	noPrivsUser, err := testEnv.AddUser(envtest.User{
		Name:   "test-user",
		Groups: []string{},
	}, nil)
	Expect(err).NotTo(HaveOccurred())

	scheme := runtime.NewScheme()
	err = api.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = authzv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = rbac.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cl, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(cl).NotTo(BeNil())

	noPrivsClient, err = client.New(noPrivsUser.Config(), client.Options{
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(noPrivsClient).NotTo(BeNil())

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	vlt = &vault.Vault{}
	err = (&SPIAccessTokenWebhook{
		Client: mgr.GetClient(),
		Vault:  vlt,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&SPIAccessTokenBindingValidatingWebhook{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&SPIAccessTokenBindingMutatingWebhook{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		err = mgr.Start(ctx)
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())

}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
