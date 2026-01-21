package prefixlist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
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

// FetchTextLines is a fetch function that retrieves plain text lines from an HTTP endpoint.
// It returns a slice of non-empty, non-comment lines. Lines starting with '#' are
// treated as comments and ignored.
func FetchTextLines(ctx context.Context, url string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return parseTextLines(resp.Body)
}

// parseTextLines parses plain text list of items (one per line)
func parseTextLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}
