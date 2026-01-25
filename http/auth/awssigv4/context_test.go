package awssigv4

import (
	"context"
	"testing"
)

func TestParseARN(t *testing.T) {
	tests := []struct {
		name            string
		arn             string
		expectedAccount string
		expectedType    string
		expectedUserID  string
		expectedRegion  string
		expectedService string
		expectError     bool
	}{
		{
			name:            "IAM user ARN",
			arn:             "arn:aws:iam::123456789012:user/alice",
			expectedAccount: "123456789012",
			expectedType:    "user",
			expectedUserID:  "alice",
			expectedRegion:  "",
			expectedService: "iam",
			expectError:     false,
		},
		{
			name:            "assumed role ARN",
			arn:             "arn:aws:sts::123456789012:assumed-role/MyRole/session-name",
			expectedAccount: "123456789012",
			expectedType:    "assumed-role",
			expectedUserID:  "MyRole/session-name",
			expectedRegion:  "",
			expectedService: "sts",
			expectError:     false,
		},
		{
			name:            "federated user ARN",
			arn:             "arn:aws:sts::123456789012:federated-user/bob",
			expectedAccount: "123456789012",
			expectedType:    "federated-user",
			expectedUserID:  "bob",
			expectedRegion:  "",
			expectedService: "sts",
			expectError:     false,
		},
		{
			name:            "EC2 instance profile ARN",
			arn:             "arn:aws:sts::123456789012:assumed-role/EC2InstanceRole/i-1234567890abcdef0",
			expectedAccount: "123456789012",
			expectedType:    "assumed-role",
			expectedUserID:  "EC2InstanceRole/i-1234567890abcdef0",
			expectedRegion:  "",
			expectedService: "sts",
			expectError:     false,
		},
		{
			name:            "S3 resource ARN with region",
			arn:             "arn:aws:s3:us-west-2:123456789012:bucket/my-bucket",
			expectedAccount: "123456789012",
			expectedType:    "bucket",
			expectedUserID:  "my-bucket",
			expectedRegion:  "us-west-2",
			expectedService: "s3",
			expectError:     false,
		},
		{
			name:        "invalid ARN - too few parts",
			arn:         "arn:aws:iam",
			expectError: true,
		},
		{
			name:        "invalid ARN - doesn't start with arn",
			arn:         "notarn:aws:iam::123456789012:user/alice",
			expectError: true,
		},
		{
			name:        "empty ARN",
			arn:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal, err := ParseARN(tt.arn)

			if tt.expectError {
				if err == nil {
					t.Error("ParseARN() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseARN() unexpected error: %v", err)
				return
			}

			if principal.AccountID != tt.expectedAccount {
				t.Errorf("AccountID = %s, want %s", principal.AccountID, tt.expectedAccount)
			}
			if principal.Type != tt.expectedType {
				t.Errorf("Type = %s, want %s", principal.Type, tt.expectedType)
			}
			if principal.UserID != tt.expectedUserID {
				t.Errorf("UserID = %s, want %s", principal.UserID, tt.expectedUserID)
			}
			if principal.Region != tt.expectedRegion {
				t.Errorf("Region = %s, want %s", principal.Region, tt.expectedRegion)
			}
			if principal.Service != tt.expectedService {
				t.Errorf("Service = %s, want %s", principal.Service, tt.expectedService)
			}
			if principal.ARN != tt.arn {
				t.Errorf("ARN = %s, want %s", principal.ARN, tt.arn)
			}
		})
	}
}

func TestAWSPrincipal_String(t *testing.T) {
	tests := []struct {
		name      string
		principal *AWSPrincipal
		expected  string
	}{
		{
			name: "with ARN",
			principal: &AWSPrincipal{
				ARN:         "arn:aws:iam::123456789012:user/alice",
				AccessKeyID: "AKIA-TEST-FAKE-KEY",
			},
			expected: "arn:aws:iam::123456789012:user/alice",
		},
		{
			name: "without ARN",
			principal: &AWSPrincipal{
				AccessKeyID: "AKIA-TEST-FAKE-KEY",
			},
			expected: "AKIA-TEST-FAKE-KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.principal.String()
			if result != tt.expected {
				t.Errorf("String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestAWSPrincipal_IsAssumedRole(t *testing.T) {
	tests := []struct {
		name      string
		principal *AWSPrincipal
		expected  bool
	}{
		{
			name:      "assumed role",
			principal: &AWSPrincipal{Type: "assumed-role"},
			expected:  true,
		},
		{
			name:      "user",
			principal: &AWSPrincipal{Type: "user"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.principal.IsAssumedRole()
			if result != tt.expected {
				t.Errorf("IsAssumedRole() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAWSPrincipal_IsUser(t *testing.T) {
	tests := []struct {
		name      string
		principal *AWSPrincipal
		expected  bool
	}{
		{
			name:      "user",
			principal: &AWSPrincipal{Type: "user"},
			expected:  true,
		},
		{
			name:      "assumed role",
			principal: &AWSPrincipal{Type: "assumed-role"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.principal.IsUser()
			if result != tt.expected {
				t.Errorf("IsUser() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAWSPrincipal_IsFederatedUser(t *testing.T) {
	tests := []struct {
		name      string
		principal *AWSPrincipal
		expected  bool
	}{
		{
			name:      "federated user",
			principal: &AWSPrincipal{Type: "federated-user"},
			expected:  true,
		},
		{
			name:      "user",
			principal: &AWSPrincipal{Type: "user"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.principal.IsFederatedUser()
			if result != tt.expected {
				t.Errorf("IsFederatedUser() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContextWithAWSPrincipal(t *testing.T) {
	principal := &AWSPrincipal{
		AccessKeyID: "AKIA-TEST-FAKE-KEY",
		ARN:         "arn:aws:iam::123456789012:user/alice",
		AccountID:   "123456789012",
		Type:        "user",
		UserID:      "alice",
	}

	ctx := context.Background()
	ctx = NewContextWithAWSPrincipal(ctx, principal)

	retrievedPrincipal := AWSPrincipalFromContext(ctx)
	if retrievedPrincipal == nil {
		t.Fatal("AWSPrincipalFromContext() returned nil")
	}

	if retrievedPrincipal.AccessKeyID != principal.AccessKeyID {
		t.Errorf("AccessKeyID = %s, want %s", retrievedPrincipal.AccessKeyID, principal.AccessKeyID)
	}
	if retrievedPrincipal.ARN != principal.ARN {
		t.Errorf("ARN = %s, want %s", retrievedPrincipal.ARN, principal.ARN)
	}
	if retrievedPrincipal.AccountID != principal.AccountID {
		t.Errorf("AccountID = %s, want %s", retrievedPrincipal.AccountID, principal.AccountID)
	}
}

func TestContextWithoutAWSPrincipal(t *testing.T) {
	ctx := context.Background()
	retrievedPrincipal := AWSPrincipalFromContext(ctx)
	if retrievedPrincipal != nil {
		t.Error("AWSPrincipalFromContext() should return nil for context without principal")
	}
}
