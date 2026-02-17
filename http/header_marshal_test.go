package http

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Example struct from the issue
type Example struct {
	FieldOne string
	FieldTwo []string
}

// TestMarshalHeaderWithPrefixAndStructName tests the example from the issue
func TestMarshalHeaderWithPrefixAndStructName(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2", "value3"},
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check fieldSet one
	if got := header.Get("X-Example-FieldOne"); got != "value1" {
		t.Errorf("X-Example-FieldOne = %q, want %q", got, "value1")
	}

	// Check fieldSet two (multiple values)
	values := header.Values("X-Example-FieldTwo")
	want := []string{"value2", "value3"}
	if diff := cmp.Diff(want, values); diff != "" {
		t.Errorf("X-Example-FieldTwo mismatch (-want +got):\n%s", diff)
	}
}

// TestMarshalHeaderWithoutStructName tests encoding without struct name
func TestMarshalHeaderWithoutStructName(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
	}

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check fieldSet one (without struct name)
	if got := header.Get("X-FieldOne"); got != "value1" {
		t.Errorf("X-FieldOne = %q, want %q", got, "value1")
	}

	// Check fieldSet two
	if got := header.Get("X-FieldTwo"); got != "value2" {
		t.Errorf("X-FieldTwo = %q, want %q", got, "value2")
	}
}

// TestMarshalHeaderNoPrefix tests encoding without prefix
func TestMarshalHeaderNoPrefix(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check fieldSet one (no prefix, no struct name)
	if got := header.Get("FieldOne"); got != "value1" {
		t.Errorf("FieldOne = %q, want %q", got, "value1")
	}

	// Check fieldSet two
	if got := header.Get("FieldTwo"); got != "value2" {
		t.Errorf("FieldTwo = %q, want %q", got, "value2")
	}
}

