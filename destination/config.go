package destination

import (
	"net/netip"

	"github.com/dioad/config/configcheck"
)

// Config is a Policy in the form services take it in their configuration.
type Config struct {
	// AllowPrefixes are CIDR ranges allowed in addition to public addresses.
	AllowPrefixes []string `desc:"CIDR ranges allowed in addition to public addresses" mapstructure:"allow"`

	// OnlyPrefixes are the only CIDR ranges allowed; public addresses are not
	// allowed unless listed.
	OnlyPrefixes []string `desc:"the only CIDR ranges allowed, replacing the public-address default" mapstructure:"only"`

	// AllowedPorts restricts destination ports when set.
	AllowedPorts []int `desc:"destination ports allowed, any when empty" mapstructure:"allowed-ports"`

	// AllowCloudMetadata permits cloud metadata endpoints.
	AllowCloudMetadata bool `desc:"allow cloud metadata endpoints" mapstructure:"allow-cloud-metadata"`
}

// Check reports to c a range that is not a CIDR prefix, a port out of range,
// and allow ranges given together with only, which would be ignored. c is
// positioned at the config's own block.
func (cfg Config) Check(c *configcheck.Collector) {
	checkPrefixes(c.Field("allow"), cfg.AllowPrefixes)
	checkPrefixes(c.Field("only"), cfg.OnlyPrefixes)
	if len(cfg.AllowPrefixes) > 0 && len(cfg.OnlyPrefixes) > 0 {
		c.Field("allow").Errorf("has no effect with only set, remove one")
	}
	for i, port := range cfg.AllowedPorts {
		if port < 1 || port > 65535 {
			c.Field("allowed-ports").Index(i).Errorf("%d is not a port", port)
		}
	}
}

func checkPrefixes(c *configcheck.Collector, prefixes []string) {
	for i, p := range prefixes {
		if _, err := netip.ParsePrefix(p); err != nil {
			c.Index(i).Errorf("%q is not a CIDR range: %v", p, err)
		}
	}
}

// Policy returns the Policy for cfg. It returns an error for a range that is
// not a CIDR prefix; Check reports the same.
func (cfg Config) Policy() (Policy, error) {
	allow, err := parsePrefixes(cfg.AllowPrefixes)
	if err != nil {
		return Policy{}, err
	}
	only, err := parsePrefixes(cfg.OnlyPrefixes)
	if err != nil {
		return Policy{}, err
	}

	return Policy{
		AllowPrefixes:      allow,
		OnlyPrefixes:       only,
		AllowedPorts:       cfg.AllowedPorts,
		AllowCloudMetadata: cfg.AllowCloudMetadata,
	}, nil
}

func parsePrefixes(values []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		prefix, err := netip.ParsePrefix(v)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
}
