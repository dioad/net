package awssigv4

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	authcontext "github.com/dioad/net/http/auth/context"
)

// TestClientHandlerIntegration tests the full client-to-server flow with AWS SigV4 authentication.
func TestClientHandlerIntegration(t *testing.T) {
	const accessKeyID = "AKIAIOSFODNN7EXAMPLE"
	const secretAccessKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	const region = "us-east-1"
	const service = "execute-api"

	// Create server handler
	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			Region:  region,
			Service: service,
		},
		VerifyCredentials: false, // Skip AWS STS verification for testing
	})

	// Create test server
	testServer := httptest.NewServer(
		serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := authcontext.AuthenticatedPrincipalFromContext(r.Context())
			if principal != accessKeyID {
				t.Errorf("expected principal %q, got %q", accessKeyID, principal)
			}

			// Check AWS principal in context
			awsPrincipal := AWSPrincipalFromContext(r.Context())
			if awsPrincipal == nil {
				t.Error("expected AWS principal in context, got nil")
			} else {
				if awsPrincipal.AccessKeyID != accessKeyID {
					t.Errorf("expected AccessKeyID %q, got %q", accessKeyID, awsPrincipal.AccessKeyID)
				}
				if awsPrincipal.Region != region {
					t.Errorf("expected Region %q, got %q", region, awsPrincipal.Region)
				}
				if awsPrincipal.Service != service {
					t.Errorf("expected Service %q, got %q", service, awsPrincipal.Service)
				}
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authenticated"))
		})),
	)
	defer testServer.Close()

	// Create client with static credentials
	awsConfig := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"", // no session token
		),
	}

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				Region:  region,
				Service: service,
			},
			AWSConfig: awsConfig,
		},
	}

	// Create and sign request
	req, err := http.NewRequest("GET", testServer.URL+"/api/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if err := clientAuth.AddAuth(req); err != nil {
		t.Fatalf("failed to add auth: %v", err)
	}

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestClientHandlerIntegration_POST tests POST requests with body.
func TestClientHandlerIntegration_POST(t *testing.T) {
	const accessKeyID = "AKIAIOSFODNN7EXAMPLE"
	const secretAccessKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	const region = "us-west-2"
	const service = "execute-api"
	const requestBody = `{"message": "hello world"}`

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			Region:  region,
			Service: service,
		},
		VerifyCredentials: false,
	})

	testServer := httptest.NewServer(
		serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)
	defer testServer.Close()

	awsConfig := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		),
	}

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				Region:  region,
				Service: service,
			},
			AWSConfig: awsConfig,
		},
	}

	req, err := http.NewRequest("POST", testServer.URL+"/api/action", bytes.NewBufferString(requestBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if err := clientAuth.AddAuth(req); err != nil {
		t.Fatalf("failed to add auth: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestClientHandlerIntegration_WithSessionToken tests requests with session token.
func TestClientHandlerIntegration_WithSessionToken(t *testing.T) {
	const accessKeyID = "ASIAIOSFODNN7EXAMPLE"
	const secretAccessKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	const sessionToken = "FwoGZXIvYXdzEBQaDDU2NzExMjM0NTY3OCKgASABCiEKHgoQPPIaQbGvRdExampleToken"
	const region = "eu-west-1"
	const service = "s3"

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			Region:  region,
			Service: service,
		},
		VerifyCredentials: false,
	})

	testServer := httptest.NewServer(
		serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify session token header is present
			if r.Header.Get(SecurityTokenHeader) != sessionToken {
				t.Errorf("expected session token %q, got %q", sessionToken, r.Header.Get(SecurityTokenHeader))
			}
			w.WriteHeader(http.StatusOK)
		})),
	)
	defer testServer.Close()

	awsConfig := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			sessionToken,
		),
	}

	clientAuth := ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				Region:  region,
				Service: service,
			},
			AWSConfig: awsConfig,
		},
	}

	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if err := clientAuth.AddAuth(req); err != nil {
		t.Fatalf("failed to add auth: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestHandlerInvalidAuth tests various invalid authentication scenarios.
func TestHandlerInvalidAuth(t *testing.T) {
	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			Region:  "us-east-1",
			Service: "execute-api",
		},
		VerifyCredentials: false,
	})

	tests := []struct {
		name           string
		modifyRequest  func(*http.Request)
		expectedStatus int
	}{
		{
			name: "missing authorization header",
			modifyRequest: func(r *http.Request) {
				// No authorization header
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid authorization scheme",
			modifyRequest: func(r *http.Request) {
				r.Header.Set(AuthorizationHeader, "Bearer token123")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "missing timestamp",
			modifyRequest: func(r *http.Request) {
				r.Header.Set(AuthorizationHeader, "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123")
				// No X-Amz-Date header
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "expired timestamp",
			modifyRequest: func(r *http.Request) {
				oldTime := time.Now().UTC().Add(-10 * time.Minute)
				r.Header.Set(DateHeader, oldTime.Format(TimeFormat))
				r.Header.Set(AuthorizationHeader, "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123")
				r.Header.Set(ContentSHA256Header, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.modifyRequest(req)

			rr := httptest.NewRecorder()
			handler := serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

// TestRoundTripper tests the AWSSigV4RoundTripper.
func TestRoundTripper(t *testing.T) {
	const accessKeyID = "AKIAIOSFODNN7EXAMPLE"
	const secretAccessKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	const region = "us-east-1"
	const service = "execute-api"

	serverHandler := NewHandler(ServerConfig{
		CommonConfig: CommonConfig{
			Region:  region,
			Service: service,
		},
		VerifyCredentials: false,
	})

	testServer := httptest.NewServer(
		serverHandler.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})),
	)
	defer testServer.Close()

	awsConfig := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		),
	}

	client := &http.Client{
		Transport: &AWSSigV4RoundTripper{
			Config: ClientConfig{
				CommonConfig: CommonConfig{
					Region:  region,
					Service: service,
				},
				AWSConfig: awsConfig,
			},
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}
