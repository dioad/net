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

	opts := HeaderMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check field one
	if got := header.Get("X-Example-Field-One"); got != "value1" {
		t.Errorf("X-Example-Field-One = %q, want %q", got, "value1")
	}

	// Check field two (multiple values)
	values := header.Values("X-Example-Field-Two")
	want := []string{"value2", "value3"}
	if diff := cmp.Diff(want, values); diff != "" {
		t.Errorf("X-Example-Field-Two mismatch (-want +got):\n%s", diff)
	}
}

// TestMarshalHeaderWithoutStructName tests encoding without struct name
func TestMarshalHeaderWithoutStructName(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := HeaderMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
	}

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check field one (without struct name)
	if got := header.Get("X-Field-One"); got != "value1" {
		t.Errorf("X-Field-One = %q, want %q", got, "value1")
	}

	// Check field two
	if got := header.Get("X-Field-Two"); got != "value2" {
		t.Errorf("X-Field-Two = %q, want %q", got, "value2")
	}
}

// TestMarshalHeaderNoPrefix tests encoding without prefix
func TestMarshalHeaderNoPrefix(t *testing.T) {
	example := Example{
		FieldOne: "value1",
		FieldTwo: []string{"value2"},
	}

	opts := DefaultHeaderMarshalOptions()

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Check field one (no prefix, no struct name)
	if got := header.Get("field-one"); got != "value1" {
		t.Errorf("field-one = %q, want %q", got, "value1")
	}

	// Check field two
	if got := header.Get("field-two"); got != "value2" {
		t.Errorf("field-two = %q, want %q", got, "value2")
	}
}

// TestUnmarshalHeaderWithPrefixAndStructName tests decoding with prefix and struct name
func TestUnmarshalHeaderWithPrefixAndStructName(t *testing.T) {
	header := http.Header{}
	header.Set("X-Example-Field-One", "value1")
	header.Add("X-Example-Field-Two", "value2")
	header.Add("X-Example-Field-Two", "value3")

	opts := HeaderMarshalOptions{
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
	header.Set("X-Field-One", "value1")
	header.Set("X-Field-Two", "value2")

	opts := HeaderMarshalOptions{
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

	opts := HeaderMarshalOptions{
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

	opts := HeaderMarshalOptions{
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

	opts := DefaultHeaderMarshalOptions()

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

	opts := DefaultHeaderMarshalOptions()

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

	opts := DefaultHeaderMarshalOptions()

	header, err := MarshalHeader(example, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	if got := header.Get("field-one"); got != "value1" {
		t.Errorf("field-one = %q, want %q", got, "value1")
	}
}

// TestUnmarshalInvalidDestination tests error handling for invalid destinations
func TestUnmarshalInvalidDestination(t *testing.T) {
	header := http.Header{}
	header.Set("field-one", "value1")

	opts := DefaultHeaderMarshalOptions()

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

	opts := DefaultHeaderMarshalOptions()

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
		{"HTTPHeader", "h-t-t-p-header"},
		{"Example", "example"},
		{"APIKey", "a-p-i-key"},
		{"UserID", "user-i-d"},
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

// TestMultipleTypes tests marshaling different field types
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

	opts := DefaultHeaderMarshalOptions()

	header, err := MarshalHeader(mt, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	if got := header.Get("string-field"); got != "test" {
		t.Errorf("string-field = %q, want %q", got, "test")
	}

	if got := header.Get("int-field"); got != "42" {
		t.Errorf("int-field = %q, want %q", got, "42")
	}

	if got := header.Get("uint-field"); got != "100" {
		t.Errorf("uint-field = %q, want %q", got, "100")
	}

	if got := header.Get("bool-field"); got != "true" {
		t.Errorf("bool-field = %q, want %q", got, "true")
	}

	values := header.Values("slice-field")
	want := []string{"a", "b"}
	if diff := cmp.Diff(want, values); diff != "" {
		t.Errorf("slice-field mismatch (-want +got):\n%s", diff)
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

	opts := DefaultHeaderMarshalOptions()

	header, err := MarshalHeader(s, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Only exported field should be present
	if got := header.Get("exported-field"); got != "visible" {
		t.Errorf("exported-field = %q, want %q", got, "visible")
	}

	// Unexported field should not be present
	if _, ok := header["unexported-field"]; ok {
		t.Error("unexported-field should not be present")
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

	opts := DefaultHeaderMarshalOptions()

	header, err := MarshalHeader(s, opts)
	if err != nil {
		t.Fatalf("MarshalHeader failed: %v", err)
	}

	// Empty slices should not create headers
	if len(header) != 0 {
		t.Errorf("Expected empty header, got %v", header)
	}
}
