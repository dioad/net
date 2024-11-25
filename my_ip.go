package net

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
)

var (
	IPv4ICanHazIP = "http://ipv4.icanhazip.com"
	IPv6ICanHazIP = "http://ipv6.icanhazip.com"
)

func getICanHazIP(ctx context.Context, url string) (netip.Addr, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return netip.Addr{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return netip.Addr{}, err
	}
	defer func() {
		resp.Body.Close()
	}()

	ipBytes, err := io.ReadAll(resp.Body)
	ipString := strings.TrimRight(string(ipBytes), "\n")

	addr, err := netip.ParseAddr(ipString)

	return addr, fmt.Errorf("could not parse IP address %v: %w", ipString, err)
}

// GetIPv4 fetches the public IPv4 address of the host.
func GetMyIPv4(ctx context.Context) (netip.Addr, error) {
	return getICanHazIP(ctx, IPv4ICanHazIP)
}

// GetIPv6 fetches the public IPv6 address of the host.
func GetMyIPv6(ctx context.Context) (netip.Addr, error) {
	return getICanHazIP(ctx, IPv6ICanHazIP)
}

// GetMyIP fetches the public IP address of the host.
func GetMyIPs(ctx context.Context) ([]netip.Addr, error) {
	addrs := make([]netip.Addr, 0, 0)

	ipv6, err := GetMyIPv6(ctx)
	if err == nil {
		addrs = append(addrs, ipv6)
	}

	ipv4, err := GetMyIPv4(ctx)
	if err != nil {
		addrs = append(addrs, ipv4)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("could not fetch IP address")
	}

	return addrs, nil
}
