package validators

import "strings"
import "github.com/go-playground/validator/v10"

func IsHttpsUrl(fl validator.FieldLevel) bool {
	return strings.HasPrefix(fl.Field().String(), "https://")
}

func AlwaysTrue(_ validator.FieldLevel) bool {
	return true
}
