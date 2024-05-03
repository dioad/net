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

	"github.com/dioad/generics"
)

func CreateAndSaveSelfSignedKeyPair(config SelfSignedConfig, certPath, keyPath string) (*tls.Certificate, *x509.CertPool, error) {
	cert, certPool, err := CreateSelfSignedKeyPair(config)
	if err != nil {

		return nil, nil, fmt.Errorf("error creating self-signed key pair: %w", err)
	}

	err = SaveTLSCertificateToFiles(cert, certPath, keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error saving cert and key to file: %w", err)
	}

	return cert, certPool, err
}

func convertConfigToX509CertificateTemplate(config SelfSignedConfig) (*x509.Certificate, error) {

	notBefore := time.Now().UTC()

	duration, err := time.ParseDuration(config.Duration)
	if err != nil {
		return nil, err
	}

	notAfter := notBefore.Add(duration)

	ipAddresses, err := generics.Map(func(ip string) (net.IP, error) {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return nil, fmt.Errorf("error parsing ip address: %s", ip)
		}
		return parsedIP, nil
	}, config.SANConfig.IPAddresses)
	if err != nil {
		return nil, fmt.Errorf("error parsing ip addresses: %w", err)
	}

	return &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:            config.Subject.Country,
			Organization:       config.Subject.Organization,
			OrganizationalUnit: config.Subject.OrganizationalUnit,
			Locality:           config.Subject.Locality,
			Province:           config.Subject.Province,
			StreetAddress:      config.Subject.StreetAddress,
			PostalCode:         config.Subject.PostalCode,
			SerialNumber:       config.Subject.SerialNumber,
			CommonName:         config.Subject.CommonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  config.IsCA,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              config.SANConfig.DNSNames,
		IPAddresses:           ipAddresses,
	}, nil
}

// pulled from inet.af/tcpproxy
func CreateSelfSignedKeyPair(config SelfSignedConfig) (*tls.Certificate, *x509.CertPool, error) {
	pkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	template, err := convertConfigToX509CertificateTemplate(config)
	if err != nil {
		return nil, nil, err
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

	tlsCert, err := tls.X509KeyPair(cert.Bytes(), key.Bytes())
	if err != nil {
		return nil, nil, err
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(cert.Bytes()) {
		return nil, nil, fmt.Errorf("failed to add cert %q to pool", config.SANConfig.DNSNames)
	}

	return &tlsCert, pool, nil
}
