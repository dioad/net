package net

import (
	"net"
	"net/url"
	"strconv"
)

func TCPAddrFromURL(url *url.URL) (string, error) {
	addr := url.Host
	if url.Port() == "" {
		port, err := TCPPortFromURL(url)
		if err != nil {
			return "", err
		}
		addr = net.JoinHostPort(url.Host, port)
	}
	return addr, nil
}

// returns string because that's what url.URL.Port() does
func TCPPortFromURL(url *url.URL) (string, error) {
	defaultPort := url.Port()
	if defaultPort == "" {
		if url.Scheme == "" {
			return "0", nil
		}
		protoPort, err := net.LookupPort("tcp", url.Scheme)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(protoPort), nil
	}
	return defaultPort, nil
}
