package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
