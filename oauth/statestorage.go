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

type StateStorage struct {
	sessionManager *scs.SessionManager
}
type stateBucket struct {
	realState string
	createAt  time.Time
}

var (
	noStateError                = errors.New("request has no `state` parameter")
	randomStringGenerationError = errors.New("not able to generate new random string")
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
)

func (s StateStorage) VeilRealState(req *http.Request) (string, error) {

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
	s.sessionManager.Put(req.Context(), newState, stateBucket{realState: state, createAt: time.Now()})
	return newState, nil
}

func (s StateStorage) UnveilState(ctx context.Context, req *http.Request) (string, error) {
	log := log.FromContext(ctx)
	state := req.URL.Query().Get("state")
	if state == "" {
		log.Error(noStateError, "Request has no state parameter")
		return "", noStateError
	}
	stBucket := s.sessionManager.Get(ctx, state).(stateBucket)
	log.V(logs.DebugLevel).Info("State unveiled", "veil", state, "unveiledState", stBucket.realState)
	return stBucket.realState, nil
}

func (s StateStorage) StateVeiledAt(ctx context.Context, req *http.Request) (time.Time, error) {
	log := log.FromContext(ctx)
	state := req.URL.Query().Get("state")
	if state == "" {
		log.Error(noStateError, "Request has no state parameter")
		return time.Time{}, noStateError
	}
	stBucket := s.sessionManager.Get(ctx, state).(stateBucket)
	return stBucket.createAt, nil
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

func NewStateStorage(sessionManager *scs.SessionManager) *StateStorage {
	return &StateStorage{
		sessionManager: sessionManager,
	}
}
