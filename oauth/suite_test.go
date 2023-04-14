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

package oauth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"
	kubernetes2 "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	authz "k8s.io/api/authorization/v1"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage/memorystorage"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	auth "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var IT = struct {
	TestEnvironment *envtest.Environment
	Context         context.Context
	Cancel          context.CancelFunc
	Scheme          *runtime.Scheme
	Namespace       string
	InClusterClient client.Client
	ClientFactory   kubernetes2.K8sClientFactory
	Clientset       *kubernetes.Clientset
	TokenStorage    tokenstorage.TokenStorage
	SessionManager  *scs.SessionManager
}{}

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SPI Oauth Controller Integration Test Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(k8szap.New(k8szap.WriteTo(GinkgoWriter), k8szap.UseDevMode(true)))
	logger, err := zap.NewDevelopment()
	Expect(err).NotTo(HaveOccurred())
	zap.ReplaceGlobals(logger)

	ctx, cancel := context.WithCancel(context.TODO())
	IT.Context = ctx
	IT.Cancel = cancel

	// The commented out sections below are from an attempt to use the envtest itself for our integration tests. This
	// is not working because we need fully functional service accounts in the cluster which seem not to be the case
	// in envtest even with the configuration modifications made below. It would be ideal if we COULD make this work
	// somehow but for the time being, let's just require a configured connection to a running cluster instead.
	// The commented-out sections are enclosed within [SELF_CONTAINED_TEST_ATTEMPT] [/SELF_CONTAINED_TEST_ATTEMPT].
	By("bootstrapping test environment")
	IT.TestEnvironment = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
		//[SELF_CONTAINED_TEST_ATTEMPT]
		//ControlPlane: envtest.ControlPlane{
		//	APIServer: &envtest.APIServer{},
		//},
		//AttachControlPlaneOutput: true,
		//[/SELF_CONTAINED_TEST_ATTEMPT]
	}
	//[SELF_CONTAINED_TEST_ATTEMPT]
	//IT.TestEnvironment.ControlPlane.APIServer.Configure().
	//	// Test environment switches off the ServiceAccount plugin by default... we actually need that one...
	//	Set("disable-admission-plugins", "")
	//[/SELF_CONTAINED_TEST_ATTEMPT]

	cfg, err := IT.TestEnvironment.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	IT.Scheme = runtime.NewScheme()

	Expect(corev1.AddToScheme(IT.Scheme)).To(Succeed())
	Expect(auth.AddToScheme(IT.Scheme)).To(Succeed())
	Expect(authz.AddToScheme(IT.Scheme)).To(Succeed())
	Expect(v1beta1.AddToScheme(IT.Scheme)).To(Succeed())

	IT.ClientFactory = clientfactory.UserAuthK8sClientFactory{ClientOptions: &client.Options{Scheme: IT.Scheme}, RestConfig: cfg}

	// create the test namespace which we'll use for the tests
	IT.InClusterClient, err = client.New(IT.TestEnvironment.Config, client.Options{Scheme: IT.Scheme})
	Expect(err).NotTo(HaveOccurred())

	IT.Clientset, err = kubernetes.NewForConfig(IT.TestEnvironment.Config)
	Expect(err).NotTo(HaveOccurred())

	IT.TokenStorage = &memorystorage.MemoryTokenStorage{}

	err = IT.TokenStorage.Initialize(ctx)
	Expect(err).NotTo(HaveOccurred())

	IT.TokenStorage = tokenstorage.NotifyingTokenStorage{
		ClientFactory: kubernetes2.SingleInstanceClientFactory{Client: IT.InClusterClient},
		TokenStorage:  IT.TokenStorage,
	}

	IT.SessionManager = scs.New()
	IT.SessionManager.Store = memstore.NewWithCleanupInterval(5 * time.Minute)
	IT.SessionManager.Lifetime = time.Hour
	IT.SessionManager.Cookie.Persist = false
	IT.SessionManager.IdleTimeout = 15 * time.Minute
	IT.SessionManager.Cookie.Name = "appstudio_spi_session"
	IT.SessionManager.Cookie.SameSite = http.SameSiteNoneMode
	IT.SessionManager.Cookie.Secure = true

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "spi-oauth-test-",
		},
	}
	Expect(IT.InClusterClient.Create(context.TODO(), ns)).To(Succeed())
	IT.Namespace = ns.Name

	//[SELF_CONTAINED_TEST_ATTEMPT]
	//// create the default state - we need to manually create the default service account in the default namespace
	//// this is done by the kube-controller-manger but we don't have one in our test environment...
	//cl, err := kubernetes.NewForConfig(IT.TestEnvironment.Config)
	//Expect(err).NotTo(HaveOccurred())
	//
	//sec, err := cl.CoreV1().Secrets("default").Create(context.TODO(), &corev1.Secret{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name: "default-token",
	//	},
	//	Data: map[string][]byte{
	//		"ca.crt": []byte{},
	//		"namespace": []byte{},
	//		"token": []byte{},
	//	},
	//}, metav1.CreateOptions{})
	//sa, err := cl.CoreV1().ServiceAccounts("default").Create(context.TODO(), &corev1.ServiceAccount{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name: "default",
	//	},
	//	Secrets: []corev1.ObjectReference{
	//		{
	//			Kind:            "Secret",
	//			Namespace:       "default",
	//			Name:            "default-token",
	//			APIVersion:      "v1",
	//		},
	//	},
	//}, metav1.CreateOptions{})
	//Expect(err).NotTo(HaveOccurred())
	//Expect(sa)
	//
	//sec.Annotations["kubernetes.io/service-account.name"] = "default"
	//sec.Annotations["kubernetes.io/service-account.uid"] = string(sa.UID)
	//
	//sec, err = cl.CoreV1().Secrets("default").Update(context.TODO(), sec, metav1.UpdateOptions{})
	//Expect(err).NotTo(HaveOccurred())
	//[/SELF_CONTAINED_TEST_ATTEMPT]
}, 30)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	ns := &corev1.Namespace{}
	Expect(IT.InClusterClient.Get(context.TODO(), client.ObjectKey{Name: IT.Namespace}, ns)).To(Succeed())
	Expect(IT.InClusterClient.Delete(context.TODO(), ns)).To(Succeed())
	IT.Cancel()
	Expect(IT.TestEnvironment.Stop()).To(Succeed())
})
