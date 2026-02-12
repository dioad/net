package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestNewTokenSource(t *testing.T) {
	ts := NewTokenSource(
		WithAudience("my-audience"),
		WithSigningAlgorithm("RS256"),
	).(*tokenSource)
	assert.NotNil(t, ts)
	assert.Equal(t, "my-audience", ts.audience)
	assert.Equal(t, "RS256", ts.signingAlgorithm)
}

func TestWithAWSConfig(t *testing.T) {
	cfg := aws.Config{Region: "us-east-1"}
	ts := NewTokenSource(WithAWSConfig(cfg)).(*tokenSource)
	assert.NotNil(t, ts.awsConfig)
	assert.Equal(t, "us-east-1", ts.awsConfig.Region)
}
