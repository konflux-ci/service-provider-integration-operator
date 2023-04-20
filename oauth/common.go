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
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/kubernetesclient"

	"github.com/redhat-appstudio/service-provider-integration-operator/oauth/clientfactory"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"

	v1 "k8s.io/api/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/tokenstorage"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	noActiveSessionError = errors.New("no active oauth session found")
)

// commonController is the implementation of the Controller interface that assumes typical OAuth flow.
type commonController struct {
	OAuthServiceConfiguration
	ClientFactory       kubernetesclient.K8sClientFactory
	InClusterK8sClient  client.Client
	TokenStorage        tokenstorage.TokenStorage
	RedirectTemplate    *template.Template
	Authenticator       *Authenticator
	StateStorage        StateStorage
	ServiceProviderType config.ServiceProviderType
}

// exchangeResult this the result of the OAuth exchange with all the data necessary to store the token into the storage
type exchangeResult struct {
	oauthstate.OAuthInfo
	result              oauthFinishResult
	token               *oauth2.Token
	authorizationHeader string
}

// redirectUrl constructs the URL to the callback endpoint so that it can be handled by this controller.
func (c *commonController) redirectUrl() string {
	return strings.TrimSuffix(c.OAuthServiceConfiguration.BaseUrl, "/") + oauth.CallBackRoutePath
}

func (c *commonController) Authenticate(w http.ResponseWriter, r *http.Request, state *oauthstate.OAuthInfo) {
	ctx := r.Context()
	lg := log.FromContext(ctx)

	defer logs.TimeTrack(lg, time.Now(), "/authenticate")

	token, err := c.Authenticator.GetToken(ctx, r)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusUnauthorized, "No active session was found. Please use `/login` method to authorize your request and try again. Or provide the token as a `k8s_token` query parameter.", err)
		return
	}
	ctx = clientfactory.WithAuthIntoContext(token, ctx)

	hasAccess, err := c.checkIdentityHasAccess(ctx, state)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to determine if the authenticated user has access", err)
		lg.Error(err, "The token is incorrect or the SPI OAuth service is not configured properly "+
			"and the API_SERVER environment variable points it to the incorrect Kubernetes API server. "+
			"If SPI is running with Devsandbox Proxy, make sure this env var points to the Kubernetes API proxy,"+
			" otherwise unset this variable. See more https://github.com/redhat-appstudio/infra-deployments/pull/264")
		return
	}

	if !hasAccess {
		LogDebugAndWriteResponse(ctx, w, http.StatusUnauthorized, "authenticating the request in Kubernetes unsuccessful")
		return
	}
	AuditLogWithTokenInfo(ctx, "OAuth authentication flow started", state.TokenNamespace, state.TokenName, "scopes", state.Scopes, "providerName", state.ServiceProviderName, "providerUrl", state.ServiceProviderUrl)
	newStateString, err := c.StateStorage.VeilRealState(r)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusBadRequest, err.Error(), err)
		return
	}

	oauthCfg, oauthConfigErr := c.obtainOauthConfig(ctx, state)
	if oauthConfigErr != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to create oauth confgiuration", oauthConfigErr)
		return
	}
	oauthCfg.Scopes = state.Scopes

	templateData := struct {
		Url string
	}{
		Url: oauthCfg.AuthCodeURL(newStateString),
	}
	lg.V(logs.DebugLevel).Info("Redirecting ", "url", templateData.Url)
	err = c.RedirectTemplate.Execute(w, templateData)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to return redirect notice HTML page", err)
		return
	}
}

func (c *commonController) Callback(ctx context.Context, w http.ResponseWriter, r *http.Request, state *oauthstate.OAuthInfo) {
	lg := log.FromContext(ctx)
	defer logs.TimeTrack(lg, time.Now(), "/callback")

	exchange, err := c.finishOAuthExchange(ctx, r, state)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusBadRequest, "error in Service Provider token exchange", err)
		return
	}

	if exchange.result == oauthFinishK8sAuthRequired {
		LogErrorAndWriteResponse(ctx, w, http.StatusUnauthorized, "could not authenticate to Kubernetes", err)
		return
	}

	err = c.syncTokenData(ctx, &exchange)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to store token data to cluster", err)
		return
	}
	AuditLogWithTokenInfo(ctx, "OAuth authentication completed successfully", exchange.TokenNamespace, exchange.TokenName, "scopes", exchange.Scopes, "providerName", exchange.ServiceProviderName, "providerUrl", exchange.ServiceProviderUrl)
	redirectLocation := r.FormValue("redirect_after_login")
	if redirectLocation == "" {
		redirectLocation = strings.TrimSuffix(c.SharedConfiguration.BaseUrl, "/") + "/" + "callback_success"
	}
	http.Redirect(w, r, redirectLocation, http.StatusFound)
}

