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

func (p BasicAuthPair) VerifyPassword(password string) (bool, error) {
	byteHash := []byte(p.HashedPassword)
	err := bcrypt.CompareHashAndPassword(byteHash, []byte(password))
	if err != nil {
		return false, err
	}

	return true, nil
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

func LoadBasicAuthFromReader(reader io.Reader) BasicAuthMap {
	scanner := bufio.NewScanner(reader)

	return LoadBasicAuthFromScanner(scanner)
}

func LoadBasicAuthFromScanner(scanner *bufio.Scanner) BasicAuthMap {
	userMap := make(map[string]BasicAuthPair)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		userMap[parts[0]] = BasicAuthPair{User: parts[0], HashedPassword: parts[1]}
	}
	return userMap
}
