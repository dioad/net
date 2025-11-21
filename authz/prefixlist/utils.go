package prefixlist

import (
	"fmt"
	"net/netip"
)

// parseCIDRs parses a list of CIDR strings into netip.Prefix objects
func parseCIDRs(cidrs []string) ([]netip.Prefix, error) {
	var result []netip.Prefix
	seen := make(map[string]bool)

	for _, cidr := range cidrs {
		// Skip duplicates
		if seen[cidr] {
			continue
		}
		seen[cidr] = true

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		result = append(result, prefix)
	}

	return result, nil
}
