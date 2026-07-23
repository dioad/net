package tls

import (
	"crypto/tls"
	"slices"
	"testing"
)

func TestNewAutocertTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    ACMEConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    ACMEConfig{},
			want: nil,
		},
		{
			name: "all",
			c: ACMEConfig{
				Domains:        []string{"example.com"},
				CacheDirectory: t.TempDir(),
				Email:          "asdf@asdf.uk",
				DirectoryURL:   "https://example.com",
			},
			want: &tls.Config{
				NextProtos: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newAutocertTLSConfig(tt.c)
			if err != nil {
				t.Errorf("newAutocertTLSConfig() error = %v", err)
				return
			}
			if tt.want == nil && got != nil {
				t.Errorf("newAutocertTLSConfig() = %v, want %v", got, tt.want)
			} else if got != nil {
				if got.GetCertificate == nil {
					t.Errorf("newAutocertTLSConfig() GetCertificate is nil")
				}

				if !slices.Contains(got.NextProtos, "acme-tls/1") {
					t.Errorf("newAutocertTLSConfig() NextProtos = %v, should contain [acme-tls/1]", got.NextProtos)
				}
			}

		})
	}
}
