package authz

// IsPrincipalAuthorised checks if a user is authorised based on allow and deny lists.
//
// allowByDefault controls the outcome when allowList is empty (no allow rule
// configured): true authorises the user (subject to denyList), false denies
// by default. This mirrors NetworkACL.AllowByDefault -- callers should not
// assume an empty allowList means "allow everyone" without setting this
// explicitly.
func IsPrincipalAuthorised(user string, allowList []string, denyList []string, allowByDefault bool) bool {
	principalAuthorised := allowByDefault
	if len(allowList) > 0 {
		principalAuthorised = false
		for _, p := range allowList {
			if p == user {
				principalAuthorised = true
			}
		}
	}

	if len(denyList) > 0 {
		for _, p := range denyList {
			if p == user {
				principalAuthorised = false
			}
		}
	}

	return principalAuthorised
}
