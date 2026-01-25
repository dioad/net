package awssigv4

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// CommonConfig contains shared configuration for AWS SigV4 authentication.
type CommonConfig struct {
	// Region is the AWS region (e.g., "us-east-1")
	Region string `mapstructure:"region"`
	// Service is the AWS service name (e.g., "execute-api", "s3")
	Service string `mapstructure:"service"`
	// AllowInsecureHTTP allows HTTP connections (for testing only)
	AllowInsecureHTTP bool `mapstructure:"allow-insecure-http"`
}

// ClientConfig contains configuration for an AWS SigV4 authentication client.
type ClientConfig struct {
	CommonConfig `mapstructure:",squash"`
	// AWSConfig is the AWS SDK v2 configuration.
	// This should be loaded using config.LoadDefaultConfig() or similar.
	// The credentials provider from this config will be used to sign requests.
	AWSConfig aws.Config
}

// ServerConfig contains configuration for an AWS SigV4 authentication server.
type ServerConfig struct {
	CommonConfig `mapstructure:",squash"`
	// MaxTimestampDiff is the maximum allowed time difference for request timestamps (default: 5m)
	MaxTimestampDiff time.Duration `mapstructure:"max-timestamp-diff"`
	// VerifyCredentials enables server-side credential verification against AWS STS.
	// When false, the server only validates the signature structure and format
	// but does not verify the credentials are valid with AWS.
	// Set to true if you want to verify the signing identity with AWS STS GetCallerIdentity.
	VerifyCredentials bool `mapstructure:"verify-credentials"`
	// AWSConfig is optional and only required if VerifyCredentials is true.
	// It's used to make AWS STS API calls to verify the signing identity.
	AWSConfig aws.Config
}
