package http

import (
	"time"

	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

// DefaultCORSMaxAge is how long browsers may cache a preflight response when
// CORSConfig does not say.
const DefaultCORSMaxAge = 24 * time.Hour

// CORSConfig is the cross-origin policy of an API, in the form services take
// it in their configuration. Options turns it into rs/cors options and
// Middleware into a middleware.
//
// Nothing is allowed unless an origin is listed or AllowLocalhost is set; an
// empty config is closed, unlike rs/cors, which allows every origin when it is
// given none.
type CORSConfig struct {
	// AllowedOrigins are the origins that may call the API. "*" allows every
	// origin.
	AllowedOrigins []string `desc:"origins that may call the API" mapstructure:"allowed-origins"`

	// AllowLocalhost also allows localhost origins. It is for a local UI that
	// calls the API and for development. With AllowCredentials it lets any page
	// on the user's machine make credentialed requests, so enable it only
	// where that is meant.
	AllowLocalhost bool `desc:"also allow localhost origins" mapstructure:"allow-localhost"`

	// AllowedMethods defaults to the methods rs/cors allows, GET, POST and HEAD.
	AllowedMethods []string `desc:"methods allowed on cross-origin requests" mapstructure:"allowed-methods"`

	// AllowedHeaders defaults to Content-Type and Authorization.
	AllowedHeaders []string `desc:"request headers allowed on cross-origin requests" mapstructure:"allowed-headers"`

	// ExposedHeaders are the response headers a browser may read.
	ExposedHeaders []string `desc:"response headers exposed to browsers" mapstructure:"exposed-headers"`

	// AllowCredentials lets browsers send cookies and authorisation headers.
	AllowCredentials bool `desc:"allow credentialed cross-origin requests" mapstructure:"allow-credentials"`

	// AllowPrivateNetwork answers Private Network Access preflights, for a
	// public page calling an API on a private network.
	AllowPrivateNetwork bool `desc:"allow private network access preflights" mapstructure:"allow-private-network"`

	// MaxAgeSeconds is how long browsers may cache a preflight response. It
	// defaults to DefaultCORSMaxAge.
	MaxAgeSeconds int `desc:"seconds a preflight response may be cached" mapstructure:"max-age-seconds"`
}

// Options returns the rs/cors options for c, logging through logger and, with
// debug, logging every decision.
func (c CORSConfig) Options(logger zerolog.Logger, debug bool) cors.Options {
	headers := c.AllowedHeaders
	if len(headers) == 0 {
		headers = []string{"Content-Type", "Authorization"}
	}
	maxAge := c.MaxAgeSeconds
	if maxAge <= 0 {
		maxAge = int(DefaultCORSMaxAge / time.Second)
	}

	return cors.Options{
		AllowOriginFunc:     c.allowOrigin,
		AllowedMethods:      c.AllowedMethods,
		AllowedHeaders:      headers,
		ExposedHeaders:      c.ExposedHeaders,
		AllowCredentials:    c.AllowCredentials,
		AllowPrivateNetwork: c.AllowPrivateNetwork,
		MaxAge:              maxAge,
		Debug:               debug,
		Logger:              &logger,
	}
}

// Middleware returns the CORS middleware for c. See Options.
func (c CORSConfig) Middleware(logger zerolog.Logger, debug bool) Middleware {
	return CORSHandler(c.Options(logger, debug))
}

func (c CORSConfig) allowOrigin(origin string) bool {
	for _, allowed := range c.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return c.AllowLocalhost && CORSAllowLocalhostOrigin(origin)
}
