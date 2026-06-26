package http

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// tlsHandshakeErrorRe parses the Go stdlib http.Server error log format:
//
//	server.go:N: http: TLS handshake error from <addr>: <reason>
var tlsHandshakeErrorRe = regexp.MustCompile(`http: TLS handshake error from ([^\s]+): (.+)$`)

// tlsHandshakeErrorFilter is an io.Writer for http.Server.ErrorLog that routes
// benign TLS handshake failures from external probes/scanners to WARN with
// structured fields, and forwards everything else at ERROR.
type tlsHandshakeErrorFilter struct {
	logger zerolog.Logger
}

func (f *tlsHandshakeErrorFilter) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	if m := tlsHandshakeErrorRe.FindStringSubmatch(msg); m != nil {
		reason := m[2]
		if isBenignTLSHandshakeReason(reason) {
			f.logger.Warn().
				Str("remote_addr", m[1]).
				Str("reason", reason).
				Msg("tls_probe")
			return len(p), nil
		}
	}
	f.logger.Error().Msg(msg)
	return len(p), nil
}

// isBenignTLSHandshakeReason returns true for reasons that represent expected
// external probe/scanner behaviour rather than a server or configuration fault.
func isBenignTLSHandshakeReason(reason string) bool {
	for _, s := range []string{
		"i/o deadline reached",
		"i/o timeout",
		"connection reset by peer",
		"EOF",
		"unsupported application protocols",
		"unsupported versions",
		"no cipher suite",
	} {
		if strings.Contains(reason, s) {
			return true
		}
	}
	return false
}
