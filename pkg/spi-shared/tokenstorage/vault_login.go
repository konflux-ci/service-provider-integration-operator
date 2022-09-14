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

package tokenstorage

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	vault "github.com/hashicorp/vault/api"
	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type loginHandler struct {
	client     *vault.Client
	authMethod vault.AuthMethod
}

// Login tries to log in to Vault and starts a background routine to renew the login token.
func (h loginHandler) Login(ctx context.Context) error {
	// we make the first login attempt in a blocking fashion so that we can bail out quickly if it is not possible.
	authInfo, err := h.doLogin(ctx)
	if err != nil {
		return err
	}

	go h.loginLoop(ctx, authInfo)

	return nil
}

// loginLoop basically calls startRenew in an infinite loop, trying to re-login if startRenew tells it to.
func (h loginHandler) loginLoop(ctx context.Context, authInfo *vault.Secret) {
	lg := log.FromContext(ctx, "vaultLoginHandler", true)
	getRenewableTokenBackoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Steps:    10,
	}

	attemptsToGetRenewableToken := getRenewableTokenBackoff
	for {
		var err error

		if authInfo == nil || !authInfo.Auth.Renewable {
			lg.V(logs.DebugLevel).Info("token not renewable, reattempting login")

			authInfo, err = h.doLogin(ctx)
			if err != nil {
				lg.Error(err, "failed to login to vault after detecting the current token is not renewable")
			}

			time.Sleep(attemptsToGetRenewableToken.Step())
			continue
		} else {
			attemptsToGetRenewableToken = getRenewableTokenBackoff
		}

		// ok, we have a renewable token
		var reLogin bool
		reLogin, err = h.startRenew(ctx, authInfo)
		if err != nil {
			lg.Error(err, "failed to run the Vault token renewal routine")
		}
		if !reLogin {
			return
		}

		authInfo, err = h.doLogin(ctx)
		if err != nil {
			lg.Error(err, "failed to login to Vault")
		}
	}
}

// doLogin performs a single login attempt and, after some basic error checking, returns the vault secret
func (h loginHandler) doLogin(ctx context.Context) (*vault.Secret, error) {
	authInfo, err := h.client.Auth().Login(ctx, h.authMethod)
	if err != nil {
		return nil, fmt.Errorf("error while authenticating: %w", err)
	}
	if authInfo == nil {
		return nil, noAuthInfoInVaultError
	}

	return authInfo, nil
}

// startRenew takes care of renewing the Vault token. This only returns in case of error or if a new login attempt
// needs to be made. Returns true if the re-login is possible, false if no more login attempts should be made (if the
// context is done).
func (h loginHandler) startRenew(ctx context.Context, secret *vault.Secret) (bool, error) {
	watcher, err := h.client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: secret,
	})
	if err != nil {
		return true, fmt.Errorf("failed to construct Vault token lifetime watcher: %w", err)
	}

	lg := log.FromContext(ctx)

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			lg.Info("stopping the Vault token renewal routine because the context is done")
			return false, nil
		case err = <-watcher.DoneCh():
			return true, err
		case <-watcher.RenewCh():
			lg.V(logs.DebugLevel).Info("successfully renewed the Vault token")
		}
	}
}
