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

package config

import (
	"fmt"
	"net/url"
)

func GetBaseUrl(url *url.URL) string {
	if url == nil || url.Scheme == "" || url.Host == "" {
		return ""
	}
	return url.Scheme + "://" + url.Host
}

// GetHostWithScheme is a helper function to extract the scheme and host portion of the provided url.
func GetHostWithScheme(repoUrl string) (string, error) {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s as URL: %w", repoUrl, err)
	}
	return GetBaseUrl(u), nil
}
