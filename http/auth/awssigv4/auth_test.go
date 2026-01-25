package awssigv4

import (
	"net/http"
	"testing"
	"time"
)

func TestCanonicalRequest(t *testing.T) {
	tests := []struct {
		name              string
		method            string
		uri               string
		query             string
		canonicalHeaders  string
		signedHeaders     string
		hashedPayload     string
		expectedCanonical string
	}{
		{
			name:              "simple GET request",
			method:            "GET",
			uri:               "/",
			query:             "",
			canonicalHeaders:  "host:example.com\nx-amz-date:20150830T123600Z\n",
			signedHeaders:     "host;x-amz-date",
			hashedPayload:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			expectedCanonical: "GET\n/\n\nhost:example.com\nx-amz-date:20150830T123600Z\n\nhost;x-amz-date\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:              "POST request with query params",
			method:            "POST",
			uri:               "/api/test",
			query:             "param1=value1&param2=value2",
			canonicalHeaders:  "host:api.example.com\nx-amz-date:20150830T123600Z\n",
			signedHeaders:     "host;x-amz-date",
			hashedPayload:     "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
			expectedCanonical: "POST\n/api/test\nparam1=value1&param2=value2\nhost:api.example.com\nx-amz-date:20150830T123600Z\n\nhost;x-amz-date\n44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanonicalRequest(
				tt.method,
				tt.uri,
				tt.query,
				tt.canonicalHeaders,
				tt.signedHeaders,
				tt.hashedPayload,
			)
			if result != tt.expectedCanonical {
				t.Errorf("CanonicalRequest() mismatch:\nGot:\n%s\n\nExpected:\n%s", result, tt.expectedCanonical)
			}
		})
	}
}

func TestStringToSign(t *testing.T) {
	timestamp, _ := time.Parse(TimeFormat, "20150830T123600Z")
	region := "us-east-1"
	service := "service"
	hashedCanonicalRequest := "abc123"

	result := StringToSign(timestamp, region, service, hashedCanonicalRequest)
	expected := "AWS4-HMAC-SHA256\n20150830T123600Z\n20150830/us-east-1/service/aws4_request\nabc123"

	if result != expected {
		t.Errorf("StringToSign() mismatch:\nGot:\n%s\n\nExpected:\n%s", result, expected)
	}
}

func TestCredentialScope(t *testing.T) {
	timestamp, _ := time.Parse(TimeFormat, "20150830T123600Z")
	region := "us-west-2"
	service := "s3"

	result := CredentialScope(timestamp, region, service)
	expected := "20150830/us-west-2/s3/aws4_request"

	if result != expected {
		t.Errorf("CredentialScope() = %s, want %s", result, expected)
	}
}

func TestDeriveSigningKey(t *testing.T) {
	secretKey := "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	timestamp, _ := time.Parse(TimeFormat, "20150830T123600Z")
	region := "us-east-1"
	service := "iam"

	signingKey := DeriveSigningKey(secretKey, timestamp, region, service)

	// Verify it's not empty and has expected length (32 bytes for SHA256)
	if len(signingKey) != 32 {
		t.Errorf("DeriveSigningKey() length = %d, want 32", len(signingKey))
	}
}

func TestCalculateSignature(t *testing.T) {
	signingKey := []byte("test-signing-key")
	stringToSign := "test-string-to-sign"

	signature := CalculateSignature(signingKey, stringToSign)

	// Verify signature is hex-encoded (64 characters for SHA256)
	if len(signature) != 64 {
		t.Errorf("CalculateSignature() length = %d, want 64", len(signature))
	}

	// Verify it's deterministic
	signature2 := CalculateSignature(signingKey, stringToSign)
	if signature != signature2 {
		t.Errorf("CalculateSignature() not deterministic")
	}
}

