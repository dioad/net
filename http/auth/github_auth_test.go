package auth

import (
	"testing"
)

func TestGitHubAuthenticator_AuthenticateToken(t *testing.T) {
	tests := map[string]struct {
		token   string
		userNil bool
		login   string
	}{
		"valid token":   {token: "64b9d04d389defed0c7d8037c164a6f3c8912cd4", userNil: false, login: "patdowney"},
		"invalid token": {token: "somethinegelse", userNil: true},
	}

	authenticator := NewGitHubAuthenticator(GitHubAuthServerConfig{
		GitHubAuthCommonConfig: GitHubAuthCommonConfig{
			ClientID:     "bbf369ec17928529a7e8",
			ClientSecret: "491c7ea4efeff78bb7944fb70381cb9c33aca7a3",
		},
	})

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			user, _ := authenticator.AuthenticateToken(tc.token)
			if tc.userNil && user != nil {
				t.Fatalf("expected nil user, got %v", user.Login)
			}

			if !tc.userNil {
				if user == nil {
					t.Fatalf("did not expect nil user")
				}
				if *user.Login != tc.login {
					t.Fatalf("expected %v, got %v", tc.login, user.Login)
				}
			}
		})
	}
}

//IsUserAuthorised
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
