package basic

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

type BasicAuthServerConfig struct {
	HTPasswdFile string   `mapstructure:"htpasswd-file"`
	Users        []string `mapstructure:"users"`
}
