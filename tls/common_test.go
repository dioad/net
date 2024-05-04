package tls

import (
	"crypto/tls"
	"testing"
)

func Test_convertClientAuthType(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		want     tls.ClientAuthType
	}{
		{
			name:     "RequestClientCert",
			authType: "RequestClientCert",
			want:     tls.RequestClientCert,
		},
		{
			name:     "RequireAnyClientCert",
			authType: "RequireAnyClientCert",
			want:     tls.RequireAnyClientCert,
		},
		{
			name:     "VerifyClientCertIfGiven",
			authType: "VerifyClientCertIfGiven",
			want:     tls.VerifyClientCertIfGiven,
		},
		{
			name:     "RequireAndVerifyClientCert",
			authType: "RequireAndVerifyClientCert",
			want:     tls.RequireAndVerifyClientCert,
		},
		{
			name:     "NoClientCert",
			authType: "NoClientCert",
			want:     tls.NoClientCert,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertClientAuthType(tt.authType); got != tt.want {
				t.Errorf("convertClientAuthType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertTLSVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    uint16
	}{
		{
			name:    "TLS13",
			version: "TLS13",
			want:    tls.VersionTLS13,
		},
		{
			name:    "TLS12",
			version: "TLS12",
			want:    tls.VersionTLS12,
		},
		{
			name:    "TLS11",
			version: "TLS11",
			want:    tls.VersionTLS11,
		},
		{
			name:    "TLS10",
			version: "TLS10",
			want:    tls.VersionTLS10,
		},
		{
			name:    "default",
			version: "default",
			want:    tls.VersionTLS12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertTLSVersion(tt.version); got != tt.want {
				t.Errorf("convertTLSVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
