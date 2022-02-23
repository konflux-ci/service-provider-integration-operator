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
	"net/http"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	adm "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	wh "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var spiAccessTokenLog = logf.Log.WithName("spiaccesstoken-webhook")

//+kubebuilder:webhook:path=/check-token,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokens,verbs=delete,versions=v1beta1,name=spiaccesstoken-vwh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}
//+kubebuilder:webhook:path=/check-token,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokens,verbs=create;update,versions=v1beta1,name=spiaccesstoken-mwh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}

type SPIAccessTokenWebhook struct {
	Client       client.Client
	TokenStorage tokenstorage.TokenStorage
	decoder      *wh.Decoder
}

var _ wh.DecoderInjector = (*SPIAccessTokenWebhook)(nil)
var _ wh.Handler = (*SPIAccessTokenWebhook)(nil)

func (w *SPIAccessTokenWebhook) SetupWithManager(mgr ctrl.Manager) error {
	webhook := w.asWebhook()
	mgr.GetWebhookServer().Register("/check-token", webhook)
	return nil
}

func (w *SPIAccessTokenWebhook) Handle(ctx context.Context, req wh.Request) wh.Response {
	token := &api.SPIAccessToken{}
	obj := req.Object
	if req.Operation == adm.Delete {
		obj = req.OldObject
	}

	if err := w.decoder.DecodeRaw(obj, token); err != nil {
		return wh.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case adm.Create:
		return w.handleCreate(ctx, req, token)
	case adm.Update:
		oldToken := &api.SPIAccessToken{}
		if err := w.decoder.DecodeRaw(req.OldObject, oldToken); err != nil {
			return wh.Errored(http.StatusBadRequest, err)
		}
		return w.handleUpdate(ctx, req, oldToken, token)
	}

	return wh.Allowed("")
}

func (w *SPIAccessTokenWebhook) InjectDecoder(dec *wh.Decoder) error {
	w.decoder = dec
	return nil
}

func (w *SPIAccessTokenWebhook) asWebhook() *wh.Webhook {
	return &wh.Webhook{
		Handler: w,
	}
}

func (w *SPIAccessTokenWebhook) handleCreate(ctx context.Context, req wh.Request, t *api.SPIAccessToken) wh.Response {
	changed := false

	if t.Spec.RawTokenData != nil {
		dataLocation, err := w.TokenStorage.Store(ctx, t, t.Spec.RawTokenData)
		if err != nil {
			spiAccessTokenLog.Error(err, "failed to store token into TokenStorage", "object", client.ObjectKeyFromObject(t))
			return wh.Denied(err.Error())
		}
		t.Spec.RawTokenData = nil
		t.Spec.DataLocation = dataLocation
		changed = true
	}

	changed = t.EnsureLabels() || changed

	if changed {
		json, err := json.Marshal(t)
		if err != nil {
			return wh.Errored(http.StatusInternalServerError, err)
		}

		return wh.PatchResponseFromRaw(req.Object.Raw, json)
	} else {
		return wh.Allowed("")
	}
}

func (w *SPIAccessTokenWebhook) handleUpdate(ctx context.Context, req wh.Request, oldToken *api.SPIAccessToken, newToken *api.SPIAccessToken) wh.Response {
	return w.handleCreate(ctx, req, newToken)
}
