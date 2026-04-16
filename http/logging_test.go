package http

import "testing"

func TestHeaderToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "X-Forwarded-For",
			input:    "X-Forwarded-For",
			expected: "x_forwarded_for",
		},
		{
			name:     "Content-Type",
			input:    "Content-Type",
			expected: "content_type",
		},
		{
			name:     "User-Agent",
			input:    "User-Agent",
			expected: "user_agent",
		},
		{
			name:     "Authorization",
			input:    "Authorization",
			expected: "authorization",
		},
		{
			name:     "Already snake case",
			input:    "x_request_id",
			expected: "x_request_id",
		},
		{
			name:     "Mixed case with hyphens",
			input:    "x-FoRwArDeD-fOr",
			expected: "x_forwarded_for",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single character",
			input:    "X",
			expected: "x",
		},
		{
			name:     "Hyphen at start",
			input:    "-X-Header",
			expected: "_x_header",
		},
		{
			name:     "Hyphen at end",
			input:    "X-Header-",
			expected: "x_header_",
		},
		{
			name:     "Multiple hyphens",
			input:    "X--Forwarded--For",
			expected: "x__forwarded__for",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := headerToSnakeCase(tt.input)
			if actual != tt.expected {
				t.Errorf("headerToSnakeCase(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}
