package tls

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

type SelfSignedConfig struct {
	Subject     pkix.Name
	DNSNames    []string
	IPAddresses []net.IP
	NotBefore   time.Time
	NotAfter    time.Time
	IsCA        bool
}

func CreateAndSaveSelfSignedKeyPair(config SelfSignedConfig, certPath, keyPath string) (*tls.Certificate, *x509.CertPool, error) {
	cert, certPool, err := CreateSelfSignedKeyPair(config)
	if err != nil {
		panic(err)
	}

	err = SaveTLSCertificateToFiles(cert, certPath, keyPath)
	if err != nil {
		panic(err)
	}

	return cert, certPool, err
}

// pulled from inet.af/tcpproxy
func CreateSelfSignedKeyPair(config SelfSignedConfig) (*tls.Certificate, *x509.CertPool, error) {
	pkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               config.Subject,
		NotBefore:             config.NotBefore,
		NotAfter:              config.NotAfter,
		IsCA:                  config.IsCA,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              config.DNSNames[:],
		IPAddresses:           config.IPAddresses[:],
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, pkey.Public(), pkey)
	if err != nil {
		return nil, nil, err
	}

	var cert, key bytes.Buffer
	err = pem.Encode(&cert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, nil, err
	}
	err = pem.Encode(&key, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pkey)})
	if err != nil {
		return nil, nil, err
	}

	tlscert, err := tls.X509KeyPair(cert.Bytes(), key.Bytes())
	if err != nil {
		return nil, nil, err
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(cert.Bytes()) {
		return nil, nil, fmt.Errorf("failed to add cert %q to pool", config.DNSNames)
	}

	return &tlscert, pool, nil
}
