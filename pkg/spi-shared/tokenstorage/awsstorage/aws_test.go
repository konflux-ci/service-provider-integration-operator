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

package awsstorage

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"testing"

// 	"github.com/aws/aws-sdk-go-v2/config"
// 	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// )

// //
// //func TestNewSecretManagerTokenStorage(t *testing.T) {
// //	type args struct {
// //		config aws.Config
// //	}
// //	tests := []struct {
// //		name    string
// //		args    args
// //		want    TokenStorage
// //		wantErr assert.ErrorAssertionFunc
// //	}{
// //		// TODO: Add test cases.
// //	}
// //	for _, tt := range tests {
// //		t.Run(tt.name, func(t *testing.T) {
// //			got, err := NewSecretManagerTokenStorage(tt.args.config)
// //			if !tt.wantErr(t, err, fmt.Sprintf("NewSecretManagerTokenStorage(%v)", tt.args.config)) {
// //				return
// //			}
// //			assert.Equalf(t, tt.want, got, "NewSecretManagerTokenStorage(%v)", tt.args.config)
// //		})
// //	}
// //}
// //
// //func Test_secretManagerTokenStorage_Delete(t *testing.T) {
// //	type fields struct {
// //		client *secretsmanager.Client
// //	}
// //	type args struct {
// //		ctx   context.Context
// //		owner *api.SPIAccessToken
// //	}
// //	tests := []struct {
// //		name    string
// //		fields  fields
// //		args    args
// //		wantErr assert.ErrorAssertionFunc
// //	}{
// //		// TODO: Add test cases.
// //	}
// //	for _, tt := range tests {
// //		t.Run(tt.name, func(t *testing.T) {
// //			s := &secretManagerTokenStorage{
// //				client: tt.fields.client,
// //			}
// //			tt.wantErr(t, s.Delete(tt.args.ctx, tt.args.owner), fmt.Sprintf("Delete(%v, %v)", tt.args.ctx, tt.args.owner))
// //		})
// //	}
// //}
// //
// //func Test_secretManagerTokenStorage_Get(t *testing.T) {
// //	type fields struct {
// //		client *secretsmanager.Client
// //	}
// //	type args struct {
// //		ctx   context.Context
// //		owner *api.SPIAccessToken
// //	}
// //	tests := []struct {
// //		name    string
// //		fields  fields
// //		args    args
// //		want    *api.Token
// //		wantErr assert.ErrorAssertionFunc
// //	}{
// //		// TODO: Add test cases.
// //	}
// //	for _, tt := range tests {
// //		t.Run(tt.name, func(t *testing.T) {
// //			s := &secretManagerTokenStorage{
// //				client: tt.fields.client,
// //			}
// //			got, err := s.Get(tt.args.ctx, tt.args.owner)
// //			if !tt.wantErr(t, err, fmt.Sprintf("Get(%v, %v)", tt.args.ctx, tt.args.owner)) {
// //				return
// //			}
// //			assert.Equalf(t, tt.want, got, "Get(%v, %v)", tt.args.ctx, tt.args.owner)
// //		})
// //	}
// //}
// //
// //func Test_secretManagerTokenStorage_Initialize(t *testing.T) {
// //	type fields struct {
// //		client *secretsmanager.Client
// //	}
// //	type args struct {
// //		ctx context.Context
// //	}
// //	tests := []struct {
// //		name    string
// //		fields  fields
// //		args    args
// //		wantErr assert.ErrorAssertionFunc
// //	}{
// //		// TODO: Add test cases.
// //	}
// //	for _, tt := range tests {
// //		t.Run(tt.name, func(t *testing.T) {
// //			s := &secretManagerTokenStorage{
// //				client: tt.fields.client,
// //			}
// //			tt.wantErr(t, s.Initialize(tt.args.ctx), fmt.Sprintf("Initialize(%v)", tt.args.ctx))
// //		})
// //	}
// //}

