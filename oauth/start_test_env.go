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

package oauth

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	kubernetes2 "github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	authz "k8s.io/api/authorization/v1"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
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
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var IT = struct {
	TestEnvironment *envtest.Environment
	Cancel          context.CancelFunc
	Scheme          *runtime.Scheme
	Namespace       string
	InClusterClient client.Client
	ClientFactory   kubernetes2.K8sClientFactory
	Clientset       *kubernetes.Clientset
	TokenStorage    tokenstorage.TokenStorage
	SessionManager  *scs.SessionManager
}{}

var ctx context.Context

func StartTestEnv() (struct {
	TestEnvironment *envtest.Environment
	Cancel          context.CancelFunc
	Scheme          *runtime.Scheme
	Namespace       string
	InClusterClient client.Client
	ClientFactory   kubernetes2.K8sClientFactory
	Clientset       *kubernetes.Clientset
	TokenStorage    tokenstorage.TokenStorage
	SessionManager  *scs.SessionManager
}, context.Context) {

	logf.SetLogger(k8szap.New(k8szap.WriteTo(GinkgoWriter), k8szap.UseDevMode(true)))
	logger, err := zap.NewDevelopment()
	Expect(err).NotTo(HaveOccurred())
	zap.ReplaceGlobals(logger)

	ctx, IT.Cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	CRDDirectoryPath := filepath.Join("..", "config", "crd", "bases")
	_, err = os.Stat(CRDDirectoryPath)
	if err != nil && os.IsNotExist(err) {
		CRDDirectoryPath = filepath.Join("..", CRDDirectoryPath)
		_, err = os.Stat(CRDDirectoryPath)
		if err != nil && os.IsNotExist(err) {
			panic(err)
		}
	}
	println("CRDDirectoryPath set to ", CRDDirectoryPath)
	IT.TestEnvironment = &envtest.Environment{
		ControlPlane: envtest.ControlPlane{
			APIServer: &envtest.APIServer{},
		},
		AttachControlPlaneOutput: true,
		CRDDirectoryPaths:        []string{CRDDirectoryPath},
		ErrorIfCRDPathMissing:    true,
	}
	IT.TestEnvironment.ControlPlane.APIServer.Configure().
		// Test environment switches off the ServiceAccount plugin by default... we actually need that one...
		Set("disable-admission-plugins", "")

	cfg, err := IT.TestEnvironment.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	IT.Scheme = runtime.NewScheme()

	Expect(corev1.AddToScheme(IT.Scheme)).To(Succeed())
	Expect(auth.AddToScheme(IT.Scheme)).To(Succeed())
	Expect(authz.AddToScheme(IT.Scheme)).To(Succeed())
	Expect(api.AddToScheme(IT.Scheme)).To(Succeed())

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

	// create the default state - we need to manually create the default service account in the default namespace
	// this is done by the kube-controller-manger but we don't have one in our test environment...
	cl, err := kubernetes.NewForConfig(IT.TestEnvironment.Config)
	Expect(err).NotTo(HaveOccurred())

	sec, err := cl.CoreV1().Secrets(IT.Namespace).Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-token",
		},
		Data: map[string][]byte{
			"ca.crt":    {},
			"namespace": {},
			"token":     {},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	sa, err := cl.CoreV1().ServiceAccounts(IT.Namespace).Create(context.TODO(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Secrets: []corev1.ObjectReference{
			{
				Kind:       "Secret",
				Namespace:  "default",
				Name:       "default-token",
				APIVersion: "v1",
			},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	if sec.Annotations == nil {
		sec.Annotations = make(map[string]string)
	}
	sec.Annotations["kubernetes.io/service-account.name"] = "default"
	sec.Annotations["kubernetes.io/service-account.uid"] = string(sa.UID)

	_, err = cl.CoreV1().Secrets(IT.Namespace).Update(context.TODO(), sec, metav1.UpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
	return IT, ctx
}
