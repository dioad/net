package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

func LoadKeyPairFromFiles(certPath, keyPath string) (*tls.Certificate, error) {
	certPathClean := filepath.Clean(certPath)
	certPEM, err := os.ReadFile(certPathClean)
	if err != nil {
		return nil, err
	}
	keyPathClean := filepath.Clean(keyPath)
	keyPEM, err := os.ReadFile(keyPathClean)
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
	certPoolPEM, err := os.ReadFile(certPoolPathClean)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(certPoolPEM)

	return certPool, nil
}

func SaveTLSCertificateToFiles(cert *tls.Certificate, certPath, keyPath string) error {
	certPathClean := filepath.Clean(certPath)
	err := os.WriteFile(certPathClean, cert.Certificate[0], 0644)
	if err != nil {
		return err
	}

	keyPathClean := filepath.Clean(keyPath)
	err = os.WriteFile(keyPathClean, cert.PrivateKey.(*rsa.PrivateKey).D.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}

// From: https://gist.github.com/ukautz/cd118e298bbd8f0a88fc
// LoadKeyPairAndCertsFromFile reads file, divides into key and certificates
func LoadKeyPairAndCertsFromFile(path string) (*tls.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cert tls.Certificate
	for {
		block, rest := pem.Decode(raw)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, block.Bytes)
		} else {
			cert.PrivateKey, err = parsePrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("Failure reading private key from \"%s\": %s", path, err)
			}
		}
		raw = rest
	}

	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("No certificate found in \"%s\"", path)
	} else if cert.PrivateKey == nil {
		return nil, fmt.Errorf("No private key found in \"%s\"", path)
	}

	return &cert, nil
}

func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("Found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("Failed to parse private key")
}
