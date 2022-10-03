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

	// now, let's start the concurrent infinite loop that checks for the validity of the login token and does
	// the refresh or re-login, if needed. Because we're using the Vault client for all this work, it automatically
	// picks up any changes to the login token that we make.
	go h.loginLoop(ctx, authInfo)

	return nil
}

// loginLoop basically calls startRenew in an infinite loop, trying to re-login if startRenew tells it to.
func (h loginHandler) loginLoop(ctx context.Context, authInfo *vault.Secret) {
	lg := log.FromContext(ctx, "vaultLoginHandler", true)

	// we're trying to re-login with increasing pauses between the attempts (the pause is increased 10 times, but there
	// are infinite possible attempts).
	attemptsToGetRenewableToken := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Steps:    10,
	}
	for {
		var err error

		if authInfo == nil || !authInfo.Auth.Renewable {
			// it seems like we have a problem with the login, let's retry

			// we've already tried before, so let's wait now a bit before the next attempt.
			pause := attemptsToGetRenewableToken.Step()
			lg.V(logs.DebugLevel).Info("token not renewable, reattempting login", "pause", pause)
			time.Sleep(pause)

			authInfo, err = h.doLogin(ctx)
			if err != nil {
				lg.Error(err, "failed to login to vault after detecting the current token is not renewable")
			}

			continue
		} else {
			// ok, we're logged in successfully, so let's reset our back-off strategy so that we can start over the
			// next time we're seeing the login to fail.
			attemptsToGetRenewableToken.Steps = 10
		}

		// ok, we have a renewable token. Let's start the renewal loop. This only ever returns if we need to re-login
		// to Vault or if we should exit.
		var reLogin bool
		reLogin, err = h.startRenew(ctx, authInfo)
		if err != nil {
			lg.Error(err, "failed to run the Vault token renewal routine")
		}
		if !reLogin {
			// ok, we're suppposed to quit, so let's do...
			return
		}

		// the token renewal loop exited and told us we need to re-login to Vault. So let's do that now and check
		// the outcome back at the start of this loop.
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
	// Watcher is a Vault utility that renews the token if necessary and reports the results using a channel. It
	// makes sure the client that created the watcher gets updated with the renewed token automatically.
	watcher, err := h.client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: secret,
	})
	if err != nil {
		return true, fmt.Errorf("failed to construct Vault token lifetime watcher: %w", err)
	}

	lg := log.FromContext(ctx)

	// start the watcher
	go watcher.Start()
	defer watcher.Stop()

	for {
		// look for the results of the watcher operation and also for our context being cancelled/done.
		select {
		case <-ctx.Done():
			lg.Info("stopping the Vault token renewal routine because the context is done")
			return false, nil
		case err = <-watcher.DoneCh():
			// we enter here if the watcher detects it can no longer renew the token. We therefore exist and ask the caller
			// to try and log in again.
			return true, err
		case <-watcher.RenewCh():
			// yay, the login token is renewed. We can happily wait for another message in the next iteration of this
			// loop.
			lg.V(logs.DebugLevel).Info("successfully renewed the Vault token")
		}
	}
}
