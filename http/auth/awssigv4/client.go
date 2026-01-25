package awssigv4

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// ClientAuth implements AWS SigV4 authentication for an HTTP client.
type ClientAuth struct {
	Config ClientConfig
}

// AddAuth adds AWS SigV4 authentication to the request.
// It signs the request using the credentials from the AWS config.
func (a ClientAuth) AddAuth(req *http.Request) error {
	// Read and restore the request body
	bodyBytes := []byte{}
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		// Configure GetBody for retries
		if req.GetBody == nil {
			bodyCopy := append([]byte(nil), bodyBytes...)
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyCopy)), nil
			}
		}
	}

	// Get AWS credentials
	creds, err := a.Config.AWSConfig.Credentials.Retrieve(req.Context())
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	// Set timestamp
	timestamp := time.Now().UTC()
	req.Header.Set(DateHeader, timestamp.Format(TimeFormat))

	// Calculate payload hash
	payloadHash := hashSHA256(bodyBytes)
	req.Header.Set(ContentSHA256Header, payloadHash)

	// Add session token if present
	if creds.SessionToken != "" {
		req.Header.Set(SecurityTokenHeader, creds.SessionToken)
	}

	// Determine signed headers
	signedHeaders := []string{"host", DateHeader, ContentSHA256Header}
	if creds.SessionToken != "" {
		signedHeaders = append(signedHeaders, SecurityTokenHeader)
	}

	// Ensure host header is set
	if req.Host != "" {
		req.Header.Set("Host", req.Host)
	} else {
		req.Header.Set("Host", req.URL.Host)
	}

	// Build canonical request
	uri := req.URL.Path
	if uri == "" {
		uri = "/"
	}
	query := CanonicalQueryString(req.URL.RawQuery)
	canonicalHeaders := CanonicalHeaders(req.Header, signedHeaders)
	signedHeadersList := SignedHeadersList(signedHeaders)
	canonicalRequest := CanonicalRequest(
		req.Method,
		uri,
		query,
		canonicalHeaders,
		signedHeadersList,
		payloadHash,
	)

	// Build string to sign
	hashedCanonicalRequest := hashSHA256([]byte(canonicalRequest))
	stringToSign := StringToSign(timestamp, a.Config.Region, a.Config.Service, hashedCanonicalRequest)

	// Calculate signature
	signingKey := DeriveSigningKey(creds.SecretAccessKey, timestamp, a.Config.Region, a.Config.Service)
	signature := CalculateSignature(signingKey, stringToSign)

	// Build Authorization header
	credentialScope := CredentialScope(timestamp, a.Config.Region, a.Config.Service)
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		AuthScheme,
		creds.AccessKeyID,
		credentialScope,
		signedHeadersList,
		signature,
	)
	req.Header.Set(AuthorizationHeader, authHeader)

	return nil
}

// HTTPClient returns an http.Client that automatically adds AWS SigV4 authentication to requests.
func (a ClientAuth) HTTPClient() *http.Client {
	return &http.Client{
		Transport: &AWSSigV4RoundTripper{
			Config: a.Config,
		},
	}
}

// AWSSigV4RoundTripper is an http.RoundTripper that adds AWS SigV4 authentication.
type AWSSigV4RoundTripper struct {
	Config ClientConfig
	Base   http.RoundTripper
}

// RoundTrip executes a single HTTP transaction, adding AWS SigV4 authentication.
func (t *AWSSigV4RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())

	clientAuth := ClientAuth{Config: t.Config}
	if err := clientAuth.AddAuth(req); err != nil {
		return nil, err
	}

	if t.Base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}

	return t.Base.RoundTrip(req)
}

// NewClientAuth creates a new ClientAuth with the provided AWS config.
// The region and service must be specified in the config.
func NewClientAuth(awsConfig aws.Config, region, service string) *ClientAuth {
	return &ClientAuth{
		Config: ClientConfig{
			CommonConfig: CommonConfig{
				Region:  region,
				Service: service,
			},
			AWSConfig: awsConfig,
		},
	}
}
