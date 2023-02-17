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
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	Foo string `validate:"omitempty,https_only"`
}

func TestValidationsOff(t *testing.T) {
	err := SetupCustomValidations(CustomValidationOptions{AllowInsecureURLs: true})
	assert.NoError(t, err)
	err = ValidateStruct(&TestStruct{Foo: "bar"})
	assert.NoError(t, err)
}
func TestValidationsOn(t *testing.T) {
	err := SetupCustomValidations(CustomValidationOptions{AllowInsecureURLs: false})
	assert.NoError(t, err)
	err = ValidateStruct(&TestStruct{Foo: "bar"})
	assert.Error(t, err)
}

func TestValidationsOnButCorrectPath(t *testing.T) {
	err := SetupCustomValidations(CustomValidationOptions{AllowInsecureURLs: false})
	assert.NoError(t, err)
	err = ValidateStruct(&TestStruct{Foo: "https://foo.bar"})
	assert.NoError(t, err)
}
