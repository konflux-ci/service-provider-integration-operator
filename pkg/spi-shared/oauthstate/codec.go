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

package oauthstate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ParseInto tries to parse the provided state into the dest object. Note that no validation is done on the parsed
// object.
func ParseInto(state string, dest interface{}) error {
	data, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return fmt.Errorf("failed to base64-decode the state: %w", err)
	}

	err = json.Unmarshal(data, dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal the state JSON: %w", err)
	}
	return nil
}

// Encode encodes the provided state as a URL-safe string.
func Encode(state interface{}) (token string, err error) {
	var data []byte

	data, err = json.Marshal(state)
	if err != nil {
		return
	}

	token = base64.RawURLEncoding.EncodeToString(data)

	return
}
