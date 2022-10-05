package authz

import "reflect"

var (
	EmptyPrincipalACLConfig = PrincipalACLConfig{}
)

type PrincipalACLConfig struct {
	AllowList []string `mapstructure:"allow-list"`
	DenyList  []string `mapstructure:"deny-list"`
}

func (c PrincipalACLConfig) IsEmpty() bool {
	return reflect.DeepEqual(c, EmptyPrincipalACLConfig)
}
