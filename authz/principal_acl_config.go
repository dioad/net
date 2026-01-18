package authz

// PrincipalACLConfig describes the configuration for principal-based access control.
type PrincipalACLConfig struct {
	AllowList []string `mapstructure:"allow-list"`
	DenyList  []string `mapstructure:"deny-list"`
}
