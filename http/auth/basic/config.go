package basic

import "reflect"

var (
	EmptyBasicAuthClientConfig = BasicAuthClientConfig{}
	EmptyBasicAuthServerConfig = BasicAuthServerConfig{}
)

type BasicAuthClientConfig struct {
	// https://everything.curl.dev/usingcurl/netrc
	//
	// machine connect.lab.dioad.net
	// login blah
	// password blah
	NetRCFile string `mapstructure:"netrc-file"`
	User      string `mapstructure:"user"`
	Password  string `mapstructure:"password"`
}

func (c BasicAuthClientConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyBasicAuthClientConfig)
}

type BasicAuthServerConfig struct {
	AllowInsecureHTTP bool     `mapstructure:"allow-insecure-http"`
	HTPasswdFile      string   `mapstructure:"htpasswd-file"`
	Users             []string `mapstructure:"users"`
}

func (c BasicAuthServerConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyBasicAuthServerConfig)
}
