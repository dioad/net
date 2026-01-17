// Package dkim provides utilities for working with DKIM (DomainKeys Identified Mail) records.
package dkim

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/exp/maps"
)

// KeyType represents the DKIM key type (e.g., "rsa").
type KeyType string

// k=
const (
	KeyTypeRSA KeyType = "rsa"
)

// Record represents a DKIM DNS record.
type Record struct {
	Version   string `mapstructure:"version"`
	KeyType   string `mapstructure:"key-type"`
	PublicKey string `mapstructure:"public-key"`
}

// RecordPrefix returns the default DNS prefix for DKIM records.
func (r *Record) RecordPrefix() string {
	return "default._domainkey."
}

// RecordType returns the DNS record type for DKIM records.
func (r *Record) RecordType() string {
	return "TXT"
}

// RecordValue returns the formatted DKIM record value.
func (r *Record) RecordValue() string {
	return fmt.Sprintf("\\\"%v\\\"", r.String())
}

// String returns the string representation of the DKIM record.
func (r *Record) String() string {
	parts := make([]string, 0)

	if r.Version == "" {
		parts = append(parts, "v=DKIM1")
	} else {
		parts = append(parts, fmt.Sprintf("v=%s", r.Version))
	}

	if r.KeyType == "" {
		parts = append(parts, fmt.Sprintf("k=%s", KeyTypeRSA))
	} else {
		parts = append(parts, fmt.Sprintf("k=%s", r.KeyType))
	}

	parts = append(parts, fmt.Sprintf("p=%s", r.PublicKey))

	return strings.Join(parts, "; ")
}

// FromRecordFile parses a DKIM record from a file.
func FromRecordFile(r io.Reader) (*Record, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	m, err := ParseParams(string(b))
	if err != nil {
		return nil, err
	}
	return &Record{
		Version:   "DKIM1",
		KeyType:   m["k"],
		PublicKey: m["p"],
	}, nil
}

func ParseParams(s string) (map[string]string, error) {
	validParams := map[string]bool{
		"v": true,
		"k": true,
		"p": true,
	}

	return parseParams(validParams, s)
}

// ParseParams borrowed from https://github.com/emersion/go-msgauth/dkim/header.go
func parseParams(validParams map[string]bool, s string) (map[string]string, error) {

	pairs := strings.Split(s, ";")
	params := make(map[string]string)
	for _, s := range pairs {
		k, v, _ := strings.Cut(s, "=")

		strippedK := strings.TrimSpace(k)
		if p, ok := validParams[strippedK]; ok && p {
			params[strings.TrimSpace(k)] = strings.TrimSpace(v)
		} else {
			return nil, fmt.Errorf("invalid parameter %v not in %v", strippedK, strings.Join(maps.Keys(validParams), ","))
		}
	}
	return params, nil
}