// TestUnmarshalHeaderWithPrefixAndStructName tests decoding with prefix and struct name
func TestUnmarshalHeaderWithPrefixAndStructName(t *testing.T) {
	header := http.Header{}
	header.Set("X-Example-FieldOne", "value1")
	header.Add("X-Example-FieldTwo", "value2")
	header.Add("X-Example-FieldTwo", "value3")

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	var example Example
	err := UnmarshalHeader(header, &example, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	if example.FieldOne != "value1" {
		t.Errorf("FieldOne = %q, want %q", example.FieldOne, "value1")
	}

	want := []string{"value2", "value3"}
	if diff := cmp.Diff(want, example.FieldTwo); diff != "" {
		t.Errorf("FieldTwo mismatch (-want +got):\n%s", diff)
	}
}

// TestUnmarshalHeaderWithoutStructName tests decoding without struct name
func TestUnmarshalHeaderWithoutStructName(t *testing.T) {
	header := http.Header{}
	header.Set("X-FieldOne", "value1")
	header.Set("X-FieldTwo", "value2")

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
	}

	var example Example
	err := UnmarshalHeader(header, &example, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	if example.FieldOne != "value1" {
		t.Errorf("FieldOne = %q, want %q", example.FieldOne, "value1")
	}

	want := []string{"value2"}
	if diff := cmp.Diff(want, example.FieldTwo); diff != "" {
		t.Errorf("FieldTwo mismatch (-want +got):\n%s", diff)
	}
}

// TestMarshalUnmarshalRoundTrip tests that marshal followed by unmarshal returns the original
func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	original := Example{
		FieldOne: "test-value",
		FieldTwo: []string{"a", "b", "c"},
	}

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	// Marshal
	header, err := MarshalHeader(original, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Unmarshal
	var result Example
	err = UnmarshalHeader(header, &result, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	// Compare
	if diff := cmp.Diff(original, result); diff != "" {
		t.Errorf("Round trip mismatch (-want +got):\n%s", diff)
	}
}

// TestStructWithTags tests struct with custom header tags
func TestStructWithTags(t *testing.T) {
	type CustomStruct struct {
		Field1 string   `header:"custom-name"`
		Field2 []string `header:"another-name"`
		Field3 string   `header:"-"` // Should be ignored
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

	header, err := MarshalHeader(cs, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check custom names
	if got := header.Get("X-custom-name"); got != "value1" {
		t.Errorf("X-custom-name = %q, want %q", got, "value1")
	}

	if got := header.Get("X-another-name"); got != "value2" {
		t.Errorf("X-another-name = %q, want %q", got, "value2")
	}

	// Field3 should not be present
	if _, ok := header["X-Field3"]; ok {
		t.Error("X-Field3 should not be present (marked with '-' tag)")
	}
}

// TestMarshalEmptyStruct tests marshaling an empty struct
func TestMarshalEmptyStruct(t *testing.T) {
	example := Example{}

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Empty strings should not create headers
	if len(header) != 0 {
		t.Errorf("Expected empty header, got %v", header)
	}
}

// TestMarshalNilPointer tests marshaling a nil pointer
func TestMarshalNilPointer(t *testing.T) {
	var example *Example

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	if len(header) != 0 {
		t.Errorf("Expected empty header, got %v", header)
	}
}

// TestMarshalPointerToStruct tests marshaling a pointer to a struct
func TestMarshalPointerToStruct(t *testing.T) {
	example := &Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	if got := header.Get("FieldOne"); got != "value1" {
		t.Errorf("FieldOne = %q, want %q", got, "value1")
	}
}

// TestUnmarshalInvalidDestination tests error handling for invalid destinations
func TestUnmarshalInvalidDestination(t *testing.T) {
	header := http.Header{}
	header.Set("field-one", "value1")

	opts := DefaultHTTPMarshalOptions()

	// Test with non-pointer
	var example Example
	err := UnmarshalHeader(header, example, opts)
	if err == nil {
		t.Error("Expected error when unmarshaling to non-pointer, got nil")
	}

	// Test with nil
	err = UnmarshalHeader(header, nil, opts)
	if err == nil {
		t.Error("Expected error when unmarshaling to nil, got nil")
	}

	// Test with nil pointer
	var nilPtr *Example
	err = UnmarshalHeader(header, nilPtr, opts)
	if err == nil {
		t.Error("Expected error when unmarshaling to nil pointer, got nil")
	}
}

// TestMarshalNonStruct tests error handling for non-struct types
func TestMarshalNonStruct(t *testing.T) {
	notAStruct := "string"

	opts := DefaultHTTPMarshalOptions()

	_, err := MarshalHeader(notAStruct, opts)
	if err == nil {
		t.Error("Expected error when marshaling non-struct, got nil")
	}
}

// TestKebabCase tests the toKebabCase function
func TestKebabCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"FieldOne", "field-one"},
		{"FieldTwo", "field-two"},
		{"HTTPHeader", "http-header"},
		{"Example", "example"},
		{"APIKey", "api-key"},
		{"UserID", "user-id"},
		{"", ""},
		{"lowercase", "lowercase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toKebabCase(tt.input)
			if got != tt.want {
				t.Errorf("toKebabCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestMultipleTypes tests marshaling different fieldSet types
func TestMultipleTypes(t *testing.T) {
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

	header, err := MarshalHeader(mt, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	if got := header.Get("StringField"); got != "test" {
		t.Errorf("StringField = %q, want %q", got, "test")
	}

	if got := header.Get("IntField"); got != "42" {
		t.Errorf("IntField = %q, want %q", got, "42")
	}

	if got := header.Get("UintField"); got != "100" {
		t.Errorf("UintField = %q, want %q", got, "100")
	}

	if got := header.Get("BoolField"); got != "true" {
		t.Errorf("BoolField = %q, want %q", got, "true")
	}

	values := header.Values("SliceField")
	want := []string{"a", "b"}
	if diff := cmp.Diff(want, values); diff != "" {
		t.Errorf("SliceField mismatch (-want +got):\n%s", diff)
	}

	// Test unmarshal
	var result MultiTypeStruct
	err = UnmarshalHeader(header, &result, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	if diff := cmp.Diff(mt, result); diff != "" {
		t.Errorf("Round trip mismatch (-want +got):\n%s", diff)
	}
}

// TestUnexportedFields tests that unexported fields are skipped
func TestUnexportedFields(t *testing.T) {
	type StructWithUnexported struct {
		ExportedField   string
		unexportedField string
	}

	s := StructWithUnexported{
		ExportedField:   "visible",
		unexportedField: "hidden",
	}

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(s, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Only exported fieldSet should be present
	if got := header.Get("ExportedField"); got != "visible" {
		t.Errorf("ExportedField = %q, want %q", got, "visible")
	}

	// Unexported fieldSet should not be present
	if _, ok := header["UnexportedField"]; ok {
		t.Error("UnexportedField should not be present")
	}
}

// TestEmptySlice tests handling of empty slices
func TestEmptySlice(t *testing.T) {
	type StructWithSlice struct {
		EmptySlice []string
		NilSlice   []string
	}

	s := StructWithSlice{
		EmptySlice: []string{},
		NilSlice:   nil,
	}

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(s, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Empty slices should not create headers
	if len(header) != 0 {
		t.Errorf("Expected empty header, got %v", header)
	}
}

// TestRFC9110MultipleHeaderOccurrences verifies RFC 9110 Section 5.5 compliance
// Multiple header fieldSet occurrences should be unmarshaled into a slice
func TestRFC9110MultipleHeaderOccurrences(t *testing.T) {
	// Simulate receiving headers with multiple occurrences
	// RFC 9110 allows: X-Field: value1
	//                  X-Field: value2
	header := http.Header{}
	header.Add("X-Example-FieldTwo", "value1")
	header.Add("X-Example-FieldTwo", "value2")
	header.Add("X-Example-FieldTwo", "value3")

	opts := HTTPMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	var example Example
	err := UnmarshalHeader(header, &example, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	want := []string{"value1", "value2", "value3"}
	if diff := cmp.Diff(want, example.FieldTwo); diff != "" {
		t.Errorf("Multiple header occurrences not unmarshaled correctly (-want +got):\n%s", diff)
	}

	// Verify marshaling creates multiple occurrences
	header2, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	values := header2.Values("X-Example-FieldTwo")
	if diff := cmp.Diff(want, values); diff != "" {
		t.Errorf("Multiple values not marshaled correctly (-want +got):\n%s", diff)
	}
}

// TestValuesWithCommas tests that values containing commas are preserved
// RFC 9110 allows comma-separated lists, but when using multiple header occurrences,
// each value is kept separate and commas within values are preserved
func TestValuesWithCommas(t *testing.T) {
	type CommaStruct struct {
		Values []string
	}

	// Test marshaling values with commas
	cs := CommaStruct{
		Values: []string{"simple", "value,with,comma", "another,one", "normal"},
	}

	opts := DefaultHTTPMarshalOptions()

	header, err := MarshalHeader(cs, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Unmarshal and verify round-trip
	var result CommaStruct
	err = UnmarshalHeader(header, &result, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	if diff := cmp.Diff(cs, result); diff != "" {
		t.Errorf("Values with commas not preserved (-want +got):\n%s", diff)
	}

	// Verify each value is a separate header occurrence
	values := header.Values("values")
	if len(values) != 4 {
		t.Errorf("Expected 4 separate header occurrences, got %d", len(values))
	}

	// Verify commas are preserved within each value
	if values[1] != "value,with,comma" {
		t.Errorf("Comma not preserved in value: got %q, want %q", values[1], "value,with,comma")
	}
}

// TestValuesWithSpecialCharacters tests handling of quotes and other special characters
func TestValuesWithSpecialCharacters(t *testing.T) {
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

	header, err := MarshalHeader(ss, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	var result SpecialStruct
	err = UnmarshalHeader(header, &result, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	if diff := cmp.Diff(ss, result); diff != "" {
		t.Errorf("Special characters not preserved (-want +got):\n%s", diff)
	}
}

// TestRFC9110OrderPreservation verifies that the order of values is preserved
// per RFC 9110 requirements
func TestRFC9110OrderPreservation(t *testing.T) {
	type OrderStruct struct {
		Ordered []string
	}

	os := OrderStruct{
		Ordered: []string{"first", "second", "third", "fourth", "fifth"},
	}

	opts := DefaultHTTPMarshalOptions()

	// Marshal
	header, err := MarshalHeader(os, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Unmarshal
	var result OrderStruct
	err = UnmarshalHeader(header, &result, opts)
	if err != nil {
		t.Fatalf("UnmarshalHeader failed: %v", err)
	}

	// Verify order is preserved
	if diff := cmp.Diff(os.Ordered, result.Ordered); diff != "" {
		t.Errorf("Order not preserved (-want +got):\n%s", diff)
	}
}
