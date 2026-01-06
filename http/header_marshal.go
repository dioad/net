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

// MarshalHeader encodes a struct into an http.Header using the provided options
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

// UnmarshalHeader decodes an http.Header into a struct using the provided options
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
		value := field.String()
		if value != "" {
			header.Set(headerName, value)
		}
	
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			// Handle []string
			for i := 0; i < field.Len(); i++ {
				value := field.Index(i).String()
				if value != "" {
					header.Add(headerName, value)
				}
			}
		} else {
			return fmt.Errorf("unsupported slice type: []%s", field.Type().Elem().Kind())
		}
	
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		header.Set(headerName, fmt.Sprintf("%d", field.Int()))
	
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		header.Set(headerName, fmt.Sprintf("%d", field.Uint()))
	
	case reflect.Bool:
		header.Set(headerName, fmt.Sprintf("%t", field.Bool()))
	
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	
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
		field.SetString(values[0])
	
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			// Handle []string
			slice := reflect.MakeSlice(field.Type(), len(values), len(values))
			for i, v := range values {
				slice.Index(i).SetString(v)
			}
			field.Set(slice)
		} else {
			return fmt.Errorf("unsupported slice type: []%s", field.Type().Elem().Kind())
		}
	
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n int64
		_, err := fmt.Sscanf(values[0], "%d", &n)
		if err != nil {
			return fmt.Errorf("failed to parse int: %w", err)
		}
		field.SetInt(n)
	
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var n uint64
		_, err := fmt.Sscanf(values[0], "%d", &n)
		if err != nil {
			return fmt.Errorf("failed to parse uint: %w", err)
		}
		field.SetUint(n)
	
	case reflect.Bool:
		var b bool
		if values[0] == "true" {
			b = true
		} else if values[0] == "false" {
			b = false
		} else {
			return fmt.Errorf("invalid bool value: %s", values[0])
		}
		field.SetBool(b)
	
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	
	return nil
}
