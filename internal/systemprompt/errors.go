package systemprompt

import "fmt"

type ValidationError struct {
	Field  string
	Value  string
	Detail string
}

func (e ValidationError) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Detail)
	}
	return fmt.Sprintf("invalid %s %q: %s", e.Field, e.Value, e.Detail)
}

func invalid(field, value, detail string) error {
	return ValidationError{Field: field, Value: value, Detail: detail}
}
