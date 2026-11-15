package destination

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeResolver returns a resolver that answers every A query with ip, and
// every other query with no records, from a local UDP server.
func fakeResolver(t *testing.T, ip string) *net.Resolver {
	t.Helper()
	answer := net.ParseIP(ip).To4()
	require.NotNil(t, answer)

	conn, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 512)
		for {
			n, from, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteTo(dnsReply(buf[:n], answer), from)
		}
	}()

	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp4", conn.LocalAddr().String())
		},
	}
}

// dnsReply builds a response to query with a single A record when the
// question is for type A.
func dnsReply(query []byte, ip net.IP) []byte {
	if len(query) < 12 {
		return nil
	}
	end := 12
	for end < len(query) && query[end] != 0 {
		end += int(query[end]) + 1
	}
	end += 5 // root label, QTYPE and QCLASS
	if end > len(query) {
		return nil
	}

	reply := append([]byte{}, query[:end]...)
	reply[2], reply[3] = 0x81, 0x80 // response, recursion available
	reply[6], reply[7] = 0, 0       // answers, set below
	reply[8], reply[9], reply[10], reply[11] = 0, 0, 0, 0
	if qtype := uint16(query[end-4])<<8 | uint16(query[end-3]); qtype == 1 {
		reply[7] = 1
		reply = append(reply, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4)
		reply = append(reply, ip...)
	}

	return reply
}
