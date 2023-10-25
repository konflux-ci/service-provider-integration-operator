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
	"testing"
)

// TODO: mock selfsubjectaccessreview is difficult
func TestCheckIdentityHasAccess(t *testing.T) {
	//updateReview := &authz.SelfSubjectAccessReview{
	//	Spec: authz.SelfSubjectAccessReviewSpec{
	//		ResourceAttributes: &authz.ResourceAttributes{
	//			Namespace: "default",
	//			Verb:      "update",
	//			Group:     v1beta1.GroupVersion.Group,
	//			Version:   v1beta1.GroupVersion.Version,
	//			Resource:  "remotesecrets",
	//		},
	//	},
	//	Status: authz.SubjectAccessReviewStatus{
	//		Allowed: true,
	//		Denied:  false,
	//	},
	//}
	//
	//getReview := &authz.SelfSubjectAccessReview{
	//	Spec: authz.SelfSubjectAccessReviewSpec{
	//		ResourceAttributes: &authz.ResourceAttributes{
	//			Namespace: "default",
	//			Verb:      "get",
	//			Group:     v1beta1.GroupVersion.Group,
	//			Version:   v1beta1.GroupVersion.Version,
	//			Resource:  "remotesecrets",
	//		},
	//	},
	//	Status: authz.SubjectAccessReviewStatus{
	//		Allowed: true,
	//		Denied:  false,
	//	},
	//}

	//sch := runtime.NewScheme()
	//utilruntime.Must(corev1.AddToScheme(sch))
	//utilruntime.Must(v1beta1.AddToScheme(sch))
	//utilruntime.Must(authz.AddToScheme(sch))
	//client := fake.NewClientBuilder().WithScheme(sch).WithObjects().Build()
	//strategy := RemoteSecretSyncStrategy{
	//	ClientFactory: kubernetesclient.SingleInstanceClientFactory{Client: client},
	//}
	//hasAccess, err := strategy.checkIdentityHasAccess(context.TODO(), "default")
	//assert.NoError(t, err)
	//assert.True(t, hasAccess)
}
