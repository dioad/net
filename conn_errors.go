package net

import (
	"errors"
	"net"
	"strings"
)

// isTLSCloseNotifyError reports whether err is a TLS close_notify failure where
// Go closed the connection successfully despite being unable to send the alert.
// The Go TLS layer appends "but connection was closed anyway" in this case,
// meaning the peer disconnected first and the close itself succeeded.
func isTLSCloseNotifyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "but connection was closed anyway")
}

// IsBenignConnCloseError reports whether err is an expected error from closing
// a net.Conn that does not indicate a server or infrastructure fault:
//   - net.ErrClosed: the connection was already closed before Close was called.
//   - TLS close_notify failure where the underlying connection was already gone.
//
// Add new cases here as further benign close-error patterns are identified.
func IsBenignConnCloseError(err error) bool {
	return errors.Is(err, net.ErrClosed) || isTLSCloseNotifyError(err)
}
