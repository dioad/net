package http

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// tlsHandshakeErrorRe parses the Go stdlib http.Server error log format:
//
//	server.go:N: http: TLS handshake error from <addr>: <reason>
var tlsHandshakeErrorRe = regexp.MustCompile(`http: TLS handshake error from ([^\s]+): (.+)$`)

// benignTLSHandshakeReasonSubstrings are substring markers for reasons that
// represent expected probe/scanner behaviour. "EOF" and "unexpected EOF" are
// handled separately with exact equality in isBenignTLSHandshakeReason.
var benignTLSHandshakeReasonSubstrings = []string{
	"i/o deadline reached",
	"i/o timeout",
	"connection reset by peer",
	"unsupported application protocols",
	"unsupported versions",
	"no cipher suite",
}

// tlsHandshakeErrorFilter is an io.Writer for http.Server.ErrorLog that routes
// benign TLS handshake failures from external probes/scanners to WARN with
// structured fields, and forwards everything else at ERROR.
type tlsHandshakeErrorFilter struct {
	logger zerolog.Logger
}

func (f *tlsHandshakeErrorFilter) Write(p []byte) (n int, err error) {
	if !bytes.Contains(p, []byte("TLS handshake error from")) {
		f.logger.Error().Msg(strings.TrimRight(string(p), "\r\n"))
		return len(p), nil
	}
	msg := strings.TrimRight(string(p), "\r\n")
	if m := tlsHandshakeErrorRe.FindStringSubmatch(msg); m != nil {
		remoteAddr, reason := m[1], m[2]
		if isBenignTLSHandshakeReason(reason) {
			f.logger.Warn().
				Str("remote_addr", remoteAddr).
				Str("reason", reason).
				Msg("tls_probe")
			return len(p), nil
		}
		f.logger.Error().
			Str("remote_addr", remoteAddr).
			Str("reason", reason).
			Msg("tls_handshake_error")
		return len(p), nil
	}
	f.logger.Error().Msg(msg)
	return len(p), nil
}

// isBenignTLSHandshakeReason reports whether reason represents expected
// external probe/scanner behaviour rather than a server or configuration fault.
func isBenignTLSHandshakeReason(reason string) bool {
	// Exact equality for io.EOF / io.ErrUnexpectedEOF: using Contains would
	// silently classify any future error containing "EOF" as a substring as benign.
	if reason == "EOF" || reason == "unexpected EOF" {
		return true
	}
	for _, s := range benignTLSHandshakeReasonSubstrings {
		if strings.Contains(reason, s) {
			return true
		}
	}
	return false
}
