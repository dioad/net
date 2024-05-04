package tls

import (
	"crypto/tls"
	"slices"
	"testing"
)

func TestNewClientTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    ClientConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    ClientConfig{},
			want: nil,
		},
		{
			name: "with certificate and key",
			c:    ClientConfig{Certificate: "certificate", Key: "key"},
			want: &tls.Config{
				MinVersion: tls.VersionTLS12,
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{},
						PrivateKey:  nil,
					},
				},
				InsecureSkipVerify: false,
			},
		},
		{
			name: "with root CA file",
			c:    ClientConfig{RootCAFile: "root-ca-file"},
			want: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				RootCAs:            nil,
				InsecureSkipVerify: false,
			},
		},
		{
			name: "with insecure skip verify",
			c:    ClientConfig{InsecureSkipVerify: true},
			want: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClientTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewClientTLSConfig() error = %v", err)
			}
			if tt.want == nil && got == nil {
				return
			}

			// ignored for now until we have a way to test the generated certificate

		})
	}
}

func TestNewLocalTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    LocalConfig
		want *tls.Config
	}{
		{
			name: "single pem file",
			c:    LocalConfig{SinglePEMFile: "single-pem-file"},
			want: &tls.Config{Certificates: nil},
		},
		{
			name: "empty",
			c:    LocalConfig{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewLocalTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewLocalTLSConfig() error = %v", err)
			}
			if tt.want == nil && got == nil {
				return
			}

			// ignored for now until we have a way to test the generated certificate
		})
	}
}

func TestNewServerTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    ServerConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    ServerConfig{},
			want: nil,
		},
		{
			name: "with client CA file",
			c:    ServerConfig{ClientCAFile: "client-ca-file"},
			// want: &tls.Config{
			// 	MinVersion: tls.VersionTLS12,
			// 	ClientAuth: tls.NoClientCert,
			// 	ClientCAs:  nil,
			// },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewServerTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewServerTLSConfig() error = %v", err)
			} else {
				if tt.want == nil && got == nil {
					return
				}
				if got != nil {
					if !slices.Contains(got.NextProtos, "h2") {
						t.Errorf("NewAutocertTLSConfig() NextProtos = %v, should contain [h2]", got.NextProtos)
					}

					// ignored for now until we have a way to test the generated certificate
				}
			}
		})
	}
}

func TestNewSelfSignedTLSConfig(t *testing.T) {
	tests := []struct {
		name string
		c    SelfSignedConfig
		want *tls.Config
	}{
		{
			name: "empty",
			c:    SelfSignedConfig{},
			want: nil, // ignored for now until we have a way to test the generated certificate
		},
		{
			name: "non-empty",
			c: SelfSignedConfig{
				Alias:    "host",
				Duration: "1h",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSelfSignedTLSConfig(tt.c)
			if err != nil {
				t.Errorf("NewSelfSignedTLSConfig() error = %v", err)
			} else {
				if tt.want == nil && got == nil {
					return
				}

				// ignored for now until we have a way to test the generated certificate

			}
		})
	}
}
