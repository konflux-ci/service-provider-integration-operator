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

package vaultstorage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/go-hclog"
	vault "github.com/hashicorp/vault/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/httptransport"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/secretstorage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type VaultSecretStorage struct {
	client       *vault.Client
	loginHandler *loginHandler
	// ignoreLoginHandler is used to switch off token renewal logic. Only used in tests!!!
	ignoreLoginHandler bool
	// Config holds the configuration of the storage. After the Initialize method is called, no changes
	// to this configuration object are reflected even if Initialize is called again.
	Config *VaultStorageConfig
}

const vaultDataPathFormat = "%s/data/%s/%s"

var (
	VaultError             = errors.New("error in Vault")
	noAuthInfoInVaultError = errors.New("no auth info returned from Vault")
	unexpectedDataError    = errors.New("unexpected data")
	unspecifiedStoreError  = errors.New("failed to store the token, no error but returned nil")

	vaultRequestCountMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: config.MetricsNamespace,
		Subsystem: config.MetricsSubsystem,
		Name:      "vault_request_count_total",
		Help:      "The request counts to Vault categorized by HTTP method status code",
	}, []string{"method", "status"})

	vaultResponseTimeMetric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: config.MetricsNamespace,
		Subsystem: config.MetricsSubsystem,
		Name:      "vault_response_time_seconds",
		Help:      "The response time of Vault requests categorized by HTTP method and status code",
	}, []string{"method", "status"})

	requestMetricConfig = httptransport.HttpMetricCollectionConfig{
		CounterPicker: httptransport.HttpCounterMetricPickerFunc(func(request *http.Request, resp *http.Response, err error) []prometheus.Counter {
			if resp == nil {
				return nil
			}
			return []prometheus.Counter{vaultRequestCountMetric.WithLabelValues(request.Method, strconv.Itoa(resp.StatusCode))}
		}),
		HistogramOrSummaryPicker: httptransport.HttpHistogramOrSummaryMetricPickerFunc(func(request *http.Request, resp *http.Response, err error) []prometheus.Observer {
			if resp == nil {
				return nil
			}
			return []prometheus.Observer{vaultResponseTimeMetric.WithLabelValues(request.Method, strconv.Itoa(resp.StatusCode))}
		}),
	}
)

type VaultAuthMethod string

const (
	VaultAuthMethodKubernetes VaultAuthMethod = "kubernetes"
	VaultAuthMethodApprole    VaultAuthMethod = "approle"
)

type VaultStorageConfig struct {
	Host     string `validate:"required,https_only"`
	AuthType VaultAuthMethod
	Insecure bool

	Role                        string
	ServiceAccountTokenFilePath string

	RoleIdFilePath   string
	SecretIdFilePath string

	MetricsRegisterer prometheus.Registerer

	DataPathPrefix string
}

func (v *VaultSecretStorage) Initialize(ctx context.Context) error {
	if err := v.initFields(); err != nil {
		return err
	}

	if err := v.login(ctx); err != nil {
		return err
	}

	if err := v.initMetrics(ctx); err != nil {
		return err
	}

	return nil
}

func (v *VaultSecretStorage) Store(ctx context.Context, id secretstorage.SecretID, bytes []byte) error {
	data := map[string]interface{}{
		// yes, the data HAS TO be a map
		"data": map[string]interface{}{
			"bytes": base64.StdEncoding.EncodeToString(bytes),
		},
	}
	lg := log.FromContext(ctx)
	path := fmt.Sprintf(vaultDataPathFormat, v.Config.DataPathPrefix, id.Namespace, id.Name)

	ctx = httptransport.ContextWithMetrics(ctx, &requestMetricConfig)
	s, err := v.client.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return fmt.Errorf("error writing the data to Vault: %w", err)
	}
	if s == nil {
		return unspecifiedStoreError
	}
	for _, w := range s.Warnings {
		lg.Info(w)
	}

	return nil
}

