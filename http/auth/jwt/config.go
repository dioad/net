package jwt

import (
	"github.com/go-viper/mapstructure/v2"
)

// ServerConfig contains configuration for a JWT authentication server.
type ServerConfig struct {
	CookieName string `mapstructure:"cookie-name"`
}

// FromMap creates a ServerConfig from a map.
func FromMap(m map[string]any) ServerConfig {
	var c ServerConfig
	_ = mapstructure.Decode(m, &c)
	return c
}
