package http

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type HTTPMarshalOptions struct {
	// Prefix is prepended to all parameter names (e.g., "X" results in "X-Field-Name")
	Prefix string
	// IncludeStructName includes the struct type name in the parameter (e.g., "X-Example-Field-Name")
	IncludeStructName bool
	// DefaultKebabCase converts fieldSet names to kebab-case by default (e.g., "FieldName" becomes "field-name")
	DefaultKebabCase bool
}

// DefaultHTTPMarshalOptions returns default options with no prefix and no struct name
func DefaultHTTPMarshalOptions() HTTPMarshalOptions {
	return HTTPMarshalOptions{
		Prefix:            "",
		IncludeStructName: false,
		DefaultKebabCase:  false,
	}
}

type fieldSetter interface {
	Set(string, string)
}

type fieldAdder interface {
	Add(string, string)
}

type fieldValues interface {
	Values(string) []string
}

type fieldSet interface {
	fieldSetter
	fieldAdder
	fieldValues
}

type tagDetails struct {
	name      string
	modifiers []string
	skip      bool
}

func (d tagDetails) OmitEmpty() bool {
	return slices.Contains(d.modifiers, "omitempty")
}

func (d tagDetails) Skip() bool {
	return d.skip
}

func getTagDetails(tagName string, field reflect.StructField) tagDetails {
	details := tagDetails{}
	t := field.Tag.Get(tagName)
	if t == "-" {
		details.skip = true
	}

	name, modifiersStr, found := strings.Cut(t, ",")
	if found {
		details.name = strings.TrimSpace(name)
		details.modifiers = strings.Split(modifiersStr, ",")
	} else {
		details.name = strings.TrimSpace(t)
	}
	return details
}

func marshalFields(v interface{}, tagName string, set fieldSet, opts HTTPMarshalOptions) error {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	// Dereference pointer if necessary
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	// Only structs can be marshaled
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", val.Kind())
	}

	structName := typ.Name()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get the field name from struct tag or fieldSet name
		fieldName := getFieldName(tagName, fieldType, structName, opts)

		// Marshal the fieldSet value
		if err := marshalField(set, fieldName, field); err != nil {
			return fmt.Errorf("fieldSet %s: %w", fieldType.Name, err)
		}
	}
	return nil
}

func getFieldName(tagName string, field reflect.StructField, structName string, opts HTTPMarshalOptions) string {
	// Check for query tag
	details := getTagDetails(tagName, field)
	if details.skip {
		return ""
	}
	fieldName := field.Name
	if details.name != "" {
		fieldName = details.name
	} else {
		// Convert fieldSet name to kebab-case if default is enabled
		if opts.DefaultKebabCase {
			fieldName = toKebabCase(fieldName)
		}
	}

	return buildFieldName(fieldName, structName, opts)
}

func buildFieldName(fieldName string, structName string, opts HTTPMarshalOptions) string {
	var parts []string

	if opts.Prefix != "" {
		parts = append(parts, opts.Prefix)
	}

	if opts.IncludeStructName && structName != "" {
		parts = append(parts, toKebabCase(structName))
	}

	parts = append(parts, fieldName)

	return strings.Join(parts, "-")
}

