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
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider"
	adm "k8s.io/api/admission/v1"
	authv1 "k8s.io/api/authentication/v1"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	wh "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-binding,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=create;update,versions=v1beta1,name=spiaccesstokenbinding-vwh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}
//+kubebuilder:webhook:path=/mutate-binding,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=create;update,versions=v1beta1,name=spiaccesstokenbinding-mwh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}

type SPIAccessTokenBindingValidatingWebhook struct {
	Client  client.Client
	decoder *wh.Decoder
}

type SPIAccessTokenBindingMutatingWebhook struct {
	Client  client.Client
	decoder *wh.Decoder
}

var _ wh.DecoderInjector = (*SPIAccessTokenBindingValidatingWebhook)(nil)
var _ wh.Handler = (*SPIAccessTokenBindingValidatingWebhook)(nil)
var _ wh.DecoderInjector = (*SPIAccessTokenBindingMutatingWebhook)(nil)
var _ wh.Handler = (*SPIAccessTokenBindingMutatingWebhook)(nil)

func (w *SPIAccessTokenBindingValidatingWebhook) InjectDecoder(dec *wh.Decoder) error {
	w.decoder = dec
	return nil
}

func (w *SPIAccessTokenBindingMutatingWebhook) InjectDecoder(dec *wh.Decoder) error {
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

func (w *SPIAccessTokenBindingMutatingWebhook) SetupWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/mutate-binding", &wh.Webhook{
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

func (w *SPIAccessTokenBindingMutatingWebhook) Handle(ctx context.Context, req wh.Request) wh.Response {
	switch req.Operation {
	case adm.Create:
		return w.handleCreate(ctx, req)
	case adm.Update:
		return w.handleUpdate(ctx, req)
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

func (w *SPIAccessTokenBindingMutatingWebhook) handleCreate(ctx context.Context, req wh.Request) wh.Response {
	binding := &api.SPIAccessTokenBinding{}
	if err := w.decoder.DecodeRaw(req.Object, binding); err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	serviceProviderType, err := serviceprovider.ServiceProviderTypeFromURL(binding.Spec.RepoUrl)
	if err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	serviceProvider, err := serviceprovider.ByType(serviceProviderType, w.Client)
	if err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	token, err := serviceProvider.LookupToken(ctx, binding)
	if err != nil {
		return wh.Errored(http.StatusInternalServerError, err)
	}

	if token == nil {
		serviceProviderUrl, err := serviceProvider.GetServiceProviderUrlForRepo(binding.Spec.RepoUrl)
		if err != nil {
			return wh.Errored(http.StatusInternalServerError, err)
		}

		// create the token and put it in awaiting token state
		token = &api.SPIAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "generated-spi-access-token-",
				Namespace:    binding.Namespace,
			},
			Spec: api.SPIAccessTokenSpec{
				ServiceProviderType: serviceProviderType,
				Permissions:         binding.Spec.Permissions,
				ServiceProviderUrl:  serviceProviderUrl,
			},
		}

		if err := w.Client.Create(ctx, token); err != nil {
			return wh.Errored(http.StatusInternalServerError, err)
		}

		token.Status.Phase = api.SPIAccessTokenPhaseAwaitingTokenData
		if err := w.Client.Status().Update(ctx, token); err != nil {
			return wh.Errored(http.StatusInternalServerError, err)
		}
	}

	if binding.Labels == nil {
		binding.Labels = make(map[string]string)
	}

	binding.Labels[config.SPIAccessTokenLinkLabel] = token.Name

	json, err := json.Marshal(binding)
	if err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	return wh.PatchResponseFromRaw(req.Object.Raw, json)
}

func (w *SPIAccessTokenBindingMutatingWebhook) handleUpdate(ctx context.Context, req wh.Request) wh.Response {
	old := api.SPIAccessTokenBinding{}
	new := api.SPIAccessTokenBinding{}

	if err := w.decoder.DecodeRaw(req.OldObject, &old); err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	if err := w.decoder.DecodeRaw(req.Object, &new); err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	changed := false
	oldLabel := old.Labels[config.SPIAccessTokenLinkLabel]
	if oldLabel != "" && oldLabel != new.Labels[config.SPIAccessTokenLinkLabel] {
		if new.Labels == nil {
			new.Labels = map[string]string{}
		}
		new.Labels[config.SPIAccessTokenLinkLabel] = oldLabel
		changed = true
	}

	if changed {
		json, err := json.Marshal(new)
		if err != nil {
			return wh.Errored(http.StatusBadRequest, err)
		}

		return wh.PatchResponseFromRaw(req.Object.Raw, json)
	} else {
		return wh.Allowed("")
	}
}
