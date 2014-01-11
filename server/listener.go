package server

import (
	"net"
	"time"
)

type listener struct {
	*net.TCPListener
}

func Listen(n string, laddr string) (*listener, error) {
	tcpListener, err := net.Listen(n, laddr)
	if err != nil {
		return nil, err
	}

	return &listener{tcpListener.(*net.TCPListener)}, nil
}

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// Detect client failure
	c.SetKeepAlive(true)
	c.SetKeepAlivePeriod(time.Millisecond * 200)

	// Seems no way to detect half-disconnection

	return c, nil
}
