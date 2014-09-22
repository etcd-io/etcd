package transport

import (
	"net"
)

func NewListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
