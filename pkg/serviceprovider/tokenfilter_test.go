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

package serviceprovider

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"testing"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

var filterTrue = TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
	return true, nil
})
var filterFalse = TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
	return false, nil
})

var filterError = TokenFilterFunc(func(ctx context.Context, binding Matchable, token *api.SPIAccessToken) (bool, error) {
	return false, fmt.Errorf("some error")
})

var conditionTrue = func() bool {
	return true
}
var conditionFalse = func() bool {
	return false
}

func TestFallBackTokenFilter_Matches(t *testing.T) {
	type fields struct {
		Condition       func() bool
		MainTokenFilter TokenFilter
		FallBackFilter  TokenFilter
	}
	type args struct {
		ctx       context.Context
		matchable Matchable
		token     *api.SPIAccessToken
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{"test condition true with MatchAllTokenFilter", fields{
			Condition:       conditionTrue,
			MainTokenFilter: MatchAllTokenFilter,
			FallBackFilter:  filterError,
		},
			args{context.TODO(), &api.SPIAccessTokenBinding{}, &api.SPIAccessToken{}},
			true,
			assert.NoError,
		},
		{"test condition true and error", fields{
			Condition:       conditionTrue,
			MainTokenFilter: filterError,
			FallBackFilter:  filterTrue,
		},
			args{context.TODO(), &api.SPIAccessTokenBinding{}, &api.SPIAccessToken{}},
			false,
			assert.Error,
		},
		{"test condition false", fields{
			Condition:       conditionFalse,
			MainTokenFilter: filterTrue,
			FallBackFilter:  filterFalse,
		},
			args{context.TODO(), &api.SPIAccessTokenBinding{}, &api.SPIAccessToken{}},
			false,
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FallBackTokenFilter{
				Condition:       tt.fields.Condition,
				MainTokenFilter: tt.fields.MainTokenFilter,
				FallBackFilter:  tt.fields.FallBackFilter,
			}
			got, err := f.Matches(tt.args.ctx, tt.args.matchable, tt.args.token)
			if !tt.wantErr(t, err, fmt.Sprintf("Matches(%v, %v, %v)", tt.args.ctx, tt.args.matchable, tt.args.token)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Matches(%v, %v, %v)", tt.args.ctx, tt.args.matchable, tt.args.token)
		})
	}
}

func TestFallBackTokenFilter_getActiveFilter(t *testing.T) {
	type fields struct {
		Condition       func() bool
		MainTokenFilter TokenFilter
		FallBackFilter  TokenFilter
	}
	tests := []struct {
		name   string
		fields fields
		want   TokenFilter
	}{
		{"test condition true", fields{conditionTrue, filterTrue, filterError}, filterTrue},
		{"test condition false", fields{conditionFalse, filterTrue, filterError}, filterError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := FallBackTokenFilter{
				Condition:       tt.fields.Condition,
				MainTokenFilter: tt.fields.MainTokenFilter,
				FallBackFilter:  tt.fields.FallBackFilter,
			}
			assert.Equalf(t, runtime.FuncForPC(reflect.ValueOf(tt.want).Pointer()).Name(), runtime.FuncForPC(reflect.ValueOf(f.getActiveFilter()).Pointer()).Name(), "getActiveFilter()")
		})
	}
}

func TestTokenFilterFunc_Matches(t *testing.T) {
	type args struct {
		ctx       context.Context
		matchable Matchable
		token     *api.SPIAccessToken
	}
	tests := []struct {
		name    string
		f       TokenFilterFunc
		args    args
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{"test true filter", filterTrue, args{context.TODO(), &api.SPIAccessTokenBinding{}, &api.SPIAccessToken{}}, true, assert.NoError},
		{"test false filter", filterFalse, args{context.TODO(), &api.SPIAccessTokenBinding{}, &api.SPIAccessToken{}}, false, assert.NoError},
		{"test error filter", filterError, args{context.TODO(), &api.SPIAccessTokenBinding{}, &api.SPIAccessToken{}}, false, assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.Matches(tt.args.ctx, tt.args.matchable, tt.args.token)
			if !tt.wantErr(t, err, fmt.Sprintf("Matches(%v, %v, %v)", tt.args.ctx, tt.args.matchable, tt.args.token)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Matches(%v, %v, %v)", tt.args.ctx, tt.args.matchable, tt.args.token)
		})
	}
}
