package jwt

import (
	"github.com/mitchellh/mapstructure"
)

// ServerConfig contains configuration for a JWT authentication server.
type ServerConfig struct {
	CookieName string `mapstructure:"cookie-name"`
}

// FromMap creates a ServerConfig from a map.
func FromMap(m map[string]interface{}) ServerConfig {
	var c ServerConfig
	_ = mapstructure.Decode(m, &c)
	return c
}
