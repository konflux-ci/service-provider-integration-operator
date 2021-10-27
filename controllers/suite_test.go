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

package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	as "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

// not used yet, making linter happy
//var cfg *rest.Config
var cl client.Client
var testEnv *envtest.Environment
var ctx context.Context

// Used to mock the responses of the SPI REST API
var spiRestMock *httptest.Server
var spiRequestHandler func(http.ResponseWriter, *http.Request)

var reconciler AccessTokenSecretReconciler

const ns = "ns"

var _ = Describe("Status hadling", func() {
	var origUrl string

	BeforeEach(func() {
		origUrl = config.SpiUrl()

		Expect(cl.Create(ctx, &as.AccessTokenSecret{
			ObjectMeta: v1.ObjectMeta{
				Name:      "ats",
				Namespace: ns,
			},
			Spec: as.AccessTokenSecretSpec{
				AccessTokenName: "my-token",
				Target: as.AccessTokenTarget{
					Secret: &as.AccessTokenTargetSecret{
						Name:           "ats-secret",
						AccessTokenKey: "token",
						Labels:         map[string]string{"k": "v"},
						Annotations:    map[string]string{"a": "v"},
					}},
			},
		})).To(Succeed())
	})

	AfterEach(func() {
		config.SetSpiUrlForTest(origUrl)

		ats := &as.AccessTokenSecret{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: "ats", Namespace: ns}, ats)).To(Succeed())
		Expect(cl.Delete(ctx, ats)).To(Succeed())
	})

	It("should report failure to connect to SPI", func() {
		config.SetSpiUrlForTest("http://not-here")
		res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "ats", Namespace: ns}})
		Expect(err).To(HaveOccurred())
		Expect(res.IsZero()).To(BeTrue())

		ats := &as.AccessTokenSecret{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: "ats", Namespace: ns}, ats)).To(Succeed())

		Expect(ats.Status.Phase).To(Equal(as.AccessTokenSecretPhaseRetrieving))
		Expect(ats.Status.Reason).To(Equal(as.AccessTokenSecretFailureTokenRetrieval))
		Expect(ats.Status.Message).NotTo(BeEmpty())
	})

	It("should report successul injection", func() {
		spiRequestHandler = func(rw http.ResponseWriter, r *http.Request) {
			bytes, err := json.Marshal(map[string]interface{}{
				"token":                   "asdf",
				"name":                    "my-token",
				"serviceProviderUrl":      "https://spi.provider",
				"serviceProviderUserName": "alois",
				"serviceProviderUserId":   "42",
				"userId":                  "our-user-id",
				"expiredAfter":            "35",
			})
			if err != nil {
				log.Log.Error(err, "failed to marshal response to JSON. This should not happen.")
				return
			}
			rw.Header().Add("Content-Type", "application/json")
			_, err = rw.Write(bytes)
			if err != nil {
				log.Log.Error(err, "failed to write response. This should not happen.")
			}
		}

		res, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "ats", Namespace: ns}})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.IsZero()).To(BeTrue())

		ats := &as.AccessTokenSecret{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: "ats", Namespace: ns}, ats)).To(Succeed())

		Expect(ats.Status.Phase).To(Equal(as.AccessTokenSecretPhaseInjected))
		Expect(ats.Status.Reason).To(BeEmpty())
		Expect(ats.Status.Message).To(BeEmpty())
		Expect(ats.Status.Injected.Name).To(Equal("ats-secret"))
		Expect(ats.Status.Injected.Kind).To(Equal("Secret"))
		Expect(ats.Status.Injected.ApiVersion).To(Equal("v1"))
	})
})

// TODO add tests for config map and secret syncing

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = as.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = as.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cl, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(cl).NotTo(BeNil())

	ctx = context.TODO()
	err = cl.Create(ctx, &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ns,
		},
	})
	Expect(err).NotTo(HaveOccurred())

	spiRestMock = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if spiRequestHandler != nil {
			spiRequestHandler(rw, r)
		}
	}))
	config.SetSpiUrlForTest(spiRestMock.URL)
	reconciler = *NewAccessTokenSecretReconciler(cfg, cl, scheme.Scheme)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	spiRestMock.Close()
})

var _ = AfterEach(func() {
	spiRequestHandler = nil
})
