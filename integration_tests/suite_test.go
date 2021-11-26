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
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	rbac "k8s.io/api/rbac/v1"

	//+kubebuilder:scaffold:imports
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/vault"
	"github.com/redhat-appstudio/service-provider-integration-operator/webhooks"
	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type IntegrationTest struct {
	Client          client.Client
	NoPrivsClient   client.Client
	TestEnvironment *envtest.Environment
	Context         context.Context
	Vault           *vault.Vault
	Cancel          context.CancelFunc
}

var ITest IntegrationTest

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "SPI Operator Integration Test Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ITest = IntegrationTest{}

	ctx, cancel := context.WithCancel(context.TODO())
	ITest.Context = ctx
	ITest.Cancel = cancel

	By("bootstrapping test environment")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "config", "webhook")},
		},
	}
	ITest.TestEnvironment = testEnv

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	noPrivsUser, err := testEnv.AddUser(envtest.User{
		Name:   "test-user",
		Groups: []string{},
	}, nil)
	Expect(err).NotTo(HaveOccurred())

	scheme := runtime.NewScheme()

	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = api.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = authzv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = rbac.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(cl).NotTo(BeNil())
	ITest.Client = cl

	noPrivsClient, err := client.New(noPrivsUser.Config(), client.Options{
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(noPrivsClient).NotTo(BeNil())
	ITest.NoPrivsClient = noPrivsClient

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

	vlt := &vault.Vault{}
	err = (&webhooks.SPIAccessTokenWebhook{
		Client: mgr.GetClient(),
		Vault:  vlt,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())
	ITest.Vault = vlt

	err = (&webhooks.SPIAccessTokenBindingValidatingWebhook{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&controllers.SPIAccessTokenReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Vault:  vlt,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&controllers.SPIAccessTokenBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Vault:  vlt,
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
	}, 3600).Should(Succeed())

}, 3600)

var _ = AfterSuite(func() {
	ITest.Cancel()
	By("tearing down the test environment")
	err := ITest.TestEnvironment.Stop()
	Expect(err).NotTo(HaveOccurred())
})
