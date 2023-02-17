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
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var mutex sync.Mutex

// CustomValidationOptions
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

// ValidateStruct validates struct on the preconfigured validator instance
func ValidateStruct(s interface{}) error {
	err := getInstance().Struct(s)
	if err != nil {
		return fmt.Errorf("struct validation failed: %w", err)
	}
	return nil
}

// SetupCustomValidations creates new validator instance and configures it with requested validations
func SetupCustomValidations(options CustomValidationOptions) error {
	var err error
	mutex.Lock()
	defer mutex.Unlock()
	validatorInstance = validator.New() //if we change validation rules, we must re-create validator instance
	if options.AllowInsecureURLs {
		err = getInstance().RegisterValidation("https_only", alwaysTrue)
	} else {
		err = getInstance().RegisterValidation("https_only", isHttpsUrl)
	}
	if err != nil {
		return fmt.Errorf("failed to register custom validation %w", err)
	}
	return nil
}

func isHttpsUrl(fl validator.FieldLevel) bool {
	return strings.HasPrefix(fl.Field().String(), "https://")
}

func alwaysTrue(_ validator.FieldLevel) bool {
	return true
}
