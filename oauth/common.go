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

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/infrastructure"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"

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
	Config           config.ServiceProviderConfiguration
	K8sClient        AuthenticatingClient
	TokenStorage     tokenstorage.TokenStorage
	Endpoint         oauth2.Endpoint
	BaseUrl          string
	RedirectTemplate *template.Template
	Authenticator    *Authenticator
	StateStorage     *StateStorage
}

// exchangeState is the state that we're sending out to the SP after checking the anonymous oauth state produced by
// the operator as the initial OAuth URL. Notice that the state doesn't contain any sensitive information.
type exchangeState struct {
	oauthstate.OAuthInfo
}

// exchangeResult this the result of the OAuth exchange with all the data necessary to store the token into the storage
type exchangeResult struct {
	exchangeState
	result              oauthFinishResult
	token               *oauth2.Token
	authorizationHeader string
}

// redirectUrl constructs the URL to the callback endpoint so that it can be handled by this controller.
func (c *commonController) redirectUrl() string {
	return strings.TrimSuffix(c.BaseUrl, "/") + "/" + strings.ToLower(string(c.Config.ServiceProviderType)) + "/callback"
}

func (c *commonController) Authenticate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lg := log.FromContext(ctx)

	defer logs.TimeTrack(lg, time.Now(), "/authenticate")

	stateString := r.FormValue("state")

	state, err := oauthstate.ParseOAuthInfo(stateString)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusBadRequest, "failed to decode the OAuth state", err)
		return
	}
	ctx = infrastructure.InitKcpContext(ctx, state.TokenKcpWorkspace)
	token, err := c.Authenticator.GetToken(ctx, r)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusUnauthorized, "No active session was found. Please use `/login` method to authorize your request and try again. Or provide the token as a `k8s_token` query parameter.", err)
		return
	}
	ctx = WithAuthIntoContext(token, ctx)

	hasAccess, err := c.checkIdentityHasAccess(ctx, state)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to determine if the authenticated user has access", err)
		lg.Error(err, "The token is incorrect or the SPI OAuth service is not configured properly "+
			"and the API_SERVER environment variable points it to the incorrect Kubernetes API server. "+
			"If SPI is running with Devsandbox Proxy or KCP, make sure this env var points to the Kubernetes API proxy,"+
			" otherwise unset this variable. See more https://github.com/redhat-appstudio/infra-deployments/pull/264")
		return
	}

	if !hasAccess {
		LogDebugAndWriteResponse(ctx, w, http.StatusUnauthorized, "authenticating the request in Kubernetes unsuccessful")
		return
	}
	AuditLogWithTokenInfo(ctx, "OAuth authentication flow started", state.TokenNamespace, state.TokenName, "scopes", state.Scopes, "providerType", string(state.ServiceProviderType), "providerUrl", state.ServiceProviderUrl, "kcpWorkspace", state.TokenKcpWorkspace)
	newStateString, err := c.StateStorage.VeilRealState(r)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusBadRequest, err.Error(), err)
		return
	}
	keyedState := exchangeState{
		OAuthInfo: state,
	}

	oauthCfg, oauthConfigErr := c.obtainOauthConfig(ctx, &state)
	if oauthConfigErr != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to create oauth confgiuration", oauthConfigErr)
		return
	}
	oauthCfg.Scopes = keyedState.Scopes

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

func (c *commonController) Callback(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	lg := log.FromContext(ctx)
	defer logs.TimeTrack(lg, time.Now(), "/callback")

	exchange, err := c.finishOAuthExchange(ctx, r)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusBadRequest, "error in Service Provider token exchange", err)
		return
	}
	ctx = infrastructure.InitKcpContext(ctx, exchange.TokenKcpWorkspace)

	if exchange.result == oauthFinishK8sAuthRequired {
		LogErrorAndWriteResponse(ctx, w, http.StatusUnauthorized, "could not authenticate to Kubernetes", err)
		return
	}

	err = c.syncTokenData(ctx, &exchange)
	if err != nil {
		LogErrorAndWriteResponse(ctx, w, http.StatusInternalServerError, "failed to store token data to cluster", err)
		return
	}
	AuditLogWithTokenInfo(ctx, "OAuth authentication completed successfully", exchange.TokenNamespace, exchange.TokenName, "scopes", exchange.Scopes, "providerType", string(exchange.ServiceProviderType), "providerUrl", exchange.ServiceProviderUrl, "kcpWorkspace", exchange.TokenKcpWorkspace)
	redirectLocation := r.FormValue("redirect_after_login")
	if redirectLocation == "" {
		redirectLocation = strings.TrimSuffix(c.BaseUrl, "/") + "/" + "callback_success"
	}
	http.Redirect(w, r, redirectLocation, http.StatusFound)
}

// finishOAuthExchange implements the bulk of the Callback function. It returns the token, if obtained, the decoded
// state from the oauth flow, if available, and the result of the authentication.
func (c *commonController) finishOAuthExchange(ctx context.Context, r *http.Request) (exchangeResult, error) {
	// TODO support the implicit flow here, too?

	// check that the state is correct
	stateString, err := c.StateStorage.UnveilState(ctx, r)
	if err != nil {
		return exchangeResult{result: oauthFinishError}, fmt.Errorf("failed to unveil token state: %w", err)
	}

	state := &exchangeState{}
	err = oauthstate.ParseInto(stateString, state)
	if err != nil {
		return exchangeResult{result: oauthFinishError}, fmt.Errorf("failed to parse JWT state string: %w", err)
	}
	ctx = infrastructure.InitKcpContext(ctx, state.TokenKcpWorkspace)

	k8sToken, err := c.Authenticator.GetToken(ctx, r)
	if err != nil {
		return exchangeResult{result: oauthFinishK8sAuthRequired}, noActiveSessionError
	}
	ctx = WithAuthIntoContext(k8sToken, ctx)

	// the state is ok, let's retrieve the token from the service provider
	oauthCfg, oauthConfigErr := c.obtainOauthConfig(ctx, &state.OAuthInfo)
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
		exchangeState:       *state,
		result:              oauthFinishAuthenticated,
		token:               token,
		authorizationHeader: k8sToken,
	}, nil
}

// syncTokenData stores the data of the token to the configured TokenStorage.
func (c *commonController) syncTokenData(ctx context.Context, exchange *exchangeResult) error {
	ctx = WithAuthIntoContext(exchange.authorizationHeader, ctx)

	accessToken := &v1beta1.SPIAccessToken{}
	if err := c.K8sClient.Get(ctx, client.ObjectKey{Name: exchange.TokenName, Namespace: exchange.TokenNamespace}, accessToken); err != nil {
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

func (c *commonController) checkIdentityHasAccess(ctx context.Context, state oauthstate.OAuthInfo) (bool, error) {
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

	if err := c.K8sClient.Create(ctx, &review); err != nil {
		return false, fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}

	log.FromContext(ctx).V(logs.DebugLevel).Info("self subject review result", "review", &review)
	return review.Status.Allowed, nil
}
