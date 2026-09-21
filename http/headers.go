package http

import "net/http"

// CreateHTTPHeaderFromMap creates a new http.Header from a map[string]string.
func CreateHTTPHeaderFromMap(headerMap map[string]string) http.Header {
	outputHeaders := http.Header{}

	return MergedHTTPHeader(outputHeaders, headerMap)
}

// MergedHTTPHeader returns a clone of baseHeaders with headerMap's entries
// set on top of it. baseHeaders itself is not mutated; use the returned
// http.Header.
func MergedHTTPHeader(baseHeaders http.Header, headerMap map[string]string) http.Header {
	outputHeaders := baseHeaders.Clone()
	for key, value := range headerMap {
		outputHeaders.Set(key, value)
	}

	return outputHeaders
}
