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

package contracts

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/onsi/ginkgo"
	gomega "github.com/onsi/gomega"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	kubernetes2 "github.com/redhat-appstudio/remote-secret/pkg/kubernetesclient"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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

const timeout = 10 * time.Second
const interval = 250 * time.Millisecond

// ------------------------------------
// State handler methods implementation
// ------------------------------------
// TBD

// -----------------------
// Clenaup implementation
// -----------------------

func cleanUpNamespaces() {
	fmt.Fprintln(ginkgo.GinkgoWriter, "clean up namespaces")
	removeAllInstances(&v1beta1.SPIAccessCheck{})
}

// remove all instances of the given type within the whole cluster
func removeAllInstances(myInstance client.Object) {
	listOfNamespaces := getListOfNamespaces()
	for _, item := range listOfNamespaces.Items {
		removeAllInstacesInNamespace(item.Name, myInstance)
	}
}

// return all namespaces where the instances of the specified object kind exist
func getListOfNamespaces() core.NamespaceList {
	namespaceList := &core.NamespaceList{}
	err := IT.InClusterClient.List(context.Background(), namespaceList, &client.ListOptions{Namespace: ""})
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get list of namespaces: %s", err))
	return *namespaceList
}

func removeAllInstacesInNamespace(namespace string, myInstance client.Object) {
	objectKind := strings.Split(reflect.TypeOf(myInstance).String(), ".")[1]
	remainingCount := getObjectCountInNamespace(objectKind, namespace)
	if remainingCount == 0 {
		return
	}
	// remove resources in namespace
	err := IT.InClusterClient.DeleteAllOf(context.Background(), myInstance, client.InNamespace(namespace))
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to delete %s: %s", myInstance, err))

	// watch number of resources existing
	gomega.Eventually(func() bool {
		objectKind := strings.Split(reflect.TypeOf(myInstance).String(), ".")[1]
		remainingCount := getObjectCountInNamespace(objectKind, namespace)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Removing %s instance from %s namespace. Remaining: %d", objectKind, namespace, remainingCount)
		return remainingCount == 0
	}, timeout, interval).Should(gomega.BeTrue())
}

func getObjectCountInNamespace(objectKind string, namespace string) int {
	unstructuredObject := &unstructured.Unstructured{}

	unstructuredObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1beta1.GroupVersion.Group,
		Version: v1beta1.GroupVersion.Version,
		Kind:    objectKind,
	})

	err := IT.InClusterClient.List(context.Background(), unstructuredObject, &client.ListOptions{Namespace: namespace})
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("Failed to get list of %s: %s", objectKind, err))

	listOfObjects, _ := unstructuredObject.ToList()
	return len(listOfObjects.Items)
}
