package http

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// HeaderMarshalOptions configures how structs are marshaled to and from http.Header
type HeaderMarshalOptions struct {
	// Prefix is prepended to all header names (e.g., "X" results in "X-Field-Name")
	Prefix string
	// IncludeStructName includes the struct type name in the header (e.g., "X-Example-Field-Name")
	IncludeStructName bool
}

// DefaultHeaderMarshalOptions returns default options with no prefix and no struct name
func DefaultHeaderMarshalOptions() HeaderMarshalOptions {
	return HeaderMarshalOptions{
		Prefix:            "",
		IncludeStructName: false,
	}
}

// MarshalHeader encodes a struct into an http.Header using the provided options.
//
// RFC 9110 Compliance:
// For slice fields ([]string), each element is added as a separate header occurrence
// using http.Header.Add(). This is compliant with RFC 9110 Section 5.5, which allows
// multiple header field lines with the same name. Values containing commas, quotes,
// or other special characters are preserved as-is in each occurrence.
//
// Example:
//
//	type Example struct {
//	    Values []string
//	}
//	ex := Example{Values: []string{"val1", "val2,with,comma"}}
//	// Produces:
//	// X-Values: val1
//	// X-Values: val2,with,comma
func MarshalHeader(v interface{}, opts HeaderMarshalOptions) (http.Header, error) {
	header := http.Header{}

	if v == nil {
		return header, nil
	}

	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	// Dereference pointer if necessary
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return header, nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	// Only structs can be marshaled
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("MarshalHeader: expected struct, got %s", val.Kind())
	}

	structName := typ.Name()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get the header name from struct tag or field name
		headerName := getHeaderName(fieldType, structName, opts)

		// Marshal the field value
		if err := marshalField(header, headerName, field); err != nil {
			return nil, fmt.Errorf("MarshalHeader: field %s: %w", fieldType.Name, err)
		}
	}

	return header, nil
}

// UnmarshalHeader decodes an http.Header into a struct using the provided options.
//
// RFC 9110 Compliance:
// Multiple header field occurrences with the same name are unmarshaled into slice
// fields ([]string) using http.Header.Values(). This retrieves all values for the
// header in the order they were added, preserving the semantics defined in RFC 9110
// Section 5.5. Each value is kept as-is without parsing commas or other delimiters.
//
// Example:
//
//	// Given headers:
//	// X-Values: val1
//	// X-Values: val2,with,comma
//	type Example struct {
//	    Values []string
//	}
//	var ex Example
//	UnmarshalHeader(headers, &ex, opts)
//	// Results in: ex.Values = []string{"val1", "val2,with,comma"}
func UnmarshalHeader(header http.Header, v interface{}, opts HeaderMarshalOptions) error {
	if v == nil {
		return fmt.Errorf("UnmarshalHeader: nil destination")
	}

	val := reflect.ValueOf(v)

	// Must be a pointer to a struct
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("UnmarshalHeader: expected pointer to struct, got %s", val.Kind())
	}

	if val.IsNil() {
		return fmt.Errorf("UnmarshalHeader: nil pointer")
	}

	val = val.Elem()
	typ := val.Type()

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("UnmarshalHeader: expected pointer to struct, got pointer to %s", val.Kind())
	}

	structName := typ.Name()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get the header name from struct tag or field name
		headerName := getHeaderName(fieldType, structName, opts)

		// Unmarshal the field value
		if err := unmarshalField(header, headerName, field); err != nil {
			return fmt.Errorf("UnmarshalHeader: field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// getHeaderName constructs the header name from the field type and options
func getHeaderName(field reflect.StructField, structName string, opts HeaderMarshalOptions) string {
	// Check for header tag
	if tag := field.Tag.Get("header"); tag != "" {
		// If tag is "-", skip this field
		if tag == "-" {
			return ""
		}
		// Use the tag value
		return buildHeaderName(tag, structName, opts)
	}

	// Convert field name to kebab-case
	fieldName := toKebabCase(field.Name)
	return buildHeaderName(fieldName, structName, opts)
}

// buildHeaderName constructs the full header name with prefix and struct name
func buildHeaderName(fieldName string, structName string, opts HeaderMarshalOptions) string {
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

	var result strings.Builder

	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Insert hyphen before uppercase letters (except at start)
			result.WriteRune('-')
		}
		// Convert to lowercase
		if r >= 'A' && r <= 'Z' {
			result.WriteRune(r + ('a' - 'A'))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// marshalField marshals a single field value to the header
func marshalField(header http.Header, headerName string, field reflect.Value) error {
	if headerName == "" {
		return nil // Skip fields with empty header names
	}

	switch field.Kind() {
	case reflect.String:
		return marshalStringField(header, headerName, field)
	case reflect.Slice:
		return marshalSliceField(header, headerName, field)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return marshalIntField(header, headerName, field)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return marshalUintField(header, headerName, field)
	case reflect.Bool:
		return marshalBoolField(header, headerName, field)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}

// marshalStringField marshals a string field to the header
func marshalStringField(header http.Header, headerName string, field reflect.Value) error {
	value := field.String()
	if value != "" {
		header.Set(headerName, value)
	}
	return nil
}

// marshalSliceField marshals a slice field to the header
func marshalSliceField(header http.Header, headerName string, field reflect.Value) error {
	if field.Type().Elem().Kind() != reflect.String {
		return fmt.Errorf("unsupported slice type: []%s", field.Type().Elem().Kind())
	}

	// Handle []string
	for i := 0; i < field.Len(); i++ {
		value := field.Index(i).String()
		if value != "" {
			header.Add(headerName, value)
		}
	}
	return nil
}

// marshalIntField marshals an integer field to the header
func marshalIntField(header http.Header, headerName string, field reflect.Value) error {
	header.Set(headerName, fmt.Sprintf("%d", field.Int()))
	return nil
}

// marshalUintField marshals an unsigned integer field to the header
func marshalUintField(header http.Header, headerName string, field reflect.Value) error {
	header.Set(headerName, fmt.Sprintf("%d", field.Uint()))
	return nil
}

// marshalBoolField marshals a boolean field to the header
func marshalBoolField(header http.Header, headerName string, field reflect.Value) error {
	header.Set(headerName, fmt.Sprintf("%t", field.Bool()))
	return nil
}

// unmarshalField unmarshals a header value into a field
func unmarshalField(header http.Header, headerName string, field reflect.Value) error {
	if headerName == "" {
		return nil // Skip fields with empty header names
	}

	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	values := header.Values(headerName)
	if len(values) == 0 {
		return nil // No value in header, leave field as zero value
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
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}

// unmarshalStringField unmarshals a string field from header values
func unmarshalStringField(field reflect.Value, values []string) error {
	field.SetString(values[0])
	return nil
}

// unmarshalSliceField unmarshals a slice field from header values
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

// unmarshalIntField unmarshals an integer field from header values
func unmarshalIntField(field reflect.Value, values []string) error {
	var n int64
	_, err := fmt.Sscanf(values[0], "%d", &n)
	if err != nil {
		return fmt.Errorf("failed to parse int: %w", err)
	}
	field.SetInt(n)
	return nil
}

// unmarshalUintField unmarshals an unsigned integer field from header values
func unmarshalUintField(field reflect.Value, values []string) error {
	var n uint64
	_, err := fmt.Sscanf(values[0], "%d", &n)
	if err != nil {
		return fmt.Errorf("failed to parse uint: %w", err)
	}
	field.SetUint(n)
	return nil
}

// unmarshalBoolField unmarshals a boolean field from header values
func unmarshalBoolField(field reflect.Value, values []string) error {
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
