package oauth

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var unknownServiceProviderError = errors.New("unknown service provider")

type Router struct {
	staticallyConfiguredControllers map[config.ServiceProviderType]map[string]Controller

	// unused as of now, will be probably used when we handle
	k8sClient    client.Client
	stateStorage *StateStorage
}

type CallbackRoute struct {
	router *Router
}

type AuthenticateRoute struct {
	router *Router
}

type RouterConfiguration struct {
	OAuthServiceConfiguration
	Authenticator    *Authenticator
	StateStorage     *StateStorage
	K8sClient        client.Client
	TokenStorage     tokenstorage.TokenStorage
	RedirectTemplate *template.Template
}

func NewRouter(lg *logr.Logger, cfg RouterConfiguration) (*Router, error) {
	controllersByTypeAndBaseUrl := map[config.ServiceProviderType]map[string]Controller{}
	for _, sp := range cfg.ServiceProviders {
		lg.Info("initializing service provider controller", "type", sp.ServiceProviderType, "url", sp.ServiceProviderBaseUrl)

		controller, err := FromConfiguration(cfg.OAuthServiceConfiguration, sp, cfg.Authenticator, cfg.StateStorage, cfg.K8sClient, cfg.TokenStorage, cfg.RedirectTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize controller for SP type %s on base URL %s: %w", sp.ServiceProviderType, sp.ServiceProviderBaseUrl, err)
		}

		controllersByTypeAndBaseUrl[sp.ServiceProviderType][sp.ServiceProviderBaseUrl] = controller
	}

	return &Router{
		staticallyConfiguredControllers: controllersByTypeAndBaseUrl,
		k8sClient:                       cfg.K8sClient,
		stateStorage:                    cfg.StateStorage,
	}, nil
}

func (r *Router) Callback() *CallbackRoute {
	return &CallbackRoute{router: r}
}

func (r *Router) Authenticate() *AuthenticateRoute {
	return &AuthenticateRoute{router: r}
}

func (r *Router) findController(req *http.Request, veiled bool) (Controller, *oauthstate.OAuthInfo, error) {
	var stateString string
	var err error

	if veiled {
		stateString, err = r.stateStorage.UnveilState(req.Context(), req)
		if err != nil {
			return nil, nil, err
		}
	} else {
		stateString = req.FormValue("state")
	}

	state := &oauthstate.OAuthInfo{}
	err = oauthstate.ParseInto(stateString, state)
	if err != nil {
		return nil, nil, err
	}

	controller := r.staticallyConfiguredControllers[state.ServiceProviderType][state.ServiceProviderUrl]
	if controller == nil {
		return nil, nil, fmt.Errorf("%w: type '%s', base URL '%s'", unknownServiceProviderError, state.ServiceProviderType, state.ServiceProviderUrl)
	}

	return controller, state, nil
}

func (r *CallbackRoute) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {
	ctrl, state, err := r.router.findController(req, true)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
		return
	}

	ctrl.Callback(req.Context(), wrt, req, state)
}

func (r *AuthenticateRoute) ServeHTTP(wrt http.ResponseWriter, req *http.Request) {
	ctrl, state, err := r.router.findController(req, false)
	if err != nil {
		LogErrorAndWriteResponse(req.Context(), wrt, http.StatusBadRequest, "failed to find the service provider", err)
		return
	}

	ctrl.Authenticate(wrt, req, state)
}
