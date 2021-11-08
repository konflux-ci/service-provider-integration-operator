package webhooks

import (
	"context"
	"encoding/json"
	"net/http"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/vault"
	adm "k8s.io/api/admission/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	wh "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/check-token,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokens,verbs=delete,versions=v1beta1,name=spiaccesstoken-wh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}
//+kubebuilder:webhook:path=/check-token,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokens,verbs=create;update,versions=v1beta1,name=spiaccesstoken-wh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}

type SPIAccessTokenWebhook struct {
	Client  client.Client
	Vault   *vault.Vault
	decoder *wh.Decoder
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
	case adm.Delete:
		return w.handleDelete(ctx, token)
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
	if t.Spec.RawTokenData != nil {
		if err := w.Vault.Store(t, t.Spec.RawTokenData); err != nil {
			webhooklog.Error(err, "failed to store token into Vault", "object", client.ObjectKeyFromObject(t))
			return wh.Denied(err.Error())
		}
		t.Spec.RawTokenData = nil
		// We cannot update the status with the location here. That needs to wait for the controller
	}

	json, err := json.Marshal(t)
	if err != nil {
		return wh.Errored(http.StatusInternalServerError, err)
	}

	return wh.PatchResponseFromRaw(req.Object.Raw, json)
}

func (w *SPIAccessTokenWebhook) handleUpdate(ctx context.Context, req wh.Request, oldToken *api.SPIAccessToken, newToken *api.SPIAccessToken) wh.Response {
	return w.handleCreate(ctx, req, newToken)
}

func (w *SPIAccessTokenWebhook) handleDelete(ctx context.Context, token *api.SPIAccessToken) wh.Response {
	if err := w.Vault.Delete(token); err != nil {
		return wh.Denied(err.Error())
	}
	return wh.Allowed("")
}
