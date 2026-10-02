package authz

import "net"

// Authoriser checks whether a network address is permitted to connect. A nil
// return means permitted; a non-nil return describes why not, and matches
// errors.Is(err, ErrDenied) when the implementation deliberately denied the
// address (as opposed to failing to evaluate it, e.g. a malformed address).
type Authoriser interface {
	Authorise(addr *net.TCPAddr) error
	AuthoriseFromString(addr string) error
	AuthoriseConn(c net.Conn) error
}
