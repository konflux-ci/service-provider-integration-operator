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
	"github.com/onsi/ginkgo"
	"github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"

	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/redhat-appstudio/remote-secret/pkg/logs"

	apiexv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"

	config2 "github.com/onsi/ginkgo/config"

	rconfig "github.com/redhat-appstudio/remote-secret/pkg/config"
	"github.com/redhat-appstudio/remote-secret/pkg/secretstorage/memorystorage"
	opconfig "github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"sigs.k8s.io/controller-runtime/pkg/cache"

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

var testServiceProvider = config.ServiceProviderType{
	Name: "TestServiceProvider",
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
	logs.InitDevelLoggers()
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel := context.WithCancel(context.TODO())
	ITest.Context = ctx
	ITest.Cancel = cancel

	ITest.MetricsRegistry = prometheus.NewPedanticRegistry()

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

	err = v1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = apiexv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(cl).NotTo(BeNil())

	// Let's configure the logging client as a pass-through, not outputting anything to not clobber up the logs too much.
	// Configure it in case you're having trouble understanding when and where the interactions with K8s API are happening.
	logK8sReads := false
	logK8sWrites := false
	includeStacktracesInK8sWrites := false
	lgCl := &LoggingKubernetesClient{
		Client:             cl,
		LogReads:           logK8sReads,
		LogWrites:          logK8sWrites,
		IncludeStacktraces: includeStacktracesInK8sWrites,
	}

	ITest.Client = lgCl

	noPrivsClient, err := client.New(noPrivsUser.Config(), client.Options{
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(noPrivsClient).NotTo(BeNil())
	ITest.NoPrivsClient = &LoggingKubernetesClient{
		Client:             noPrivsClient,
		LogReads:           logK8sReads,
		LogWrites:          logK8sWrites,
		IncludeStacktraces: logK8sWrites,
	}

	ITest.TestServiceProvider = serviceprovider.TestServiceProvider{}
	ITest.TestServiceProviderProbe = serviceprovider.ProbeFunc(func(_ *http.Client, baseUrl string) (string, error) {
		if strings.HasPrefix(baseUrl, "test-provider://") {
			return "test-provider://baseurl", nil
		}

		return "", nil
	})
	config.SupportedServiceProviderTypes = []config.ServiceProviderType{ITest.TestServiceProvider.GetType()}
	ITest.ValidationOptions = rconfig.CustomValidationOptions{AllowInsecureURLs: true}

	ITest.Capabilities = serviceprovider.TestCapabilities{}

	ITest.Capabilities.DownloadFileImpl = func(_ context.Context, _ string, _ string, _ string, _ *api.SPIAccessToken, i int) (string, error) {
		return "abcdefg", nil
	}

	ITest.Capabilities.GetOAuthEndpointImpl = func() string {
		return ITest.OperatorConfiguration.BaseUrl + "/test/oauth"
	}

	ITest.Capabilities.OAuthScopesForImpl = func(_ *api.Permissions) []string {
		return []string{}
	}

	ITest.HostCredsServiceProvider = serviceprovider.TestServiceProvider{}
	ITest.HostCredsServiceProvider.CustomizeReset = func(provider *serviceprovider.TestServiceProvider) {
		provider.GetTypeImpl = func() config.ServiceProviderType {
			return config.ServiceProviderTypeHostCredentials
		}
		provider.GetBaseUrlImpl = func() string {
			return "not-test-provider://not-baseurl"
		}
	}
	ITest.HostCredsServiceProvider.Reset()

	ITest.OperatorConfiguration = &opconfig.OperatorConfiguration{
		SharedConfiguration: config.SharedConfiguration{
			ServiceProviders: []config.ServiceProviderConfiguration{
				{
					OAuth2Config: &oauth2.Config{
						ClientID:     "testClient",
						ClientSecret: "testSecret",
					},
					ServiceProviderType: testServiceProvider,
				},
				{
					ServiceProviderBaseUrl: "https://spi-club.org",
					ServiceProviderType:    config.ServiceProviderType{Name: "SpiClub"},
				},
			},
		},
		AccessCheckTtl:        10 * time.Second,
		AccessTokenTtl:        10 * time.Second,
		AccessTokenBindingTtl: 10 * time.Second,
		FileContentRequestTtl: 10 * time.Second,
		DeletionGracePeriod:   10 * time.Second,
		EnableTokenUpload:     true,
		EnableRemoteSecrets:   true,
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
		NewClient: func(cache cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
			cl, err := cluster.DefaultNewClient(cache, config, options, uncachedObjects...)
			if err != nil {
				return nil, err
			}

			return &LoggingKubernetesClient{
				Client:             cl,
				LogReads:           logK8sReads,
				LogWrites:          logK8sWrites,
				IncludeStacktraces: includeStacktracesInK8sWrites,
			}, nil
		},
	})
	Expect(err).NotTo(HaveOccurred())

	strg := &memorystorage.MemoryStorage{}
	tokenStorage := tokenstorage.NewJSONSerializingTokenStorage(strg)

	ITest.TokenStorage = &tokenstorage.NotifyingTokenStorage{
		ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: ITest.Client},
		TokenStorage:  tokenStorage,
	}

	Expect(ITest.TokenStorage.Initialize(ctx)).To(Succeed())

	initializers := serviceprovider.NewInitializers().
		AddKnownInitializer(config.ServiceProviderType{Name: "TestServiceProvider"},
			serviceprovider.Initializer{
				Probe: serviceprovider.ProbeFunc(func(cl *http.Client, baseUrl string) (string, error) {
					return ITest.TestServiceProviderProbe.Examine(cl, baseUrl)
				}),
				Constructor: serviceprovider.ConstructorFunc(func(f *serviceprovider.Factory, _ *config.ServiceProviderConfiguration) (serviceprovider.ServiceProvider, error) {
					return ITest.TestServiceProvider, nil
				}),
			}).
		AddKnownInitializer(config.ServiceProviderType{Name: "HostCredentials"},
			serviceprovider.Initializer{
				Probe: serviceprovider.ProbeFunc(func(cl *http.Client, baseUrl string) (string, error) {
					return ITest.TestServiceProviderProbe.Examine(cl, baseUrl)
				}),
				Constructor: serviceprovider.ConstructorFunc(func(f *serviceprovider.Factory, _ *config.ServiceProviderConfiguration) (serviceprovider.ServiceProvider, error) {
					return ITest.HostCredsServiceProvider, nil
				}),
			})

	// the controllers themselves do not need the notifying token storage because they operate in the cluster
	// the notifying token storage is only needed if changes are only made to the storage and the cluster needs to be
	// notified about it. This only happens in OAuth service or in tests. So the test code is using notifying token
	// storage so that the controllers can react to the changes made to the token storage by the testsuite but the
	// controllers themselves use the "raw" token storage because they only write to the storage based on the conditions
	// in the cluster.
	err = controllers.SetupAllReconcilers(mgr, ITest.OperatorConfiguration, strg, initializers)
	Expect(err).NotTo(HaveOccurred())

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

var _ = BeforeEach(func() {
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>", "test", ginkgo.CurrentGinkgoTestDescription().FullTestText)
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>")
	log.Log.Info(">>>>>>>")
})

var _ = AfterEach(func() {
	testDesc := ginkgo.CurrentGinkgoTestDescription()
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<", "test", testDesc.FullTestText, "duration", testDesc.Duration, "failed", testDesc.Failed)
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<")
	log.Log.Info("<<<<<<<")
})
