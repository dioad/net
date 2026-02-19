package http

import (
	"fmt"
	"net/http"
)

const (
	// HeaderMarshalTagName is the struct tag name used for marshaling/unmarshaling HTTP headers
	HeaderMarshalTagName = "header"
)

// MarshalHeader encodes a struct into an http.Header using the provided options.
//
// RFC 9110 Compliance:
// For slice fields ([]string), each element is added as a separate header occurrence
// using http.Header.Add(). This is compliant with RFC 9110 Section 5.5, which allows
// multiple header fieldSet lines with the same name. Values containing commas, quotes,
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
func MarshalHeader(v any, opts HTTPMarshalOptions) (http.Header, error) {

	header := http.Header{}

	if isNilAny(v) {
		return header, nil
	}

	err := marshalFields(v, HeaderMarshalTagName, header, opts)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}

	return header, nil
}

// UnmarshalHeader decodes an http.Header into a struct using the provided options.
//
// RFC 9110 Compliance:
// Multiple header fieldSet occurrences with the same name are unmarshaled into slice
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
//	err := UnmarshalHeader(headers, &ex, opts)
//
// Results in: ex.Values = []string{"val1", "val2,with,comma"}
func UnmarshalHeader(header http.Header, v any, opts HTTPMarshalOptions) error {
	tagName := HeaderMarshalTagName

	err := unmarshalFields(header, v, tagName, opts)
	if err != nil {
		return fmt.Errorf("UnmarshalHeader: %w", err)
	}

	return nil
}

// UnmarshalHeaderFromRequest is a helper function that extracts headers from an http.Request and unmarshals them into a
// struct using the provided options.
func UnmarshalHeaderFromRequest(req *http.Request, v any, opts HTTPMarshalOptions) error {
	return UnmarshalHeader(req.Header, v, opts)
}
