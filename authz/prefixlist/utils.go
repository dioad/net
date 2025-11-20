package prefixlist

import (
	"fmt"
	"net"
)

// parseCIDRs parses a list of CIDR strings into IPNet objects
func parseCIDRs(cidrs []string) ([]*net.IPNet, error) {
	var result []*net.IPNet
	seen := make(map[string]bool)

	for _, cidr := range cidrs {
		// Skip duplicates
		if seen[cidr] {
			continue
		}
		seen[cidr] = true

		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		result = append(result, ipNet)
	}

	return result, nil
}
