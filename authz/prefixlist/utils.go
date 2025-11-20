package prefixlist

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

// fetchURL fetches content from a URL with timeout and returns the response body
func fetchURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
