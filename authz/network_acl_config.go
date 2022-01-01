package authz

type NetworkACLConfig struct {
	AllowedNets    []string `mapstructure:"allow"`
	DeniedNets     []string `mapstructure:"deny"`
	AllowByDefault bool     `mapstructure:"allow-by-default"`
}
