package auth

// need something to deserialize and append details to http.Request
type AuthenticationClientConfig struct {
	BasicAuthConfig  BasicAuthClientConfig  `mapstructure:"basic"`
	GitHubAuthConfig GitHubAuthClientConfig `mapstructure:"github"`
	HMACAuthConfig   HMACAuthClientConfig   `mapstructure:"hmac"`
}

type AuthenticationServerConfig struct {
	BasicAuthConfig  BasicAuthServerConfig  `mapstructure:"basic"`
	GitHubAuthConfig GitHubAuthServerConfig `mapstructure:"github"`
	HMACAuthConfig   HMACAuthServerConfig   `mapstructure:"hmac"`
}
