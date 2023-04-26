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

package bindings

import "errors"

type ErrorReason string

const (
	ErrorReasonNone ErrorReason = ""

	// XXX: note that this used to be used as:
	// - api.SPIAccessTokenBindingErrorReasonTokenSync originally in secretHandler.Sync
	ErrorReasonSecretUpdate ErrorReason = "SecretUpdate"
	// XXX: note that this used to be used as:
	// - api.SPIAccessTokenBindingErrorReasonServiceAccountUnavailable in ensureReferencedServiceAccount -> serviceAccountHandler.Sync
	ErrorReasonServiceAccountUnavailable ErrorReason = "ServiceAccountUnavailable"
	// XXX: note that this used to be used as:
	// - api.SPIAccessTokenBindingErrorReasonServiceAccountUpdate in ensureReferencedServiceAccount -> serviceAccountHandler.Sync
	// - api.SPIAccessTokenBindingErrorReasonTokenSync in ensureReferencedServiceAccount -> serviceAccountHandler.Sync
	ErrorReasonServiceAccountUpdate ErrorReason = "ServiceAccountUpdate"
)

var (
	SecretDataNotFoundError = errors.New("data not found")
)
