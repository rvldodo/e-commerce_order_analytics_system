package validator

import (
	"errors"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/id"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	entrans "github.com/go-playground/validator/v10/translations/en"
	idtrans "github.com/go-playground/validator/v10/translations/id"
)

type Validators struct {
	registeredValidators map[string]*SingleValidator
}

type SingleValidator struct {
	validator  *validator.Validate
	translator ut.Translator
}

func New(languages ...string) *Validators {
	validators := make(map[string]*SingleValidator, 0)
	for _, lang := range languages {
		validators[lang] = newValidator(lang)
	}

	return &Validators{
		registeredValidators: validators,
	}
}

func newValidator(lang string) *SingleValidator {
	var (
		defaultLang locales.Translator
		activeLang  locales.Translator
	)

	defaultLang = en.New()
	if lang == "" {
		lang = "en"
	}

	if lang == "en" {
		activeLang = en.New()
	}

	if lang == "id" {
		activeLang = id.New()
	}
	uni := ut.New(defaultLang, activeLang)
	trans, _ := uni.GetTranslator(lang)

	v := validator.New()
	if lang == "en" {
		entrans.RegisterDefaultTranslations(v, trans)
	}

	if lang == "id" {
		idtrans.RegisterDefaultTranslations(v, trans)
	}

	return &SingleValidator{
		validator:  v,
		translator: trans,
	}
}

func (v *Validators) ValidateStruct(lang string, obj interface{}) string {
	valMessages := ""
	if _, exist := v.registeredValidators[lang]; !exist {
		lang = "en"
	}
	if _, exist := v.registeredValidators[lang]; !exist {
		return "validation language is not supported"
	}
	singleValidator := v.registeredValidators[lang]
	err := singleValidator.validator.Struct(obj)
	if err != nil {
		var errs validator.ValidationErrors
		if !errors.As(err, &errs) {
			return err.Error()
		}

		for i, e := range errs {
			limiter := ""
			if i != (len(errs) - 1) {
				limiter = " | "
			}
			valMessages += e.Translate(singleValidator.translator) + limiter
		}
	}

	return valMessages
}
