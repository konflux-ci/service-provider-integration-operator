package webhooks

import (
	"context"
	"fmt"
	"net/http"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	authv1 "k8s.io/api/authentication/v1"
	authzv1 "k8s.io/api/authorization/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	wh "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/check-binding,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=spiaccesstokenbindings,verbs=create;update,versions=v1beta1,name=spiaccesstokenbinding-wh.appstudio.redhat.com,admissionReviewVersions={v1,v1beta1}

type SPIAccessTokenBindingWebhook struct {
	Client  client.Client
	decoder *wh.Decoder
}

var _ wh.DecoderInjector = (*SPIAccessTokenBindingWebhook)(nil)
var _ wh.Handler = (*SPIAccessTokenBindingWebhook)(nil)

func (w *SPIAccessTokenBindingWebhook) InjectDecoder(dec *wh.Decoder) error {
	w.decoder = dec
	return nil
}

func (w *SPIAccessTokenBindingWebhook) SetupWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/check-binding", w.asWebhook())
	return nil
}

func (w *SPIAccessTokenBindingWebhook) asWebhook() *wh.Webhook {
	return &wh.Webhook{
		Handler: w,
	}
}

func (w *SPIAccessTokenBindingWebhook) Handle(ctx context.Context, req wh.Request) wh.Response {
	if err := w.checkCanReadAccessTokens(ctx, req.Namespace, req.UserInfo); err != nil {
		return wh.Errored(http.StatusForbidden, err)
	}

	return wh.Allowed("")
}

func (w *SPIAccessTokenBindingWebhook) checkCanReadAccessTokens(ctx context.Context, ns string, user authv1.UserInfo) error {
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
