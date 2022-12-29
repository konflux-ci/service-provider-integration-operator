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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
)

func TestVeilSPIState(t *testing.T) {
	//given
	req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s&k8s_token=%s", "statestr", "token-234234"), nil)
	res := httptest.NewRecorder()
	sessionManager := scs.New()
	storage := NewStateStorage(sessionManager)

	//when
	sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newStateString, err := storage.VeilRealState(r)
		assert.NoError(t, err)
		assert.True(t, "statestr" != newStateString)
		assert.Equal(t, 32, len(newStateString))
		assert.Equal(t, "statestr", sessionManager.Get(r.Context(), newStateString))

	})).ServeHTTP(res, req)

	//then
	assert.Equal(t, 1, len(res.Result().Cookies()))
	assert.Equal(t, "session", res.Result().Cookies()[0].Name)
	assert.NotEmpty(t, res.Result().Cookies()[0].Value)

}
func Test_FailToVeilIfStateIsEmpty(t *testing.T) {
	//given
	req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s&k8s_token=%s", "", "token-234234"), nil)
	res := httptest.NewRecorder()
	sessionManager := scs.New()
	storage := &SessionStateStorage{sessionManager: sessionManager}

	//when
	sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newStateString, err := storage.VeilRealState(r)
		assert.True(t, errors.Is(err, noStateError))
		assert.Empty(t, newStateString)

	})).ServeHTTP(res, req)

	//then
	assert.Equal(t, 0, len(res.Result().Cookies()))

}

func Test_ShouldUnveilState(t *testing.T) {
	//given
	var oAuthState = "state-2342iijaoidfjoiwe"
	var spiState = "state-spi"
	sessionManager := scs.New()
	sessionManager.Cookie.Persist = false
	res := httptest.NewRecorder()
	sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionManager.Put(r.Context(), oAuthState, spiState)
		sessionManager.Put(r.Context(), oAuthState+"-createdAt", time.Now().Unix())
	})).ServeHTTP(res, httptest.NewRequest("GET", "/", nil))

	req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s", oAuthState), nil)
	req.AddCookie(res.Result().Cookies()[0])
	storage := NewStateStorage(sessionManager)

	//when
	sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalSpiState, err := storage.UnveilState(r.Context(), r)
		time, err2 := storage.StateVeiledAt(r.Context(), r)
		assert.NotNil(t, time)
		assert.Nil(t, err2)
		assert.NoError(t, err)
		assert.Equal(t, spiState, originalSpiState)

	})).ServeHTTP(res, req)

	//then
	assert.Equal(t, 1, len(res.Result().Cookies()))
	assert.Equal(t, "session", res.Result().Cookies()[0].Name)
	assert.NotEmpty(t, res.Result().Cookies()[0].Value)

}

func Test_FailToUnveilStateIfStateIsEmpty(t *testing.T) {
	//given
	req := httptest.NewRequest("GET", fmt.Sprintf("/?state=%s", ""), nil)
	res := httptest.NewRecorder()
	sessionManager := scs.New()
	storage := &SessionStateStorage{sessionManager: sessionManager}

	//when
	sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newStateString, err := storage.UnveilState(r.Context(), r)
		assert.True(t, errors.Is(err, noStateError))
		assert.Empty(t, newStateString)

	})).ServeHTTP(res, req)

	//then
	assert.Equal(t, 0, len(res.Result().Cookies()))

}

type SimpleStateStorage struct {
	state     string
	vailState string
	vailAt    time.Time
}

var _ StateStorage = (*SimpleStateStorage)(nil)

func (n SimpleStateStorage) VeilRealState(req *http.Request) (string, error) {
	return n.vailState, nil
}

func (n SimpleStateStorage) UnveilState(ctx context.Context, req *http.Request) (string, error) {
	return n.state, nil
}

func (n SimpleStateStorage) StateVeiledAt(ctx context.Context, req *http.Request) (time.Time, error) {
	return n.vailAt, nil
}
