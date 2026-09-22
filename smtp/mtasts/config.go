// Package mtasts provides types for building and serving an MTA-STS policy.
package mtasts

// Config represents MTA-STS configuration used to create a policy.
type Config struct {
	Mode   Mode     `mapstructure:"mode"`
	MX     []string `mapstructure:"mx"`
	MaxAge uint32   `mapstructure:"max-age"`
}

// PolicyFromConfig builds a Policy from cfg, using the fixed "STSv1" version.
func PolicyFromConfig(cfg Config) *Policy {
	return &Policy{
		Version: "STSv1",
		Mode:    cfg.Mode,
		MX:      cfg.MX,
		MaxAge:  cfg.MaxAge,
	}
}
