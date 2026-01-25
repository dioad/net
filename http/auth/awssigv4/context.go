package awssigv4

import (
	"context"
	"fmt"
	"strings"
)

// AWSPrincipal represents an AWS identity extracted from a SigV4 signature.
type AWSPrincipal struct {
	// AccessKeyID is the AWS access key ID used to sign the request
	AccessKeyID string
	// ARN is the full Amazon Resource Name of the identity (if available from STS verification)
	ARN string
	// AccountID is the AWS account ID
	AccountID string
	// UserID is the unique identifier for the user/role
	UserID string
	// Type indicates if this is a "user", "assumed-role", "federated-user", etc.
	Type string
	// Region is the AWS region from the credential scope
	Region string
	// Service is the AWS service from the credential scope
	Service string
}

// ParseARN parses an AWS ARN and extracts components.
// ARN format: arn:partition:service:region:account-id:resource-type/resource-name
// or: arn:partition:service:region:account-id:resource-type:resource-name
func ParseARN(arn string) (*AWSPrincipal, error) {
	if arn == "" {
		return nil, fmt.Errorf("empty ARN")
	}

	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid ARN format: %s", arn)
	}

	if parts[0] != "arn" {
		return nil, fmt.Errorf("ARN must start with 'arn:'")
	}

	principal := &AWSPrincipal{
		ARN:       arn,
		AccountID: parts[4],
		Service:   parts[2],
		Region:    parts[3],
	}

	// Parse the resource part to determine type
	resource := strings.Join(parts[5:], ":")
	if strings.Contains(resource, "/") {
		resourceParts := strings.SplitN(resource, "/", 2)
		principal.Type = resourceParts[0]
		if len(resourceParts) > 1 {
			principal.UserID = resourceParts[1]
		}
	} else if strings.Contains(resource, ":") {
		resourceParts := strings.SplitN(resource, ":", 2)
		principal.Type = resourceParts[0]
		if len(resourceParts) > 1 {
			principal.UserID = resourceParts[1]
		}
	} else {
		principal.Type = resource
	}

	return principal, nil
}

// String returns a string representation of the principal.
func (p *AWSPrincipal) String() string {
	if p.ARN != "" {
		return p.ARN
	}
	return p.AccessKeyID
}

// IsAssumedRole returns true if this principal represents an assumed role.
func (p *AWSPrincipal) IsAssumedRole() bool {
	return p.Type == "assumed-role"
}

// IsUser returns true if this principal represents an IAM user.
func (p *AWSPrincipal) IsUser() bool {
	return p.Type == "user"
}

// IsFederatedUser returns true if this principal represents a federated user.
func (p *AWSPrincipal) IsFederatedUser() bool {
	return p.Type == "federated-user"
}

type awsPrincipalContext struct{}

// NewContextWithAWSPrincipal returns a new context with the provided AWS principal.
func NewContextWithAWSPrincipal(ctx context.Context, principal *AWSPrincipal) context.Context {
	return context.WithValue(ctx, awsPrincipalContext{}, principal)
}

// AWSPrincipalFromContext returns the AWS principal from the provided context.
// It returns nil if no principal is found.
func AWSPrincipalFromContext(ctx context.Context) *AWSPrincipal {
	val := ctx.Value(awsPrincipalContext{})
	if val != nil {
		return val.(*AWSPrincipal)
	}
	return nil
}
