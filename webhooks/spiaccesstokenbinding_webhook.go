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

package webhooks

import (
	"context"
	"fmt"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	authv1 "k8s.io/api/authentication/v1"
	authzv1 "k8s.io/api/authorization/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	wh "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-binding,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=create;update,versions=v1beta1,name=spiaccesstokenbinding-vwh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}

type SPIAccessTokenBindingValidatingWebhook struct {
	Client  client.Client
	decoder *wh.Decoder
}

var _ wh.DecoderInjector = (*SPIAccessTokenBindingValidatingWebhook)(nil)
var _ wh.Handler = (*SPIAccessTokenBindingValidatingWebhook)(nil)

func (w *SPIAccessTokenBindingValidatingWebhook) InjectDecoder(dec *wh.Decoder) error {
	w.decoder = dec
	return nil
}

func (w *SPIAccessTokenBindingValidatingWebhook) SetupWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/validate-binding", &wh.Webhook{
			Handler: w,
		})
	return nil
}

func (w *SPIAccessTokenBindingValidatingWebhook) Handle(ctx context.Context, req wh.Request) wh.Response {
	if err := w.checkCanReadAccessTokens(ctx, req.Namespace, req.UserInfo); err != nil {
		return wh.Denied(err.Error())
	}

	return wh.Allowed("")
}

func (w *SPIAccessTokenBindingValidatingWebhook) checkCanReadAccessTokens(ctx context.Context, ns string, user authv1.UserInfo) error {
	sar := &authzv1.SubjectAccessReview{
		Spec: authzv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Namespace: ns,
				Verb:      "get",
				Group:     api.GroupVersion.Group,
				Version:   api.GroupVersion.Version,
				Resource:  "SPIAccessToken",
			},
			UID:    user.UID,
			User:   user.Username,
			Groups: user.Groups,
		},
	}

	if err := w.Client.Create(ctx, sar); err != nil {
		return err
	}

	if !sar.Status.Allowed {
		if sar.Status.Reason != "" {
			return fmt.Errorf("not authorized to operate on SPIAccessTokenBindings because user cannot read SPIAccessTokens in namespace %s: %s", ns, sar.Status.Reason)
		} else {
			return fmt.Errorf("not authorized to operate on SPIAccessTokenBindings because user cannot read SPIAccessTokens in namespace %s", ns)
		}
	}

	return nil
}
