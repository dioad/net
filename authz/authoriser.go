package authz

import "net"

// Authoriser checks whether a network address is permitted to connect.
type Authoriser interface {
	Authorise(addr *net.TCPAddr) bool
	AuthoriseFromString(addr string) (bool, error)
	AuthoriseConn(c net.Conn) (bool, error)
}
