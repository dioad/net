package util

import "testing"

// IsUserAuthorised
func TestIsUserAuthorised(t *testing.T) {
	tests := map[string]struct {
		user      string
		allowList []string
		denyList  []string
		expected  bool
	}{
		"no acl":            {user: "userA", allowList: nil, denyList: nil, expected: true},
		"empty allow":       {user: "userA", allowList: []string{}, denyList: nil, expected: true},
		"empty deny":        {user: "userA", allowList: nil, denyList: []string{}, expected: true},
		"user in allow":     {user: "userA", allowList: []string{"userA"}, denyList: nil, expected: true},
		"user not in allow": {user: "userA", allowList: []string{"userB"}, denyList: nil, expected: false},
		"user in deny":      {user: "userA", allowList: []string{"userB"}, denyList: []string{"userA"}, expected: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IsUserAuthorised(tc.user, tc.allowList, tc.denyList)
			if got != tc.expected {
				t.Fatalf("expected %v , got %v", tc.expected, got)
			}
		})
	}
}
