package basic

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
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

type ClientAuth struct {
	Config   ClientConfig
	user     string
	password string
}

func (a ClientAuth) HTTPClient() (*http.Client, error) {
	return &http.Client{
		Transport: &RoundTripper{
			Username: a.user,
			Password: a.password,
		},
	}, nil
}

func (a ClientAuth) AddAuth(req *http.Request) error {
	if a.user == "" {
		if a.Config.User != "" {
			a.user = a.Config.User
			a.password = a.Config.Password
		} else {
			host := req.URL.Hostname()

			netrcOnce.Do(readNetrc)
			for _, l := range netrc {
				if l.machine == host {
					a.user = l.login
					a.password = l.password
				}
			}
		}
	}
	req.SetBasicAuth(a.user, a.password)
	return nil
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

func LoadBasicAuthFromFile(filePath string) (AuthMap, error) {
	expFilePath, err := homedir.Expand(filePath)
	if err != nil {
		return nil, err
	}
	filePathClean := path.Clean(expFilePath)
	f, err := os.Open(filePathClean)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Mode() != 0600 {
		return nil, errors.New(fmt.Sprintf("file mode is not 0600, it is %v %s", stat.Mode(), filePathClean))
	}
	authMap := LoadBasicAuthFromReader(f)

	err = f.Close()
	if err != nil {
		return nil, nil
	}

	return authMap, nil
}

func LoadBasicAuthFromReader(reader io.Reader) AuthMap {
	scanner := bufio.NewScanner(reader)

	return LoadBasicAuthFromScanner(scanner)
}

func LoadBasicAuthFromScanner(scanner *bufio.Scanner) AuthMap {
	userMap := make(AuthMap)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		userMap.AddUserWithHashedPassword(parts[0], parts[1])
	}
	return userMap
}
