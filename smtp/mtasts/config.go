package mtasts

// Config represents MTA-STS configuration used to create a policy.
type Config struct {
	Mode   Mode     `mapstructure:"mode"`
	MX     []string `mapstructure:"mx"`
	MaxAge uint32   `mapstructure:"max-age"`
}

func PolicyFromConfig(cfg Config) *Policy {
	return &Policy{
		Version: "STSv1",
		Mode:    cfg.Mode,
		MX:      cfg.MX,
		MaxAge:  cfg.MaxAge,
	}
}
