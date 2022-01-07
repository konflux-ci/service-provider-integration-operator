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
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	config2 "github.com/onsi/ginkgo/config"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

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
	controller_config "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
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
	Client                   client.Client
	NoPrivsClient            client.Client
	TestEnvironment          *envtest.Environment
	Context                  context.Context
	TokenStorage             tokenstorage.TokenStorage
	Cancel                   context.CancelFunc
	TestServiceProviderProbe serviceprovider.Probe
	TestServiceProvider      TestServiceProvider
}

var ITest IntegrationTest

// TestServiceProvider is an implementation of the serviceprovider.ServiceProvider interface that can be modified by
// supplying custom implementations of each of the interface method. It provides dummy implementations of them, too, so
// that no null pointer dereferences should occur under normal operation.
type TestServiceProvider struct {
	LookupTokenImpl       func(context.Context, client.Client, *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error)
	GetBaseUrlImpl        func() string
	TranslateToScopesImpl func(permission api.Permission) []string
	GetTypeImpl           func() api.ServiceProviderType
	GetOauthEndpointImpl  func() string
}

var _ serviceprovider.ServiceProvider = (*TestServiceProvider)(nil)

func (t TestServiceProvider) LookupToken(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	if t.LookupTokenImpl == nil {
		return nil, nil
	}
	return t.LookupTokenImpl(ctx, cl, binding)
}

func (t TestServiceProvider) GetBaseUrl() string {
	if t.GetBaseUrlImpl == nil {
		return "test-provider://"
	}
	return t.GetBaseUrlImpl()
}

func (t TestServiceProvider) TranslateToScopes(permission api.Permission) []string {
	if t.TranslateToScopesImpl == nil {
		return []string{}
	}
	return t.TranslateToScopesImpl(permission)
}

func (t TestServiceProvider) GetType() api.ServiceProviderType {
	if t.GetTypeImpl == nil {
		return "TestServiceProvider"
	}
	return t.GetTypeImpl()
}

func (t TestServiceProvider) GetOAuthEndpoint() string {
	if t.GetOauthEndpointImpl == nil {
		return ""
	}
	return t.GetOauthEndpointImpl()
}

func (t TestServiceProvider) Reset() {
	t.LookupTokenImpl = nil
	t.GetBaseUrlImpl = nil
	t.TranslateToScopesImpl = nil
	t.GetTypeImpl = nil
	t.GetOauthEndpointImpl = nil
}

// Returns a function that can be used as an implementation of the serviceprovider.ServiceProvider.LookupToken method
// that just returns a freshly loaded version of the provided token. The token is a pointer to a pointer to the token
// so that this can also support lazily initialized tokens.
func LookupConcreteToken(tokenPointer **api.SPIAccessToken) func(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	return func(ctx context.Context, cl client.Client, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
		if *tokenPointer == nil {
			return nil, nil
		}

		freshToken := &api.SPIAccessToken{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(*tokenPointer), freshToken); err != nil {
			return nil, err
		}
		return freshToken, nil
	}
}

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

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	controller_config.SetSpiUrlForTest("https://spi-oauth")

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

	ITest.TestServiceProvider = TestServiceProvider{}
	ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
		if strings.HasPrefix(baseUrl, "test-provider://") {
			return "test-provider://", nil
		}

		return "", nil
	})

	operatorCfg := config.Configuration{
		ServiceProviders: []config.ServiceProviderConfiguration{
			{
				ClientId:            "testClient",
				ClientSecret:        "testSecret",
				ServiceProviderType: "TestServiceProvider",
			},
		},
		SharedSecret: []byte("secret"),
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

	strg, err := tokenstorage.New(ITest.Client)
	Expect(err).NotTo(HaveOccurred())

	err = (&webhooks.SPIAccessTokenWebhook{
		Client:       mgr.GetClient(),
		TokenStorage: strg,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())
	ITest.TokenStorage = strg

	err = (&webhooks.SPIAccessTokenBindingValidatingWebhook{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	factory := serviceprovider.Factory{
		Configuration: operatorCfg,
		Client:        http.DefaultClient,
		Initializers: map[config.ServiceProviderType]serviceprovider.Initializer{
			"TestServiceProvider": {
				Probe: serviceprovider.ProbeFunc(func(cl *http.Client, baseUrl string) (string, error) {
					return ITest.TestServiceProviderProbe.Examine(cl, baseUrl)
				}),
				Constructor: serviceprovider.ConstructorFunc(func(f *serviceprovider.Factory, _ string) (serviceprovider.ServiceProvider, error) {
					return ITest.TestServiceProvider, nil
				}),
			},
		},
	}

	err = (&controllers.SPIAccessTokenReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           strg,
		Configuration:          operatorCfg,
		ServiceProviderFactory: factory,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&controllers.SPIAccessTokenBindingReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		TokenStorage:           strg,
		ServiceProviderFactory: factory,
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
	if ITest.Cancel != nil {
		ITest.Cancel()
	}

	By("tearing down the test environment")
	if ITest.TestEnvironment != nil {
		err := ITest.TestEnvironment.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
})
