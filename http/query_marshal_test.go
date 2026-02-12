package http

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMarshalQueryWithPrefixAndStructName tests the example from the issue
func TestMarshalQueryWithPrefixAndStructName(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2", "value3"},
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
		DefaultKebabCase:  true,
	}

	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery should not fail")

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "URL parse should not fail")

	// Check fieldSet one
	assert.Equal(t, "value1", values.Get("X-example-fieldSet-one"))

	// Check fieldSet two (multiple values)
	assert.Equalf(t, []string{"value2", "value3"}, values["X-example-fieldSet-two"], "X-example-fieldSet-two mismatch")
}

// TestMarshalQueryWithoutStructName tests encoding without struct name
func TestMarshalQueryWithoutStructName(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
	}

	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)

	// Check fieldSet one (without struct name)
	assert.Equal(t, values.Get("X-FieldOne"), "value1")

	// Check fieldSet two
	assert.Equal(t, values.Get("X-FieldTwo"), "value2")
}

// TestMarshalQueryNoPrefix tests encoding without prefix
func TestMarshalQueryNoPrefix(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	// Check fieldSet one (no prefix, no struct name)
	assert.Equal(t, values.Get("FieldOne"), "value1")

	// Check fieldSet two
	assert.Equal(t, values.Get("FieldTwo"), "value2")
}

