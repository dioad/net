package net

import (
	"bytes"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"text/template"
)

// TCPAddrFromURL returns a TCP address in the form of "host:port" from a given URL.
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

// TCPPortFromURL returns the TCP port from a given URL.
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

func ConvertAddrToIP(addr netip.Addr) net.IP {
	return addr.AsSlice()
}

func FindInterfaceForAddr(a netip.Addr) (string, error) {
	ip := ConvertAddrToIP(a)

	return FindInterfaceForIP(ip)
}

func FindInterfaceForIP(ip net.IP) (string, error) {
	interfaces, err := net.Interfaces()

	if err != nil {
		return "", err
	}

	for _, i := range interfaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if v.Contains(ip) {
					return i.Name, nil
				}
			case *net.IPAddr:
				if v.IP.Equal(ip) {
					return i.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not find interface for %v", ip)
}

func AddrPortDetailsFromString(addrPort string) (netip.AddrPort, string, error) {
	listenAddr, err := netip.ParseAddrPort(addrPort)
	if err != nil {
		return netip.AddrPort{}, "", err
	}
	listenInterface, err := FindInterfaceForAddr(listenAddr.Addr())
	if err != nil {
		return netip.AddrPort{}, "", err
	}

	return listenAddr, listenInterface, nil
}

func ExpandStringTemplate(templateString string, data any) (string, error) {
	tmpl, err := template.New("tmpl").Parse(templateString)
	if err != nil {
		return "", err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
