package hmac

import (
	"fmt"
	"net/http"
)

type HMACClientAuth struct {
	Config HMACAuthClientConfig
	Data   string
}

func (a HMACClientAuth) AddAuth(req *http.Request) error {
	token, err := HMACKey(a.Config.SharedKey, a.Data)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("bearer %v", token))

	return nil
}