func TestUnmarshalQuery(t *testing.T) {
	var example Example

	rawQuery := "FieldOne=value1&FieldTwo=value2&FieldTwo=value3"

	opts := DefaultHTTPMarshalOptions()
	err := UnmarshalQuery(rawQuery, &example, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equal(t, "value1", example.FieldOne)
	assert.Equal(t, []string{"value2", "value3"}, example.FieldTwo)
}

// TestUnmarshalQueryWithPrefixAndStructName tests decoding with prefix and struct name
func TestUnmarshalQueryWithPrefixAndStructName(t *testing.T) {
	values := url.Values{}
	values.Set("X-example-fieldSet-one", "value1")
	values.Add("X-example-fieldSet-two", "value2")
	values.Add("X-example-fieldSet-two", "value3")

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
		DefaultKebabCase:  true,
	}

	var example Example
	err := UnmarshalQuery(values.Encode(), &example, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equal(t, "value1", example.FieldOne)

	assert.Equal(t, []string{"value2", "value3"}, example.FieldTwo)
}

// TestUnmarshalQueryWithoutStructName tests decoding without struct name
func TestUnmarshalQueryWithoutStructName(t *testing.T) {
	values := url.Values{}
	values.Set("X-fieldSet-one", "value1")
	values.Set("X-fieldSet-two", "value2")

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
		DefaultKebabCase:  true,
	}

	var example Example
	err := UnmarshalQuery(values.Encode(), &example, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equal(t, "value1", example.FieldOne)

	assert.Equalf(t, []string{"value2"}, example.FieldTwo, "FieldTwo mismatch")
}

// TestMarshalUnmarshalQueryRoundTrip tests that marshal followed by unmarshal returns the original
func TestMarshalUnmarshalQueryRoundTrip(t *testing.T) {
	original := Example{
		FieldOne: "test-value",
		FieldTwo: []string{"a", "b", "c"},
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	// Marshal
	query, err := MarshalQuery(original, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	// Unmarshal
	var result Example
	err = UnmarshalQuery(query, &result, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	// Compare
	assert.Equalf(t, original, result, "Round trip mismatch")
}

// TestStructWithQueryTags tests struct with custom query tags
func TestStructWithQueryTags(t *testing.T) {
	type CustomStruct struct {
		Field1 string   `query:"custom-name"`
		Field2 []string `query:"another-name"`
		Field3 string   `query:"-"` // Should be ignored
	}

	cs := CustomStruct{
		Field1: "value1",
		Field2: []string{"value2"},
		Field3: "ignored",
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
	}

	query, err := MarshalQuery(cs, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)

	// Check custom names
	assert.Equal(t, "value1", values.Get("X-custom-name"))

	assert.Equal(t, "value2", values.Get("X-another-name"))

	// Field3 should not be present
	assert.NotContains(t, values.Get("X-Field3"), "X-Field3 should not be present (marked with '-' tag)")
}

// TestMarshalQueryEmptyStruct tests marshaling an empty struct
func TestMarshalQueryEmptyStruct(t *testing.T) {
	example := Example{}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	// Empty strings should not create parameters
	assert.Emptyf(t, query, "Expected empty query,")
}

// TestMarshalQueryNilPointer tests marshaling a nil pointer
func TestMarshalQueryNilPointer(t *testing.T) {
	var example *Example

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	assert.Emptyf(t, query, "Expected an empty string")
}

// TestMarshalQueryPointerToStruct tests marshaling a pointer to a struct
func TestMarshalQueryPointerToStruct(t *testing.T) {
	example := &Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)

	assert.Equalf(t, values.Get("FieldOne"), "value1", "FieldOne mismatch")
}

// TestUnmarshalQueryInvalidDestination tests error handling for invalid destinations
func TestUnmarshalQueryInvalidDestination(t *testing.T) {
	values := url.Values{}
	values.Set("fieldSet-one", "value1")

	opts := DefaultHTTPMarshalOptions()

	// Test with non-pointer
	var example Example
	err := UnmarshalQuery(values.Encode(), example, opts)
	assert.Error(t, err, "Expected error when unmarshaling to non-pointer")

	// Test with nil
	err = UnmarshalQuery(values.Encode(), nil, opts)
	assert.Error(t, err, "Expected error when unmarshaling to nil")

	// Test with nil pointer
	var nilPtr *Example
	err = UnmarshalQuery(values.Encode(), nilPtr, opts)
	assert.Error(t, err, "Expected error when unmarshaling to nil")
}

// TestMarshalQueryNonStruct tests error handling for non-struct types
func TestMarshalQueryNonStruct(t *testing.T) {
	notAStruct := "string"

	opts := DefaultHTTPMarshalOptions()

	_, err := MarshalQuery(notAStruct, opts)
	assert.Error(t, err, "Expected error when unmarshaling to non-struct")
}

// TestMultipleQueryTypes tests marshaling different fieldSet types
func TestMultipleQueryTypes(t *testing.T) {
	type MultiTypeStruct struct {
		StringField string
		IntField    int
		UintField   uint
		BoolField   bool
		SliceField  []string
	}

	mt := MultiTypeStruct{
		StringField: "test",
		IntField:    42,
		UintField:   100,
		BoolField:   true,
		SliceField:  []string{"a", "b"},
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(mt, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)
	if err != nil {
		t.Fatalf("ParseQuery failed: %v", err)
	}

	assert.Equalf(t, values.Get("StringField"), mt.StringField, "StringField mismatch")
	assert.Equalf(t, values.Get("IntField"), "42", "IntField mismatch")
	assert.Equalf(t, values.Get("UintField"), "100", "UintField mismatch")
	assert.Equalf(t, values.Get("BoolField"), "true", "BoolField mismatch")

	assert.Equalf(t, values["SliceField"], []string{"a", "b"}, "SliceField mismatch")

	// Test unmarshal
	var result MultiTypeStruct
	err = UnmarshalQuery(query, &result, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equalf(t, result, mt, "Round trip mismatch")
}

// TestUnexportedQueryFields tests that unexported fields are skipped
func TestUnexportedQueryFields(t *testing.T) {
	type StructWithUnexported struct {
		ExportedField   string
		unexportedField string
	}

	s := StructWithUnexported{
		ExportedField:   "visible",
		unexportedField: "hidden",
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(s, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)

	// Only exported fieldSet should be present
	assert.Equalf(t, values.Get("ExportedField"), "visible", "exported fieldSet mismatch")

	// Unexported fieldSet should not be present
	assert.NotContainsf(t, values, "UnexportedFfield", "unexported fieldSet should not be present")
}

// TestEmptyQuerySlice tests handling of empty slices
func TestEmptyQuerySlice(t *testing.T) {
	type StructWithSlice struct {
		EmptySlice []string
		NilSlice   []string
	}

	s := StructWithSlice{
		EmptySlice: []string{},
		NilSlice:   nil,
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(s, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	assert.Emptyf(t, query, "Expected empty query, got %q", query)
}

// TestRFC3986MultipleQueryOccurrences verifies repeated query params are unmarshaled into a slice
func TestRFC3986MultipleQueryOccurrences(t *testing.T) {
	values := url.Values{}
	values.Add("X-example-fieldSet-two", "value1")
	values.Add("X-example-fieldSet-two", "value2")
	values.Add("X-example-fieldSet-two", "value3")

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
		DefaultKebabCase:  true,
	}

	var example Example
	err := UnmarshalQuery(values.Encode(), &example, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equalf(t, []string{"value1", "value2", "value3"}, example.FieldTwo, "Multiple query occurrences not unmarshaled correctly")

	// Verify marshaling creates multiple occurrences
	query, err := MarshalQuery(example, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	parsed, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)

	assert.Equalf(t, []string{"value1", "value2", "value3"}, parsed["X-example-fieldSet-two"], "Multiple values not marshaled correctly")
}

// TestQueryValuesWithCommas tests that values containing commas are preserved
func TestQueryValuesWithCommas(t *testing.T) {
	type CommaStruct struct {
		Values []string
	}

	// Test marshaling values with commas
	cs := CommaStruct{
		Values: []string{"simple", "value,with,comma", "another,one", "normal"},
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(cs, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	// Unmarshal and verify round-trip
	var result CommaStruct
	err = UnmarshalQuery(query, &result, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equalf(t, cs, result, "Values with commas not preserved")

	// Verify each value is a separate query occurrence
	values, err := url.ParseQuery(query)
	assert.NoErrorf(t, err, "ParseQuery failed: %v", err)

	params := values["Values"]
	assert.NotNil(t, params, "Expected 'Values' parameter to be present")
	assert.Lenf(t, params, 4, "Expected 4 separate query occurrences")

	assert.Equalf(t, "value,with,comma", params[1], "Comma not preserved in value:")
}

// TestQueryValuesWithSpecialCharacters tests handling of quotes and other special characters
func TestQueryValuesWithSpecialCharacters(t *testing.T) {
	type SpecialStruct struct {
		Values []string
	}

	ss := SpecialStruct{
		Values: []string{
			"simple",
			`value"with"quotes`,
			"value with spaces",
			"value\twith\ttabs",
		},
	}

	opts := DefaultHTTPMarshalOptions()

	query, err := MarshalQuery(ss, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	var result SpecialStruct
	err = UnmarshalQuery(query, &result, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equalf(t, ss, result, "Special characters not preserved")
}

// TestQueryOrderPreservation verifies that the order of values is preserved
func TestQueryOrderPreservation(t *testing.T) {
	type OrderStruct struct {
		Ordered []string
	}

	os := OrderStruct{
		Ordered: []string{"first", "second", "third", "fourth", "fifth"},
	}

	opts := DefaultHTTPMarshalOptions()

	// Marshal
	query, err := MarshalQuery(os, opts)
	assert.NoErrorf(t, err, "MarshalQuery failed: %v", err)

	// Unmarshal
	var result OrderStruct
	err = UnmarshalQuery(query, &result, opts)
	assert.NoErrorf(t, err, "UnmarshalQuery failed: %v", err)

	assert.Equal(t, os.Ordered, result.Ordered, "Order of values not preserved")
}