func TestHashSHA256(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty string",
			data:     []byte(""),
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "test string",
			data:     []byte("test"),
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashSHA256(tt.data)
			if result != tt.expected {
				t.Errorf("hashSHA256() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCanonicalHeaders(t *testing.T) {
	headers := http.Header{
		"Host":       []string{"example.com"},
		"X-Amz-Date": []string{"20150830T123600Z"},
		"Content-Type": []string{"application/json"},
	}

	signedHeaders := []string{"host", "x-amz-date"}
	result := CanonicalHeaders(headers, signedHeaders)
	expected := "host:example.com\nx-amz-date:20150830T123600Z\n"

	if result != expected {
		t.Errorf("CanonicalHeaders() = %q, want %q", result, expected)
	}
}

func TestCanonicalHeaders_MultipleValues(t *testing.T) {
	headers := http.Header{
		"Host":          []string{"example.com"},
		"X-Custom-Header": []string{"  value1  ", "  value2  "},
	}

	signedHeaders := []string{"host", "x-custom-header"}
	result := CanonicalHeaders(headers, signedHeaders)
	expected := "host:example.com\nx-custom-header:value1,value2\n"

	if result != expected {
		t.Errorf("CanonicalHeaders() = %q, want %q", result, expected)
	}
}

func TestSignedHeadersList(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		expected string
	}{
		{
			name:     "single header",
			headers:  []string{"Host"},
			expected: "host",
		},
		{
			name:     "multiple headers",
			headers:  []string{"Host", "X-Amz-Date", "Content-Type"},
			expected: "content-type;host;x-amz-date",
		},
		{
			name:     "mixed case headers",
			headers:  []string{"HOST", "x-amz-date", "Content-Type"},
			expected: "content-type;host;x-amz-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SignedHeadersList(tt.headers)
			if result != tt.expected {
				t.Errorf("SignedHeadersList() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCanonicalQueryString(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "empty query",
			query:    "",
			expected: "",
		},
		{
			name:     "single parameter",
			query:    "param=value",
			expected: "param=value",
		},
		{
			name:     "multiple parameters unsorted",
			query:    "z=last&a=first&m=middle",
			expected: "a=first&m=middle&z=last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanonicalQueryString(tt.query)
			if result != tt.expected {
				t.Errorf("CanonicalQueryString() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestParseAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name                   string
		authHeader             string
		expectedAccessKeyID    string
		expectedCredentialScope string
		expectedSignedHeaders  string
		expectedSignature      string
		expectError            bool
	}{
		{
			name:                   "valid authorization header",
			authHeader:             "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;range;x-amz-date, Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expectedAccessKeyID:    "AKIAIOSFODNN7EXAMPLE",
			expectedCredentialScope: "20130524/us-east-1/s3/aws4_request",
			expectedSignedHeaders:  "host;range;x-amz-date",
			expectedSignature:      "fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024",
			expectError:            false,
		},
		{
			name:        "invalid scheme",
			authHeader:  "Bearer token123",
			expectError: true,
		},
		{
			name:        "missing components",
			authHeader:  "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request",
			expectError: true,
		},
		{
			name:        "empty header",
			authHeader:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessKeyID, credentialScope, signedHeaders, signature, err := ParseAuthorizationHeader(tt.authHeader)

			if tt.expectError {
				if err == nil {
					t.Error("ParseAuthorizationHeader() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAuthorizationHeader() unexpected error: %v", err)
				return
			}

			if accessKeyID != tt.expectedAccessKeyID {
				t.Errorf("accessKeyID = %s, want %s", accessKeyID, tt.expectedAccessKeyID)
			}
			if credentialScope != tt.expectedCredentialScope {
				t.Errorf("credentialScope = %s, want %s", credentialScope, tt.expectedCredentialScope)
			}
			if signedHeaders != tt.expectedSignedHeaders {
				t.Errorf("signedHeaders = %s, want %s", signedHeaders, tt.expectedSignedHeaders)
			}
			if signature != tt.expectedSignature {
				t.Errorf("signature = %s, want %s", signature, tt.expectedSignature)
			}
		})
	}
}
