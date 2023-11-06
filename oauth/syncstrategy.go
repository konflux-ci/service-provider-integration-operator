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
)

// tokenDataSyncStrategy determines how the OAuth token data will be synced/saved.
type tokenDataSyncStrategy interface {
	// checkIdentityHasAccess verifies that the user who is going through OAuth flow has the appropriate k8s permissions
	// to save the token data.
	checkIdentityHasAccess(ctx context.Context, namespace string) (bool, error)
	// syncTokenData syncs the token data from exchange into the tokenDataSyncStrategy's underlying storage.
	syncTokenData(ctx context.Context, exchange *exchangeResult) error
}
