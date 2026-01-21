package http

import (
	"fmt"
	"reflect"
)

// fieldUnmarshaler is an interface for unmarshaling header values into struct fields
type fieldUnmarshaler interface {
	// canUnmarshal returns true if this unmarshaler can handle the given field kind
	canUnmarshal(kind reflect.Kind, typ reflect.Type) bool
	// unmarshal unmarshals the header values into the field
	unmarshal(values []string, field reflect.Value) error
}

// stringUnmarshaler handles string fields
type stringUnmarshaler struct{}

func (u stringUnmarshaler) canUnmarshal(kind reflect.Kind, typ reflect.Type) bool {
	return kind == reflect.String
}

func (u stringUnmarshaler) unmarshal(values []string, field reflect.Value) error {
	field.SetString(values[0])
	return nil
}

// stringSliceUnmarshaler handles []string fields
type stringSliceUnmarshaler struct{}

func (u stringSliceUnmarshaler) canUnmarshal(kind reflect.Kind, typ reflect.Type) bool {
	return kind == reflect.Slice && typ.Elem().Kind() == reflect.String
}

func (u stringSliceUnmarshaler) unmarshal(values []string, field reflect.Value) error {
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, v := range values {
		slice.Index(i).SetString(v)
	}
	field.Set(slice)
	return nil
}

// intUnmarshaler handles int, int8, int16, int32, int64 fields
type intUnmarshaler struct{}

func (u intUnmarshaler) canUnmarshal(kind reflect.Kind, typ reflect.Type) bool {
	return kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 ||
		kind == reflect.Int32 || kind == reflect.Int64
}

func (u intUnmarshaler) unmarshal(values []string, field reflect.Value) error {
	var n int64
	_, err := fmt.Sscanf(values[0], "%d", &n)
	if err != nil {
		return fmt.Errorf("failed to parse int: %w", err)
	}
	field.SetInt(n)
	return nil
}

// uintUnmarshaler handles uint, uint8, uint16, uint32, uint64 fields
type uintUnmarshaler struct{}

func (u uintUnmarshaler) canUnmarshal(kind reflect.Kind, typ reflect.Type) bool {
	return kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 ||
		kind == reflect.Uint32 || kind == reflect.Uint64
}

func (u uintUnmarshaler) unmarshal(values []string, field reflect.Value) error {
	var n uint64
	_, err := fmt.Sscanf(values[0], "%d", &n)
	if err != nil {
		return fmt.Errorf("failed to parse uint: %w", err)
	}
	field.SetUint(n)
	return nil
}

// boolUnmarshaler handles bool fields
type boolUnmarshaler struct{}

func (u boolUnmarshaler) canUnmarshal(kind reflect.Kind, typ reflect.Type) bool {
	return kind == reflect.Bool
}

func (u boolUnmarshaler) unmarshal(values []string, field reflect.Value) error {
	var b bool
	if values[0] == "true" {
		b = true
	} else if values[0] == "false" {
		b = false
	} else {
		return fmt.Errorf("invalid bool value: %s", values[0])
	}
	field.SetBool(b)
	return nil
}

// unsupportedSliceUnmarshaler handles unsupported slice types
type unsupportedSliceUnmarshaler struct{}

func (u unsupportedSliceUnmarshaler) canUnmarshal(kind reflect.Kind, typ reflect.Type) bool {
	return kind == reflect.Slice && typ.Elem().Kind() != reflect.String
}

func (u unsupportedSliceUnmarshaler) unmarshal(values []string, field reflect.Value) error {
	return fmt.Errorf("unsupported slice type: []%s", field.Type().Elem().Kind())
}

// unmarshalerRegistry holds all registered unmarshalers
var unmarshalerRegistry = []fieldUnmarshaler{
	stringUnmarshaler{},
	stringSliceUnmarshaler{},
	intUnmarshaler{},
	uintUnmarshaler{},
	boolUnmarshaler{},
	unsupportedSliceUnmarshaler{},
}

// findUnmarshaler returns the appropriate unmarshaler for the given field
func findUnmarshaler(field reflect.Value) fieldUnmarshaler {
	kind := field.Kind()
	typ := field.Type()

	for _, u := range unmarshalerRegistry {
		if u.canUnmarshal(kind, typ) {
			return u
		}
	}

	return nil
}
