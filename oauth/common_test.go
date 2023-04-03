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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/serviceprovider/oauth"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"golang.org/x/oauth2"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Controller", func() {

	createAnonymousState := func() *oauthstate.OAuthInfo {
		return &oauthstate.OAuthInfo{
			TokenName:           "mytoken",
			TokenNamespace:      IT.Namespace,
			Scopes:              []string{"a", "b"},
			ServiceProviderName: config.ServiceProviderTypeGitHub.Name,
			ServiceProviderUrl:  "https://special.sp",
		}
	}

	prepareAnonymousState := func() string {
		ret, err := oauthstate.Encode(createAnonymousState())
		Expect(err).NotTo(HaveOccurred())
		return ret
	}

	grabK8sToken := func(g Gomega) string {
		var result string
		//Obtaining token implemented with fallback-retry approach because
		//for OpenShift token delivering may take up to 1-3 seconds,
		//for Minikube that happens almost instantaneously.
		g.Eventually(func(gg Gomega) bool {
			var err error
			tokenRequest := &authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{},
			}
			response, err := IT.Clientset.CoreV1().ServiceAccounts(IT.Namespace).CreateToken(context.TODO(), "default", tokenRequest, metav1.CreateOptions{})
			gg.Expect(err).NotTo(HaveOccurred())
			gg.Expect(response.Status.Token).NotTo(BeEmpty())

			result = response.Status.Token
			return true
		}).Should(BeTrue(), "Could not find the token of the default service account in the test namespace")
		return result
	}

	config.SupportedServiceProviderTypes = []config.ServiceProviderType{
		{
			Name:           "special",
			DefaultHost:    "special.sp",
			DefaultBaseUrl: "https://special.sp",
			DefaultOAuthEndpoint: oauth2.Endpoint{
				AuthURL:  "https://special.sp/auth",
				TokenURL: "https://special.sp/token",
			},
		},
	}

	prepareAuthenticator := func(g Gomega) *Authenticator {
		return NewAuthenticator(IT.SessionManager, IT.ClientFactory)
	}
	prepareController := func(g Gomega) *commonController {
		tmpl, err := template.ParseFiles("../static/redirect_notice.html")
		g.Expect(err).NotTo(HaveOccurred())
		return &commonController{
			OAuthServiceConfiguration: OAuthServiceConfiguration{
				SharedConfiguration: config.SharedConfiguration{
					ServiceProviders: []config.ServiceProviderConfiguration{
						{
							ServiceProviderBaseUrl: "https://special.sp",
							OAuth2Config: &oauth2.Config{
								ClientID:     "clientId",
								ClientSecret: "clientSecret",
								Endpoint: oauth2.Endpoint{
									AuthURL:   "https://special.sp/login",
									TokenURL:  "https://special.sp/toekn",
									AuthStyle: oauth2.AuthStyleAutoDetect,
								},
							},
							ServiceProviderType: config.ServiceProviderTypeGitHub,
						}},
					BaseUrl: "https://spi.on.my.machine",
				},
			},
			ClientFactory:       IT.ClientFactory,
			InClusterK8sClient:  IT.InClusterClient,
			TokenStorage:        IT.TokenStorage,
			Authenticator:       prepareAuthenticator(g),
			RedirectTemplate:    tmpl,
			StateStorage:        NewStateStorage(IT.SessionManager),
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}
	}

	loginFlow := func(g Gomega) (*Authenticator, *httptest.ResponseRecorder) {
		token := grabK8sToken(g)

		// This is the setup for the HTTP call to /oauth/authenticate
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()

		c := prepareAuthenticator(g)

		IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Login(w, r)
		})).ServeHTTP(res, req)

		return c, res
	}

	authenticateFlow := func(g Gomega, cookies []*http.Cookie) (*commonController, string, *httptest.ResponseRecorder) {
		spiState := prepareAnonymousState()
		req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s", spiState), nil)
		for _, cookie := range cookies {
			req.Header.Set("Cookie", cookie.String())
		}

		res := httptest.NewRecorder()

		c := prepareController(g)

		IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Authenticate(w, r, createAnonymousState())
		})).ServeHTTP(res, req)

		return c, spiState, res
	}

	authenticateFlowQueryParam := func(g Gomega) (*commonController, string, *httptest.ResponseRecorder) {
		token := grabK8sToken(g)
		spiState := prepareAnonymousState()
		req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s&k8s_token=%s", spiState, token), nil)

		res := httptest.NewRecorder()

		c := prepareController(g)

		IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Authenticate(w, r, createAnonymousState())
		})).ServeHTTP(res, req)

		return c, spiState, res
	}

	getRedirectUrlFromAuthenticateResponse := func(g Gomega, res *httptest.ResponseRecorder) *url.URL {
		body, err := ioutil.ReadAll(res.Result().Body)
		g.Expect(err).NotTo(HaveOccurred())
		re, err := regexp.Compile("<meta http-equiv = \"refresh\" content = \"2; url=([^\"]+)\"")
		g.Expect(err).NotTo(HaveOccurred())
		matches := re.FindSubmatch(body)
		g.Expect(matches).To(HaveLen(2))

		unescaped := html.UnescapeString(string(matches[1]))

		redirect, err := url.Parse(unescaped)
		g.Expect(err).NotTo(HaveOccurred())
		return redirect
	}

	It("authenticate in POST", func() {
		token := grabK8sToken(Default)

		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()

		c := prepareAuthenticator(Default)
		IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Login(w, r)
		})).ServeHTTP(res, req)

		Expect(res.Code).To(Equal(http.StatusOK))
		Expect(res.Result().Cookies()).NotTo(BeEmpty())
	})

	It("invalidates the user session cookie", func() {

		req := httptest.NewRequest("GET", "/logout", nil)
		res := httptest.NewRecorder()

		c := prepareAuthenticator(Default)
		IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Logout(w, r)
		})).ServeHTTP(res, req)

		Expect(res.Code).To(Equal(http.StatusOK))
		Expect(res.Result().Cookies()).To(HaveLen(1))
		Expect(res.Result().Cookies()[0].Value).To(BeEmpty())
		Expect(res.Result().Cookies()[0].MaxAge).To(BeNumerically("<=", 0))
		Expect(res.Result().Cookies()[0].Expires).To(BeTemporally("<=", time.Now()))
	})

	It("redirects to SP OAuth URL with state and scopes", func() {
		_, res := loginFlow(Default)
		Expect(res.Code).To(Equal(http.StatusOK))
		_, spiState, res := authenticateFlow(Default, res.Result().Cookies())
		redirect := getRedirectUrlFromAuthenticateResponse(Default, res)

		Expect(redirect.Scheme).To(Equal("https"))
		Expect(redirect.Host).To(Equal("special.sp"))
		Expect(redirect.Path).To(Equal("/login"))
		Expect(redirect.Query().Get("client_id")).To(Equal("clientId"))
		Expect(redirect.Query().Get("redirect_uri")).To(Equal("https://spi.on.my.machine" + oauth.CallBackRoutePath))
		Expect(redirect.Query().Get("response_type")).To(Equal("code"))
		Expect(redirect.Query().Get("state")).NotTo(BeEmpty())
		Expect(redirect.Query().Get("state")).NotTo(Equal(spiState))
		Expect(redirect.Query().Get("scope")).To(Equal("a b"))
		Expect(res.Result().Cookies()).NotTo(BeEmpty())
		cookie := res.Result().Cookies()[0]
		Expect(cookie.Name).To(Equal("appstudio_spi_session"))
	})

	It("redirects to SP OAuth URL with state and scopes. Alternative login", func() {
		_, spiState, res := authenticateFlowQueryParam(Default)
		redirect := getRedirectUrlFromAuthenticateResponse(Default, res)

		Expect(redirect.Scheme).To(Equal("https"))
		Expect(redirect.Host).To(Equal("special.sp"))
		Expect(redirect.Path).To(Equal("/login"))
		Expect(redirect.Query().Get("client_id")).To(Equal("clientId"))
		Expect(redirect.Query().Get("redirect_uri")).To(Equal("https://spi.on.my.machine" + oauth.CallBackRoutePath))
		Expect(redirect.Query().Get("response_type")).To(Equal("code"))
		Expect(redirect.Query().Get("state")).NotTo(BeEmpty())
		Expect(redirect.Query().Get("state")).NotTo(Equal(spiState))
		Expect(redirect.Query().Get("scope")).To(Equal("a b"))
		Expect(res.Result().Cookies()).NotTo(BeEmpty())
		cookie := res.Result().Cookies()[0]
		Expect(cookie.Name).To(Equal("appstudio_spi_session"))
	})

	When("OAuth initiated", func() {
		BeforeEach(func() {
			Expect(IT.InClusterClient.Create(IT.Context, &v1beta1.SPIAccessToken{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mytoken",
					Namespace: IT.Namespace,
				},
				Spec: v1beta1.SPIAccessTokenSpec{
					ServiceProviderUrl: "https://special.sp",
				},
			})).To(Succeed())

			t := &v1beta1.SPIAccessToken{}
			Eventually(func() error {
				return IT.InClusterClient.Get(IT.Context, client.ObjectKey{Name: "mytoken", Namespace: IT.Namespace}, t)
			}).Should(Succeed())
		})

		AfterEach(func() {
			t := &v1beta1.SPIAccessToken{}
			Expect(IT.InClusterClient.Get(IT.Context, client.ObjectKey{Name: "mytoken", Namespace: IT.Namespace}, t)).To(Succeed())
			Expect(IT.InClusterClient.Delete(IT.Context, t)).To(Succeed())
			Eventually(func() error {
				return IT.InClusterClient.Get(IT.Context, client.ObjectKey{Name: "mytoken", Namespace: IT.Namespace}, t)
			}).ShouldNot(Succeed())
		})

		It("exchanges the code for token", func() {
			// this may fail at times because we're updating the token during the flow and we may intersect with
			// operator's work. Wrapping it in an Eventually block makes sure we retry on such occurrences. Note that
			// the need for this will disappear once we don't update the token anymore from OAuth service (which is
			// the plan).
			Eventually(func(g Gomega) {
				_, res := loginFlow(g)
				sessionCookies := res.Result().Cookies()
				controller, _, res := authenticateFlow(g, sessionCookies)

				redirect := getRedirectUrlFromAuthenticateResponse(Default, res)

				state := redirect.Query().Get("state")

				// simulate github redirecting back to our callback endpoint...
				req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s&code=123", state), nil)
				for _, cookie := range sessionCookies {
					req.Header.Set("Cookie", cookie.String())
				}
				res = httptest.NewRecorder()

				// The callback handler will be reaching out to github to exchange the code for the token.. let's fake that
				// response...
				bakedResponse, _ := json.Marshal(oauth2.Token{
					AccessToken:  "token",
					TokenType:    "jwt",
					RefreshToken: "refresh",
					Expiry:       time.Now(),
				})
				serviceProviderReached := false
				ctxVal := &http.Client{
					Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
						if strings.HasPrefix(r.URL.String(), "https://special.sp") {
							serviceProviderReached = true
							return &http.Response{
								StatusCode: 200,
								Header:     http.Header{},
								Body:       ioutil.NopCloser(bytes.NewBuffer(bakedResponse)),
								Request:    r,
							}, nil
						}

						return nil, fmt.Errorf("unexpected request to: %s", r.URL.String())
					}),
				}

				IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					controller.Callback(context.WithValue(r.Context(), oauth2.HTTPClient, ctxVal), w, r, createAnonymousState())
				})).ServeHTTP(res, req)

				g.Expect(res.Code).To(Equal(http.StatusFound))
				Expect(serviceProviderReached).To(BeTrue())
			}).Should(Succeed())
		})

		It("redirects to specified url", func() {
			// this may fail at times because we're updating the token during the flow and we may intersect with
			// operator's work. Wrapping it in an Eventually block makes sure we retry on such occurrences. Note that
			// the need for this will disappear once we don't update the token anymore from OAuth service (which is
			// the plan).
			Eventually(func(g Gomega) {
				_, res := loginFlow(g)
				sessionCookies := res.Result().Cookies()
				controller, _, res := authenticateFlow(g, res.Result().Cookies())

				redirect := getRedirectUrlFromAuthenticateResponse(Default, res)

				state := redirect.Query().Get("state")

				// simulate github redirecting back to our callback endpoint...
				req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s&code=123&redirect_after_login=https://redirect.to?foo=bar", state), nil)
				for _, cookie := range sessionCookies {
					req.Header.Set("Cookie", cookie.String())
				}
				res = httptest.NewRecorder()

				// The callback handler will be reaching out to github to exchange the code for the token.. let's fake that
				// response...
				bakedResponse, _ := json.Marshal(oauth2.Token{
					AccessToken:  "token",
					TokenType:    "jwt",
					RefreshToken: "refresh",
					Expiry:       time.Now(),
				})
				ctxVal := &http.Client{
					Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
						if strings.HasPrefix(r.URL.String(), "https://special.sp") {
							return &http.Response{
								StatusCode: 200,
								Header:     http.Header{},
								Body:       ioutil.NopCloser(bytes.NewBuffer(bakedResponse)),
								Request:    r,
							}, nil
						}

						return nil, fmt.Errorf("unexpected request to: %s", r.URL.String())
					}),
				}

				IT.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					controller.Callback(context.WithValue(r.Context(), oauth2.HTTPClient, ctxVal), w, r, createAnonymousState())
				})).ServeHTTP(res, req)

				g.Expect(res.Code).To(Equal(http.StatusFound))
				g.Expect(res.Result().Header.Get("Location")).To(Equal("https://redirect.to?foo=bar"))
			}).Should(Succeed())
		})
	})
})
