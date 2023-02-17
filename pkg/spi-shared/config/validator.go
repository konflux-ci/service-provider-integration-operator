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
	"github.com/go-playground/validator/v10"
	"strings"
	"sync"
)

var mutex sync.Mutex

type CustomValidationOptions struct {
	AllowInsecureURLs bool
}

var validatorInstance *validator.Validate

func getInstance() *validator.Validate {
	if validatorInstance == nil {
		mutex.Lock()
		defer mutex.Unlock()
		if validatorInstance == nil {
			validatorInstance = validator.New()
		}
	}
	return validatorInstance
}

func ValidateStruct(s interface{}) error {
	return getInstance().Struct(s)
}

func SetupCustomValidations(options CustomValidationOptions) error {
	var err error
	if options.AllowInsecureURLs {
		reset()
		err = getInstance().RegisterValidation("https_only", alwaysTrue)
	} else {
		reset()
		err = getInstance().RegisterValidation("https_only", isHttpsUrl)
	}
	return err
}

func isHttpsUrl(fl validator.FieldLevel) bool {
	return strings.HasPrefix(fl.Field().String(), "https://")
}

func alwaysTrue(_ validator.FieldLevel) bool {
	return true
}

func reset() {
	if validatorInstance != nil {
		mutex.Lock()
		defer mutex.Unlock()
		validatorInstance = nil
	}
}
