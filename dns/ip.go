package dns

import (
	"net"
	"strconv"
)

func uitoa(i uint64) string {
	return strconv.FormatUint(uint64(i), 10)
}

func ReverseIP(addr string) (string, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", &net.DNSError{Err: "unrecognized address", Name: addr}
	}
	if ip.To4() != nil {
		return uitoa(uint64(ip[15])) + "." + uitoa(uint64(ip[14])) + "." + uitoa(uint64(ip[13])) + "." + uitoa(uint64(ip[12])), nil
	}
	return "", nil
}

func BlocklistLookupAddr(addr string) (bool, error) {
	revAddr, err := ReverseIP(addr)
	if err != nil {
		return false, err
	}
	spamName := revAddr + ".zen.spamhaus.org"
	responseCodes, err := net.LookupHost(spamName)
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			return false, nil
		}
		return false, err
	}
	if len(responseCodes) == 0 {
		return false, nil
	}

	return true, nil
}
