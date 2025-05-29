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
	// TODO: change this to ipv4.myip.dioad.net(A) ipv6.myip.dioad.net (AAAA) and myip.dioad.net(A and AAAA)
	// IPv4ICanHazIP is the URL to fetch the public IPv4 address.
	IPv4ICanHazIP = "http://ipv4.icanhazip.com"
	// IPv6ICanHazIP is the URL to fetch the public IPv6 address.
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
	if err != nil {
		return netip.Addr{}, fmt.Errorf("could not parse IP address %v: %w", ipString, err)
	}

	return addr, nil
}

// GetMyIPv4 fetches the public IPv4 address of the host.
func GetMyIPv4(ctx context.Context) (netip.Addr, error) {
	return getICanHazIP(ctx, IPv4ICanHazIP)
}

// GetMyIPv6 fetches the public IPv6 address of the host.
func GetMyIPv6(ctx context.Context) (netip.Addr, error) {
	return getICanHazIP(ctx, IPv6ICanHazIP)
}

// GetIPFunc is a function type that fetches an IP address.
type GetIPFunc func(ctx context.Context) (netip.Addr, error)

// GetMyIPsFromFuncs fetches the public IP addresses using the provided functions.
func GetMyIPsFromFuncs(ctx context.Context, funcs ...GetIPFunc) ([]netip.Addr, error) {
	var ipAddresses []netip.Addr
	var err error
	for _, f := range funcs {
		var ip netip.Addr
		ip, err = f(ctx)
		if err == nil {
			ipAddresses = append(ipAddresses, ip)
		}
	}

	if len(ipAddresses) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("could not fetch IP address")
	}

	return ipAddresses, nil
}

// GetMyIPs fetches the public IP addresses of the host.
func GetMyIPs(ctx context.Context) ([]netip.Addr, error) {
	return GetMyIPsFromFuncs(ctx, GetMyIPv6, GetMyIPv4)
}
