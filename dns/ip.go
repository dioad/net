// Package dns provides DNS-related utilities, including DNS-over-HTTPS and IP reverse lookups.
package dns

import (
	"errors"
	"net"
	"strconv"
)

func uitoa(i uint64) string {
	return strconv.FormatUint(i, 10)
}

// ReverseIP returns the reverse DNS notation for an IPv4 address (e.g.
// "127.0.0.1" becomes "1.0.0.127"). It errors on IPv6 addresses rather than
// returning an empty string, since it implements no IPv6 (ip6.arpa) form.
func ReverseIP(addr string) (string, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", &net.DNSError{Err: "unrecognized address", Name: addr}
	}
	if ip.To4() == nil {
		return "", &net.DNSError{Err: "reverse DNS notation is only supported for IPv4 addresses", Name: addr}
	}

	return uitoa(uint64(ip[15])) + "." + uitoa(uint64(ip[14])) + "." + uitoa(uint64(ip[13])) + "." + uitoa(uint64(ip[12])), nil
}

// BlocklistLookupAddr checks if the given IP address is listed in the Spamhaus blocklist.
func BlocklistLookupAddr(addr string) (bool, error) {
	revAddr, err := ReverseIP(addr)
	if err != nil {
		return false, err
	}
	spamName := revAddr + ".zen.spamhaus.org"
	responseCodes, err := net.LookupHost(spamName)
	if err != nil {
		if _, ok := errors.AsType[*net.DNSError](err); ok {
			return false, nil
		}

		return false, err
	}
	if len(responseCodes) == 0 {
		return false, nil
	}

	return true, nil
}
