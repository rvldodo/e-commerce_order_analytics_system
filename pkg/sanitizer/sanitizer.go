package sanitizer

import (
	"github.com/microcosm-cc/bluemonday"
)

type Sanitize struct {
	sanitizer *bluemonday.Policy
}

func New() *Sanitize {
	sanitizer := Sanitize{
		sanitizer: bluemonday.StrictPolicy(),
	}

	return &sanitizer
}

func (s *Sanitize) Sanitize(field string) string {
	return s.sanitizer.Sanitize(field)
}
