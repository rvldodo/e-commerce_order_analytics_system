package validation

import (
	"e-commerce_order_analytics_system/pkg/sanitizer"
	"e-commerce_order_analytics_system/pkg/validator"
	"strings"
)

type Validator interface {
	ValidateStruct(lang string, obj interface{}) string
}

type Sanitizer interface {
	Sanitize(field string) string
}

type Validation struct {
	validator Validator
	sanitizer Sanitizer
}

var validation *Validation

func New() *Validation {
	validation = &Validation{}
	validation.validator = validator.New("en", "id")
	validation.sanitizer = sanitizer.New()

	return validation
}

func ValidateStruct(lang string, obj interface{}) string {
	return validation.validator.ValidateStruct(lang, obj)
}

func Sanitize(field string) string {
	return strings.TrimSpace(validation.sanitizer.Sanitize(field))
}
