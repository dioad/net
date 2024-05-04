package authz

type PrincipalACLConfig struct {
	AllowList []string `mapstructure:"allow-list"`
	DenyList  []string `mapstructure:"deny-list"`
}
