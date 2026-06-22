package prefixlist

import (
	"fmt"
	"net/netip"
	"strings"
)

// parseCommaSeparated parses comma-separated values into a slice
func parseCommaSeparated(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

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