func (v *VaultSecretStorage) Get(ctx context.Context, id secretstorage.SecretID) ([]byte, error) {
	lg := log.FromContext(ctx)

	ctx = httptransport.ContextWithMetrics(ctx, &requestMetricConfig)

	path := fmt.Sprintf(vaultDataPathFormat, v.Config.DataPathPrefix, id.Namespace, id.Name)
	secret, err := v.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("error reading the data: %w", err)
	}
	if secret == nil || secret.Data == nil || len(secret.Data) == 0 || secret.Data["data"] == nil {
		lg.V(logs.DebugLevel).Info("no data found in vault at", "path", path)
		return nil, secretstorage.NotFoundError
	}
	for _, w := range secret.Warnings {
		lg.Info(w)
	}

	dataField, ok := secret.Data["data"]
	if !ok {
		return nil, fmt.Errorf("%w: data field not present in Vault response", unexpectedDataError)
	}
	dataMap, ok := dataField.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: data field not a map", unexpectedDataError)
	}

	bytesField, ok := dataMap["bytes"]
	if !ok {
		return nil, fmt.Errorf("%w: bytes field not present", unexpectedDataError)
	}
	bytesStr, ok := bytesField.(string)
	if !ok {
		return nil, fmt.Errorf("%w: bytes field is not string", unexpectedDataError)
	}
	bytes, err := base64.StdEncoding.DecodeString(bytesStr)
	if err != nil {
		return nil, fmt.Errorf("bytes field not base64 encoded: %w", err)
	}
	return bytes, nil
}

func (v *VaultSecretStorage) Delete(ctx context.Context, id secretstorage.SecretID) error {
	ctx = httptransport.ContextWithMetrics(ctx, &requestMetricConfig)

	path := fmt.Sprintf(vaultDataPathFormat, v.Config.DataPathPrefix, id.Namespace, id.Name)
	s, err := v.client.Logical().DeleteWithContext(ctx, path)
	if err != nil {
		return fmt.Errorf("error deleting the data: %w", err)
	}
	log.FromContext(ctx).V(logs.DebugLevel).Info("deleted", "secret", s)
	return nil
}

func (v *VaultSecretStorage) initFields() error {
	// These fields are only non-nil at the point in time they're called
	// from init if called from tests that pre-initialize these to work with
	// the fake Vault cluster.
	if v.client == nil {
		if err := config.ValidateStruct(v.Config); err != nil {
			return fmt.Errorf("error validating storage config: %w", err)
		}
		config := vault.DefaultConfig()
		config.Address = v.Config.Host
		config.Logger = hclog.Default()
		if v.Config.Insecure {
			if err := config.ConfigureTLS(&vault.TLSConfig{
				Insecure: true,
			}); err != nil {
				return fmt.Errorf("error configuring insecure TLS: %w", err)
			}
		}

		// This needs to be done AFTER configuring the TLS, because ConfigureTLS assumes that the transport is http.Transport
		// and not our round tripper.
		config.HttpClient.Transport = httptransport.HttpMetricCollectingRoundTripper{RoundTripper: config.HttpClient.Transport}

		vaultClient, err := vault.NewClient(config)
		if err != nil {
			return fmt.Errorf("error creating the client: %w", err)
		}
		v.client = vaultClient
	}

	if !v.ignoreLoginHandler && v.loginHandler == nil {
		authMethod, authErr := prepareAuth(v.Config)
		if authErr != nil {
			return fmt.Errorf("error preparing vault authentication: %w", authErr)
		}

		v.loginHandler = &loginHandler{
			client:     v.client,
			authMethod: authMethod,
		}
	}

	return nil
}

func (v *VaultSecretStorage) login(ctx context.Context) error {
	if v.loginHandler != nil {
		if err := v.loginHandler.Login(ctx); err != nil {
			return fmt.Errorf("failed to login to Vault: %w", err)
		}
	} else {
		log.FromContext(ctx).Info("no login handler configured for Vault - token refresh disabled")
	}

	return nil
}

func (v *VaultSecretStorage) initMetrics(ctx context.Context) error {
	if v.Config.MetricsRegisterer != nil {
		if err := v.Config.MetricsRegisterer.Register(vaultRequestCountMetric); err != nil {
			return fmt.Errorf("failed to register request count metric: %w", err)
		}

		if err := v.Config.MetricsRegisterer.Register(vaultResponseTimeMetric); err != nil {
			return fmt.Errorf("failed to register response time metric: %w", err)
		}
	} else {
		log.FromContext(ctx).Info("no metrics registry configured - metrics collection for Vault access is disabled")
	}

	return nil
}
