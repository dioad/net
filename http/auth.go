package http

import (
	"bufio"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type BasicAuthPair struct {
	User           string
	HashedPassword string
}

func (p BasicAuthPair) VerifyPassword(password string) (bool, error) {
	byteHash := []byte(p.HashedPassword)
	err := bcrypt.CompareHashAndPassword(byteHash, []byte(password))
	if err != nil {
		return false, err
	}

	return true, nil
}

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

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func NewBasicAuthPairWithPlainPassword(user, password string) (BasicAuthPair, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return BasicAuthPair{}, err
	}

	return BasicAuthPair{User: user, HashedPassword: hashedPassword}, nil
}

func LoadBasicAuthFromFile(filePath string) BasicAuthMap {
	return nil
}

func LoadBasicAuthFromReader(reader io.Reader) BasicAuthMap {
	scanner := bufio.NewScanner(reader)

	return LoadBasicAuthFromScanner(scanner)
}

func LoadBasicAuthFromScanner(scanner *bufio.Scanner) BasicAuthMap {
	userMap := make(BasicAuthMap)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		userMap.AddUserWithHashedPassword(parts[0], parts[1])
	}
	return userMap
}
