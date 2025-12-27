package json

type M map[string]any
type Error string
type FieldError struct {
	Field string
	Error string
}

func genFieldsField(errors []FieldError) []M {
	var fields []M
	for _, err := range errors {
		field := M{}
		field[err.Field] = err.Error
		fields = append(fields, field)
	}
	return fields
}
