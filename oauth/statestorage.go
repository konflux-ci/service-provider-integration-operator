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
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"time"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/logs"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/alexedwards/scs/v2"
)

// StateStorage aims to provide a link between SPI's state and OAuth state.
type StateStorage interface {
	// VeilRealState returns the random string that can be used as OAuth state.
	// Suppose to be reused to restore the original SPI's state on OAuth callback.
	VeilRealState(req *http.Request) (string, error)
	// UnveilState recover original SPI's state from OAuth callback request.
	UnveilState(ctx context.Context, req *http.Request) (string, error)
	// StateVeiledAt informs when the state was veiled.
	StateVeiledAt(ctx context.Context, req *http.Request) (time.Time, error)
}
type SessionStateStorage struct {
	sessionManager *scs.SessionManager
}

var _ StateStorage = (*SessionStateStorage)(nil)

var (
	noStateError                = errors.New("request has no `state` parameter")
	randomStringGenerationError = errors.New("not able to generate new random string")
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
)

func (s *SessionStateStorage) VeilRealState(req *http.Request) (string, error) {
	log := log.FromContext(req.Context())
	state := req.URL.Query().Get("state")
	if state == "" {
		log.Error(noStateError, "Request has no state parameter")
		return "", noStateError
	}
	newState, err := randStringBytes(32)
	if err != nil {

		return "", err
	}
	log.V(logs.DebugLevel).Info("State veiled", "state", state, "veil", newState)
	s.sessionManager.Put(req.Context(), newState, state)
	s.sessionManager.Put(req.Context(), newState+"-createdAt", time.Now().Unix())
	return newState, nil
}

func (s *SessionStateStorage) UnveilState(ctx context.Context, req *http.Request) (string, error) {
	log := log.FromContext(ctx)
	state := req.URL.Query().Get("state")
	if state == "" {
		log.Error(noStateError, "Request has no state parameter")
		return "", noStateError
	}
	unveiledState := s.sessionManager.GetString(ctx, state)
	log.V(logs.DebugLevel).Info("State unveiled", "veil", state, "unveiledState", unveiledState)
	return unveiledState, nil

}

func (s *SessionStateStorage) StateVeiledAt(ctx context.Context, req *http.Request) (time.Time, error) {
	log := log.FromContext(ctx)
	state := req.URL.Query().Get("state")
	if state == "" {
		log.Error(noStateError, "Request has no state parameter")
		return time.Time{}, noStateError
	}
	createdAt := s.sessionManager.GetInt64(ctx, state+"-createdAt")
	if createdAt == 0 {
		log.Error(noStateError, "Request has invalid creation time")
		return time.Time{}, noStateError
	}
	return time.Unix(createdAt, 0), nil
}

func randStringBytes(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		if err != nil {
			return "", randomStringGenerationError

		}
		b[i] = letterBytes[n.Uint64()]
	}
	return string(b), nil
}

func NewStateStorage(sessionManager *scs.SessionManager) StateStorage {
	return &SessionStateStorage{
		sessionManager: sessionManager,
	}
}
