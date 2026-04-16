package authz

// NetworkACLConfig describes the configuration for network-based access control.
type NetworkACLConfig struct {
	AllowedNets    []string `json:"allow,omitzero" mapstructure:"allow"`
	DeniedNets     []string `json:"deny,omitzero" mapstructure:"deny"`
	AllowByDefault bool     `json:"allow_by_default" mapstructure:"allow-by-default"`
}
