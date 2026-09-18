package authz

import (
	"testing"
)

// IsUserAuthorised
func TestIsPrincipalAuthorised(t *testing.T) {
	tests := map[string]struct {
		user           string
		allowList      []string
		denyList       []string
		allowByDefault bool
		expected       bool
	}{
		"no acl, deny by default":        {user: "userA", allowList: nil, denyList: nil, allowByDefault: false, expected: false},
		"no acl, allow by default":       {user: "userA", allowList: nil, denyList: nil, allowByDefault: true, expected: true},
		"empty allow, deny by default":   {user: "userA", allowList: []string{}, denyList: nil, allowByDefault: false, expected: false},
		"empty allow, allow by default":  {user: "userA", allowList: []string{}, denyList: nil, allowByDefault: true, expected: true},
		"empty deny, deny by default":    {user: "userA", allowList: nil, denyList: []string{}, allowByDefault: false, expected: false},
		"empty deny, allow by default":   {user: "userA", allowList: nil, denyList: []string{}, allowByDefault: true, expected: true},
		"user in allow":                  {user: "userA", allowList: []string{"userA"}, denyList: nil, allowByDefault: false, expected: true},
		"user not in allow":              {user: "userA", allowList: []string{"userB"}, denyList: nil, allowByDefault: true, expected: false},
		"user in deny":                   {user: "userA", allowList: []string{"userB", "userA"}, denyList: []string{"userA"}, allowByDefault: false, expected: false},
		"user in deny, empty allow list": {user: "userA", allowList: nil, denyList: []string{"userA"}, allowByDefault: true, expected: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IsPrincipalAuthorised(tc.user, tc.allowList, tc.denyList, tc.allowByDefault)
			if got != tc.expected {
				t.Fatalf("expected %v , got %v", tc.expected, got)
			}
		})
	}
}