// func Test_secretManagerTokenStorage_Store(t *testing.T) {
// 	// Using the SDK's default configuration, loading additional config
// 	// and credentials values from the environment variables, shared
// 	// credentials, and shared configuration files
// 	//cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
// 	// initialize a logger
// 	//var loggerBuf bytes.Buffer
// 	//logger := logging.NewStandardLogger(&loggerBuf)
// 	//defer loggerBuf.Reset()
// 	//cfg, err := config.LoadDefaultConfig(context.TODO(),
// 	//	config.WithSharedCredentialsFiles(
// 	//		[]string{"/Users/skabashn/dev/src/prod/aws/user_credentials"},
// 	//	),
// 	//	config.WithLogConfigurationWarnings(true),
// 	//	config.WithLogger(logger),
// 	//	config.WithRegion("us-east-1"),
// 	//	config.WithSharedConfigFiles([]string{}),
// 	//	//config.WithSharedConfigFiles(
// 	//	//	[]string{"/Users/skabashn/dev/src/prod/aws/user_profile"},
// 	//	//),
// 	//)
// 	cfg, err := config.LoadDefaultConfig(context.TODO(),
// 		config.WithSharedCredentialsFiles(
// 			[]string{"/Users/skabashn/dev/src/redhat-appstudio/service-provider-integration-operator/.tmp/credentials"},
// 		),
// 		config.WithSharedConfigFiles(
// 			[]string{"/Users/skabashn/dev/src/redhat-appstudio/service-provider-integration-operator/.tmp/config"},
// 		),
// 	)

// 	if err != nil {
// 		log.Fatalf("unable to load SDK config, %v", err)
// 	}
// 	fmt.Println(cfg.Region)
// 	creds, err := cfg.Credentials.Retrieve(context.Background())
// 	if err != nil {
// 		t.Fatalf("expected no error, but received %v ", err)
// 	}
// 	//crd, err := cfg.Credentials
// 	//crd, err := cfg.Credentials
// 	//if err != nil {
// 	//	log.Fatalf("unable to load SDK config, %v", err)
// 	//}
// 	fmt.Println(creds.SecretAccessKey)
// 	fmt.Println(creds.AccessKeyID)
// 	fmt.Println(cfg.Region)

// 	tokenStorage, err := NewSecretManagerTokenStorage(cfg)
// 	if err != nil {
// 		t.Fatalf("unable to load SDK config, %v", err)
// 	}

// 	// create the token (and let its webhook and controller finish the setup)
// 	accessToken := &api.SPIAccessToken{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "generated-spi-access-token-3",
// 			Namespace: "jdoe",
// 		},
// 	}

// 	origToken := &api.Token{
// 		AccessToken:  "access",
// 		TokenType:    "fake",
// 		RefreshToken: "refresh",
// 		Expiry:       23423,
// 	}

// 	err = tokenStorage.Store(context.TODO(), accessToken, origToken)
// 	if err != nil {
// 		t.Fatalf("unable to load SDK config, %v", err)
// 	}
// 	tt, err := tokenStorage.Get(context.TODO(), accessToken)
// 	if err != nil {
// 		t.Fatalf("unable to load SDK config, %v", err)
// 	}
// 	fmt.Println(tt)
// 	err = tokenStorage.Delete(context.TODO(), accessToken)
// 	if err != nil {
// 		t.Fatalf("unable to load SDK config, %v", err)
// 	}
// 	//type fields struct {
// 	//	client *secretsmanager.Client
// 	//}
// 	//type args struct {
// 	//	ctx   context.Context
// 	//	owner *api.SPIAccessToken
// 	//	token *api.Token
// 	//}
// 	//tests := []struct {
// 	//	name    string
// 	//	fields  fields
// 	//	args    args
// 	//	wantErr assert.ErrorAssertionFunc
// 	//}{
// 	//	// TODO: Add test cases.
// 	//}
// 	//for _, tt := range tests {
// 	//	t.Run(tt.name, func(t *testing.T) {
// 	//		s := &secretManagerTokenStorage{
// 	//			client: tt.fields.client,
// 	//		}
// 	//		tt.wantErr(t, s.Store(tt.args.ctx, tt.args.owner, tt.args.token), fmt.Sprintf("Store(%v, %v, %v)", tt.args.ctx, tt.args.owner, tt.args.token))
// 	//	})

// }
