package tls

import (
	"crypto/tls"
	"io/ioutil"
)

func LoadKeyPairFromFiles(certPath, keyPath string) (*tls.Certificate, error) {
	certPem, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	keyPem, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
