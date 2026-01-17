// Package tls provides utilities for working with TLS certificates and configurations.
package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LoadX509CertFromFile loads an X.509 certificate from a PEM file.
func LoadX509CertFromFile(certPath string) (*x509.Certificate, error) {
	certPathClean := filepath.Clean(certPath)
	certPEM, err := os.ReadFile(certPathClean)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", certPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

// LoadKeyPairFromFiles loads a TLS certificate and key pair from PEM files.
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

// LoadCertPoolFromFile loads a certificate pool from a PEM file.
func LoadCertPoolFromFile(certPoolPath string) (*x509.CertPool, error) {
	certPoolPathClean := filepath.Clean(certPoolPath)
	certPoolPEM, err := os.ReadFile(certPoolPathClean)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(certPoolPEM); !ok {
		return nil, fmt.Errorf("failed to append certificates from PEM file: %s", certPoolPath)
	}

	return certPool, nil
}

func saveBlockToPEMFile(filename string, perm int, blockType string, data []byte) error {
	filenameClean := filepath.Clean(filename)

	f, err := os.OpenFile(filenameClean, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(perm))
	if err != nil {
		return err
	}

	if err = encodeBlock(f, blockType, data); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

func encodeBlock(w io.Writer, blockType string, data []byte) error {
	err := pem.Encode(w, &pem.Block{Type: blockType, Bytes: data})
	if err != nil {
		return err
	}
	return nil
}

func encodeCertificateBlock(w io.Writer, data []byte) error {
	return encodeBlock(w, "CERTIFICATE", data)
}

func encodePrivateKeyBlock(w io.Writer, data crypto.PrivateKey) error {
	privateBytes, err := x509.MarshalPKCS8PrivateKey(data)
	if err != nil {
		return err
	}

	err = encodeBlock(w, "PRIVATE KEY", privateBytes)
	if err != nil {
		return err
	}

	return nil
}

// SaveTLSCertificateToFile saves a tls.Certificate to a file
func SaveTLSCertificateToFile(cert *tls.Certificate, filename string, perm int) error {
	filenameClean := filepath.Clean(filename)

	f, err := os.OpenFile(filenameClean, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(perm))
	if err != nil {
		return err
	}

	err = encodeCertificateBlock(f, cert.Certificate[0])
	if err != nil {
		return err
	}

	err = encodePrivateKeyBlock(f, cert.PrivateKey)
	if err != nil {
		return err
	}

	return f.Close()
}

// SaveTLSCertificateToFiles saves a tls.Certificate to a certificate and key file
func SaveTLSCertificateToFiles(cert *tls.Certificate, certPath, keyPath string) error {
	err := saveBlockToPEMFile(certPath, 0644, "CERTIFICATE", cert.Certificate[0])
	if err != nil {
		return err
	}

	privateBytes, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return err
	}

	return saveBlockToPEMFile(keyPath, 0600, "PRIVATE KEY", privateBytes)
}

// LoadKeyPairAndCertsFromFile From: https://gist.github.com/ukautz/cd118e298bbd8f0a88fc
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
				return nil, fmt.Errorf("failure reading private key from \"%s\": %s", path, err)
			}
		}
		raw = rest
	}

	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("no certificate found in \"%s\"", path)
	} else if cert.PrivateKey == nil {
		return nil, fmt.Errorf("no private key found in \"%s\"", path)
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
			return nil, fmt.Errorf("found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse private key")
}