// toKebabCase converts CamelCase to kebab-case
// It inserts a hyphen before each uppercase letter (except the first)
// Examples: "FieldOne" -> "field-one", "UserID" -> "user-i-d", "HTTPServer" -> "h-t-t-p-server"
func toKebabCase(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	var result strings.Builder

	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
				result.WriteRune('-')
			}
		}

		if unicode.IsUpper(r) {
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// marshalField marshals a single field value to the fieldSet based on its type
func marshalField(set fieldSet, fieldName string, field reflect.Value) error {
	if fieldName == "" {
		return nil // Skip fields with empty field names
	}

	switch field.Kind() {
	case reflect.String:
		return marshalStringField(set, fieldName, field)
	case reflect.Slice:
		return marshalSliceField(set, fieldName, field)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return marshalIntField(set, fieldName, field)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return marshalUintField(set, fieldName, field)
	case reflect.Bool:
		return marshalBoolField(set, fieldName, field)
	default:
		return fmt.Errorf("unsupported fieldSet type: %s", field.Kind())
	}
}

// marshalStringField marshals a string field to the fieldSet
func marshalStringField(set fieldSet, fieldName string, field reflect.Value) error {
	value := field.String()
	if value != "" {
		set.Set(fieldName, value)
	}
	return nil
}

// marshalSliceField marshals a slice field to the fieldSet
func marshalSliceField(set fieldSet, fieldName string, field reflect.Value) error {
	if field.Type().Elem().Kind() != reflect.String {
		return fmt.Errorf("unsupported slice type: []%s", field.Type().Elem().Kind())
	}

	// Handle []string
	for i := 0; i < field.Len(); i++ {
		value := field.Index(i).String()
		if value != "" {
			set.Add(fieldName, value)
		}
	}
	return nil
}

// marshalIntField marshals an integer field to the fieldSet
func marshalIntField(set fieldSet, fieldName string, field reflect.Value) error {
	set.Set(fieldName, fmt.Sprintf("%d", field.Int()))
	return nil
}

// marshalUintField marshals an unsigned integer field to the fieldSet
func marshalUintField(set fieldSet, fieldName string, field reflect.Value) error {
	set.Set(fieldName, fmt.Sprintf("%d", field.Uint()))
	return nil
}

// marshalBoolField marshals a boolean field to the fieldSet
func marshalBoolField(set fieldSet, fieldName string, field reflect.Value) error {
	set.Set(fieldName, fmt.Sprintf("%t", field.Bool()))
	return nil
}

// unmarshalFields unmarshals values from a fieldSet into a struct based on struct tags and options
func unmarshalFields(set fieldSet, v interface{}, tagName string, opts HTTPMarshalOptions) error {
	if v == nil {
		return fmt.Errorf("nil destination")
	}

	val := reflect.ValueOf(v)

	// Must be a pointer to a struct
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer to struct, got %s", val.Kind())
	}

	if val.IsNil() {
		return fmt.Errorf("nil pointer")
	}

	val = val.Elem()
	typ := val.Type()

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected pointer to struct, got pointer to %s", val.Kind())
	}

	structName := typ.Name()

	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get the query name from struct tag or fieldSet name
		parameterName := getFieldName(tagName, fieldType, structName, opts)

		// Unmarshal the fieldSet value
		if err := unmarshalField(set, parameterName, f); err != nil {
			return fmt.Errorf("fieldSet %s: %w", fieldType.Name, err)
		}
	}
	return nil
}

// unmarshalField unmarshals a field value into a fieldSet
func unmarshalField(set fieldSet, fieldName string, field reflect.Value) error {
	if fieldName == "" {
		return nil // Skip fields with empty filter names
	}

	if !field.CanSet() {
		return fmt.Errorf("fieldSet is not settable")
	}

	values := set.Values(fieldName)
	if len(values) == 0 {
		return nil // No value in fieldSet, leave field as zero value
	}

	switch field.Kind() {
	case reflect.String:
		return unmarshalStringField(field, values)
	case reflect.Slice:
		return unmarshalSliceField(field, values)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return unmarshalIntField(field, values)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return unmarshalUintField(field, values)
	case reflect.Bool:
		return unmarshalBoolField(field, values)
	default:
		return fmt.Errorf("unsupported fieldSet type: %s", field.Kind())
	}
}

// unmarshalStringField unmarshals a string field from fieldSet values
func unmarshalStringField(field reflect.Value, values []string) error {
	field.SetString(values[0])
	return nil
}

// unmarshalSliceField unmarshals a slice field from fieldSet values
func unmarshalSliceField(field reflect.Value, values []string) error {
	if field.Type().Elem().Kind() != reflect.String {
		return fmt.Errorf("unsupported slice type: []%s", field.Type().Elem().Kind())
	}

	// Handle []string
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, v := range values {
		slice.Index(i).SetString(v)
	}
	field.Set(slice)
	return nil
}

// unmarshalIntField unmarshals an integer field from fieldSet values
func unmarshalIntField(field reflect.Value, values []string) error {
	var n int64
	_, err := fmt.Sscanf(values[0], "%d", &n)
	if err != nil {
		return fmt.Errorf("failed to parse int: %w", err)
	}
	field.SetInt(n)
	return nil
}

// unmarshalUintField unmarshals an unsigned integer field from fieldSet values
func unmarshalUintField(field reflect.Value, values []string) error {
	var n uint64
	_, err := fmt.Sscanf(values[0], "%d", &n)
	if err != nil {
		return fmt.Errorf("failed to parse uint: %w", err)
	}
	field.SetUint(n)
	return nil
}

// unmarshalBoolField unmarshals a boolean field from fieldSet values
func unmarshalBoolField(field reflect.Value, values []string) error {
	b, err := strconv.ParseBool(values[0])
	if err != nil {
		return fmt.Errorf("failed to parse bool: %w", err)
	}

	field.SetBool(b)
	return nil
}
