package util

func IsUserAuthorised(user string, allowList []string, denyList []string) bool {
	userAuthorised := true
	if allowList != nil && len(allowList) > 0 {
		userAuthorised = false
		for _, p := range allowList {
			if p == user {
				userAuthorised = true
			}
		}
	}

	if len(denyList) > 0 {
		for _, p := range denyList {
			if p == user {
				userAuthorised = false
			}
		}
	}

	return userAuthorised
}
