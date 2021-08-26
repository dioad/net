package tls

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"path/filepath"
)

func LoadKeyPairFromFiles(certPath, keyPath string) (*tls.Certificate, error) {
	certPathClean := filepath.Clean(certPath)
	certPEM, err := ioutil.ReadFile(certPathClean)
	if err != nil {
		return nil, err
	}
	keyPathClean := filepath.Clean(keyPath)
	keyPEM, err := ioutil.ReadFile(keyPathClean)
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
	certPoolPathClean := filepath.Clean(certPoolPath)
	certPoolPEM, err := ioutil.ReadFile(certPoolPathClean)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(certPoolPEM)

	return certPool, nil
}
