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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SPI Oauth Controller Integration Test Suite")
}

var _ = BeforeSuite(func() {
	StartTestEnv()
}, 30)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	ns := &corev1.Namespace{}
	Expect(IT.InClusterClient.Get(context.TODO(), client.ObjectKey{Name: IT.Namespace}, ns)).To(Succeed())
	Expect(IT.InClusterClient.Delete(context.TODO(), ns)).To(Succeed())
	IT.Cancel()
	Expect(IT.TestEnvironment.Stop()).To(Succeed())
})
