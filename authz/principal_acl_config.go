package authz

// PrincipalACLConfig describes the configuration for principal-based access control.
type PrincipalACLConfig struct {
	AllowList []string `mapstructure:"allow-list"`
	DenyList  []string `mapstructure:"deny-list"`
	// AllowByDefault controls the outcome when AllowList is empty (no allow
	// rule configured). Mirrors NetworkACLConfig.AllowByDefault; defaults to
	// false (deny) so an unconfigured ACL fails closed.
	AllowByDefault bool `mapstructure:"allow-by-default"`
}
