package hmac

import (
	"fmt"
	"net/http"
)

type ClientAuth struct {
	Config ClientConfig
}

// AddAuth adds the HMAC token to the request as a bearer token
//
// TODO:  This should be refactored to use the request Body to calculate the digest /token
func (a ClientAuth) AddAuth(req *http.Request) error {
	token, err := HMACKey([]byte(a.Config.SharedKey), []byte(a.Config.Data))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %v", token))

	return nil
}