// finishOAuthExchange implements the bulk of the Callback function. It returns the token, if obtained, the decoded
// state from the oauth flow, if available, and the result of the authentication.
func (c *commonController) finishOAuthExchange(ctx context.Context, r *http.Request, state *oauthstate.OAuthInfo) (exchangeResult, error) {
	// TODO support the implicit flow here, too?

	// check that the state is correct
	stateString, err := c.StateStorage.UnveilState(ctx, r)
	if err != nil {
		return exchangeResult{result: oauthFinishError}, fmt.Errorf("failed to unveil token state: %w", err)
	}

	err = oauthstate.ParseInto(stateString, state)
	if err != nil {
		return exchangeResult{result: oauthFinishError}, fmt.Errorf("failed to parse JWT state string: %w", err)
	}

	k8sToken, err := c.Authenticator.GetToken(ctx, r)
	if err != nil {
		return exchangeResult{result: oauthFinishK8sAuthRequired}, noActiveSessionError
	}
	ctx = clientfactory.WithAuthIntoContext(k8sToken, ctx)

	// the state is ok, let's retrieve the token from the service provider
	oauthCfg, oauthConfigErr := c.obtainOauthConfig(ctx, state)
	if oauthConfigErr != nil {
		return exchangeResult{result: oauthFinishError}, fmt.Errorf("failed to obtain oauth configuration: %w", oauthConfigErr)
	}

	code := r.FormValue("code")

	// adding scopes to code exchange request is little out of spec, but quay wants them,
	// while other providers will just ignore this parameter
	scopeOption := oauth2.SetAuthURLParam("scope", r.FormValue("scope"))
	token, err := oauthCfg.Exchange(ctx, code, scopeOption)
	if err != nil {
		return exchangeResult{result: oauthFinishError}, fmt.Errorf("failed to finish the OAuth exchange: %w", err)
	}
	return exchangeResult{
		OAuthInfo:           *state,
		result:              oauthFinishAuthenticated,
		token:               token,
		authorizationHeader: k8sToken,
	}, nil
}

// syncTokenData stores the data of the token to the configured TokenStorage.
func (c *commonController) syncTokenData(ctx context.Context, exchange *exchangeResult) error {
	ctx = clientfactory.WithAuthIntoContext(exchange.authorizationHeader, ctx)

	accessToken := &v1beta1.SPIAccessToken{}
	ctx = clientfactory.NamespaceIntoContext(ctx, exchange.TokenNamespace)
	k8sClient, err := c.ClientFactory.CreateClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create K8S client for namespace %s: %w", exchange.TokenNamespace, err)
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: exchange.TokenName, Namespace: exchange.TokenNamespace}, accessToken); err != nil {
		return fmt.Errorf("failed to get the SPIAccessToken object %s/%s: %w", exchange.TokenNamespace, exchange.TokenName, err)
	}

	apiToken := v1beta1.Token{
		AccessToken:  exchange.token.AccessToken,
		TokenType:    exchange.token.TokenType,
		RefreshToken: exchange.token.RefreshToken,
		Expiry:       uint64(exchange.token.Expiry.Unix()),
	}

	if err := c.TokenStorage.Store(ctx, accessToken, &apiToken); err != nil {
		return fmt.Errorf("failed to persist the token to storage: %w", err)
	}

	return nil
}

func (c *commonController) checkIdentityHasAccess(ctx context.Context, state *oauthstate.OAuthInfo) (bool, error) {
	review := v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: state.TokenNamespace,
				Verb:      "create",
				Group:     v1beta1.GroupVersion.Group,
				Version:   v1beta1.GroupVersion.Version,
				Resource:  "spiaccesstokendataupdates",
			},
		},
	}

	k8sClient, err := c.ClientFactory.CreateClient(clientfactory.NamespaceIntoContext(ctx, state.TokenNamespace))
	if err != nil {
		return false, fmt.Errorf("failed to create K8S client for namespace %s: %w", state.TokenNamespace, err)
	}
	if err := k8sClient.Create(ctx, &review); err != nil {
		return false, fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}

	log.FromContext(ctx).V(logs.DebugLevel).Info("self subject review result", "review", &review)
	return review.Status.Allowed, nil
}
