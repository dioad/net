package basic

type BasicAuthMap map[string]BasicAuthPair

func (m BasicAuthMap) UserExists(user string) bool {
	_, ok := m[user]
	return ok
}

func (m BasicAuthMap) Authenticate(user, password string) (bool, error) {
	if m.UserExists(user) {
		return m[user].VerifyPassword(password)
	}
	return false, nil
}

// if user already exists it will over ride it
func (m BasicAuthMap) AddUserWithPlainPassword(user, password string) {
	authPair, _ := NewBasicAuthPairWithPlainPassword(user, password)
	m[user] = authPair
}

// if user already exists it will over ride it
func (m BasicAuthMap) AddUserWithHashedPassword(user, hashedPassword string) {
	m[user] = BasicAuthPair{User: user, HashedPassword: hashedPassword}
}

