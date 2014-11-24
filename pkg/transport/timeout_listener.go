package transport

import (
	"net"
	"time"
)

type rwTimeoutListener struct {
	net.Listener
	wtimeoutd  time.Duration
	rdtimeoutd time.Duration
}

func (rwln *rwTimeoutListener) Accept() (net.Conn, error) {
	c, err := rwln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return timeoutConn{
		Conn:       c,
		wtimeoutd:  rwln.wtimeoutd,
		rdtimeoutd: rwln.rdtimeoutd,
	}, nil
}
