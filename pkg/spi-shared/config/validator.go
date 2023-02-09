package config

import "strings"
import "github.com/go-playground/validator/v10"

func IsHttpsUrl(fl validator.FieldLevel) bool {
	return strings.HasPrefix(fl.Field().String(), "https://")
}
