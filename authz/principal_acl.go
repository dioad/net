package authz

func IsPrincipalAuthorised(user string, allowList []string, denyList []string) bool {
	principalAuthorised := true
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
