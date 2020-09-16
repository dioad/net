package tls

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

func LoadKeyPairFromFiles(certPath, keyPath string) (*tls.Certificate, error) {
	certPEM, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	keyPEM, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func LoadCertPoolFromFile(certPoolPath string) (*x509.CertPool, error) {
	certPoolPEM, err := ioutil.ReadFile(certPoolPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(certPoolPEM)

	return certPool, nil
}
