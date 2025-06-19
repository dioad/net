package tls

import (
	"crypto/tls"
	"slices"
	"testing"
)

func TestNewAutocertTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    AutoCertConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    AutoCertConfig{},
			want: nil,
		},
		{
			name: "all",
			c: AutoCertConfig{
				AllowedHosts:   []string{"example.com"},
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
			got, err := NewAutocertTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewAutocertTLSConfig() error = %v", err)
				return
			} else {
				if tt.want == nil && got != nil {
					t.Errorf("NewAutocertTLSConfig() = %v, want %v", got, tt.want)
				} else if got != nil {
					if got.GetCertificate == nil {
						t.Errorf("NewAutocertTLSConfig() GetCertificate is nil")
					}

					if !slices.Contains(got.NextProtos, "acme-tls/1") {
						t.Errorf("NewAutocertTLSConfig() NextProtos = %v, should contain [acme-tls/1]", got.NextProtos)
					}
				}
			}
		})
	}
}
