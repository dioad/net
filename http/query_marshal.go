package http

import (
	"fmt"
	"net/http"
	"net/url"
)

const (
	// QueryMarshalTagName is the struct tag name used for marshaling/unmarshaling URI query parameters
	QueryMarshalTagName = "query"
)

// urlValuesWrapper is a wrapper around url.Values to implement the fieldSet interface for marshaling/unmarshaling
// query parameters.
type urlValuesWrapper struct {
	values url.Values
}

// Set sets the key to value. It replaces any existing values.
func (f *urlValuesWrapper) Set(key, value string) {
	f.values.Set(key, value)
}

// Add adds the value to key. It appends to any existing values.
func (f *urlValuesWrapper) Add(key, value string) {
	f.values.Add(key, value)
}

// Values returns all values associated with the given key.
func (f *urlValuesWrapper) Values(key string) []string {
	return f.values[key]
}

// MarshalQuery encodes a struct into a URI query string using the provided options.
//
// Example usage:
// type QueryParams struct {
// Search string `query:"search"`
// Tags []string `query:"tags"`
// }
// params := QueryParams{Search: "example", Tags: []string{"go", "http"}}
// queryString, err := MarshalQuery(params, opts)
//

func MarshalQuery(v interface{}, opts HTTPMarshalOptions) (string, error) {
	values := url.Values{}

	valueWrapper := &urlValuesWrapper{values: values}

	err := marshalFields(v, QueryMarshalTagName, valueWrapper, opts)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}

	return values.Encode(), nil
}

// UnmarshalQuery decodes a URI query string into a struct using the provided options.
// The rawQuery parameter should be the part of the URL after the '?' character, without the '?' itself.
// Example usage:
// type QueryParams struct {
// Search string `query:"search"`
// Tags []string `query:"tags"`
// }
// var params QueryParams
// err := UnmarshalQuery("search=example&tags=go&tags=http", &params, opts)
//
// Results in: params.Search = "example", params.Tags = []string{"go", "http"}
func UnmarshalQuery(rawQuery string, v interface{}, opts HTTPMarshalOptions) error {
	tagName := QueryMarshalTagName

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return fmt.Errorf("UnmarshalQuery: invalid query string: %w", err)
	}

	urlValues := &urlValuesWrapper{values: values}

	err = unmarshalFields(urlValues, v, tagName, opts)
	if err != nil {
		return fmt.Errorf("UnmarshalQuery: %w", err)
	}

	return nil
}

func UnmarshalQueryFromRequest(req *http.Request, v interface{}, opts HTTPMarshalOptions) error {
	return UnmarshalQuery(req.URL.RawQuery, v, opts)
}
