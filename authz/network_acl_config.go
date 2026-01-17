package authz

// NetworkACLConfig describes the configuration for network-based access control.
type NetworkACLConfig struct {
	AllowedNets    []string `mapstructure:"allow"`
	DeniedNets     []string `mapstructure:"deny"`
	AllowByDefault bool     `mapstructure:"allow-by-default"`
}
