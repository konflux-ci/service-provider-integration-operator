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
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

	apiexv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/hashicorp/vault/vault"

	config2 "github.com/onsi/ginkgo/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	rbac "k8s.io/api/rbac/v1"

	//+kubebuilder:scaffold:imports
	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/controllers"
	authzv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type IntegrationTest struct {
	Client                   client.Client
	NoPrivsClient            client.Client
	TestEnvironment          *envtest.Environment
	Context                  context.Context
	TokenStorage             tokenstorage.TokenStorage
	Cancel                   context.CancelFunc
	TestServiceProviderProbe serviceprovider.Probe
	TestServiceProvider      TestServiceProvider
	HostCredsServiceProvider TestServiceProvider
	VaultTestCluster         *vault.TestCluster
	OperatorConfiguration    *opconfig.OperatorConfiguration
}

var ITest IntegrationTest

var _ serviceprovider.ServiceProvider = (*TestServiceProvider)(nil)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "SPI Operator Integration Test Suite")
}

var _ = BeforeSuite(func() {
	if config2.GinkgoConfig.ParallelTotal > 1 {
		// The reason for this is that the tests can modify the behavior of the service provider implementation. If
		// we allowed parallel execution, the tests would step on each other's toes by possibly using an incorrect
		// service provider method implementation.
		Fail("This testsuite cannot be run in parallel")
	}
	logs.InitDevelLoggers()
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ITest = IntegrationTest{}

	ctx, cancel := context.WithCancel(context.TODO())
	ITest.Context = ctx
	ITest.Cancel = cancel

	By("bootstrapping test environment")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		//UseExistingCluster: pointer.BoolPtr(true),
	}
	ITest.TestEnvironment = testEnv

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	noPrivsUser, err := testEnv.AddUser(envtest.User{
		Name:   "test-user",
		Groups: []string{},
	}, cfg)
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

	err = apiexv1.AddToScheme(scheme)
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

	ITest.TestServiceProvider = TestServiceProvider{}
	ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
		if strings.HasPrefix(baseUrl, "test-provider://") {
			return "test-provider://baseurl", nil
		}

		return "", nil
	})
	ITest.HostCredsServiceProvider = TestServiceProvider{
		GetTypeImpl: func() api.ServiceProviderType {
			return "HostCredsServiceProvider"
		},

		GetBaseUrlImpl: func() string {
			return "not-test-provider://not-baseurl"
		},
	}

	ITest.OperatorConfiguration = &opconfig.OperatorConfiguration{
		SharedConfiguration: config.SharedConfiguration{
			ServiceProviders: []config.ServiceProviderConfiguration{
				{
					ClientId:            "testClient",
					ClientSecret:        "testSecret",
					ServiceProviderType: "TestServiceProvider",
				},
			},
		},
		AccessCheckTtl:        10 * time.Second,
		AccessTokenTtl:        10 * time.Second,
		AccessTokenBindingTtl: 10 * time.Second,
		DeletionGracePeriod:   10 * time.Second,
	}

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

	var strg tokenstorage.TokenStorage
	ITest.VaultTestCluster, strg, _, _ = tokenstorage.CreateTestVaultTokenStorageWithAuth(GinkgoT())
	Expect(err).NotTo(HaveOccurred())

	ITest.TokenStorage = &tokenstorage.NotifyingTokenStorage{
		Client:       cl,
		TokenStorage: strg,
	}

	Expect(ITest.TokenStorage.Initialize(ctx)).To(Succeed())

	factory := serviceprovider.Factory{
		Configuration:    ITest.OperatorConfiguration,
		KubernetesClient: mgr.GetClient(),
		HttpClient:       http.DefaultClient,
		Initializers: map[config.ServiceProviderType]serviceprovider.Initializer{
			"TestServiceProvider": {
				Probe: serviceprovider.ProbeFunc(func(cl *http.Client, baseUrl string) (string, error) {
					return ITest.TestServiceProviderProbe.Examine(cl, baseUrl)
				}),
				Constructor: serviceprovider.ConstructorFunc(func(f *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {
					return ITest.TestServiceProvider, nil
				}),
			},
			"HostCredentials": {
				Probe: serviceprovider.ProbeFunc(func(cl *http.Client, baseUrl string) (string, error) {
					return ITest.TestServiceProviderProbe.Examine(cl, baseUrl)
				}),
				Constructor: serviceprovider.ConstructorFunc(func(f *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {
					return ITest.HostCredsServiceProvider, nil
				}),
			},
		},
		TokenStorage: strg,
	}

	// the controllers themselves do not need the notifying token storage because they operate in the cluster
	// the notifying token storage is only needed if changes are only made to the storage and the cluster needs to be
	// notified about it. This only happens in OAuth service or in tests. So the test code is using notifying token
	// storage so that the controllers can react to the changes made to the token storage by the testsuite but the
	// controllers themselves use the "raw" token storage because they only write to the storage based on the conditions
	// in the cluster.
	err = (&controllers.SPIAccessTokenReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           strg,
		Configuration:          ITest.OperatorConfiguration,
		ServiceProviderFactory: factory,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&controllers.SPIAccessTokenBindingReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           strg,
		Configuration:          ITest.OperatorConfiguration,
		ServiceProviderFactory: factory,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&controllers.SPIAccessTokenDataUpdateReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&controllers.SPIAccessCheckReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		ServiceProviderFactory: factory,
		Configuration:          ITest.OperatorConfiguration,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		err = mgr.Start(ctx)
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
	}()
}, 3600)

var _ = AfterSuite(func() {
	if ITest.Cancel != nil {
		ITest.Cancel()
	}

	By("tearing down the test environment")
	if ITest.VaultTestCluster != nil {
		ITest.VaultTestCluster.Cleanup()
	}
	if ITest.TestEnvironment != nil {
		err := ITest.TestEnvironment.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})
