package http

import "net/http"

// CreateHTTPHeaderFromMap creates a new http.Header from a map[string]string
func CreateHTTPHeaderFromMap(headerMap map[string]string) http.Header {
	outputHeaders := http.Header{}

	return AddMapToHTTPHeader(outputHeaders, headerMap)
}

// AddMapToHTTPHeader adds a map[string]string to an existing http.Header
func AddMapToHTTPHeader(baseHeaders http.Header, headerMap map[string]string) http.Header {
	outputHeaders := baseHeaders.Clone()
	for key, value := range headerMap {
		outputHeaders.Set(key, value)
	}

	return outputHeaders
}
