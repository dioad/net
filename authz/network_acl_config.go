package authz

import "reflect"

var (
	EmptyGitHubAuthClientConfig = NetworkACLConfig{}
)

type NetworkACLConfig struct {
	AllowedNets    []string `mapstructure:"allow"`
	DeniedNets     []string `mapstructure:"deny"`
	AllowByDefault bool     `mapstructure:"allow-by-default"`
}

func (n NetworkACLConfig) IsEmpty() bool {
	return reflect.DeepEqual(n, EmptyGitHubAuthClientConfig)
}
