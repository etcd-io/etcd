package transport

import (
	"net"
	"time"
)

type timeoutConn struct {
	net.Conn
	wtimeoutd  time.Duration
	rdtimeoutd time.Duration
}

func (c timeoutConn) Write(b []byte) (n int, err error) {
	if c.wtimeoutd > 0 {
		if err := c.SetWriteDeadline(time.Now().Add(c.wtimeoutd)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Write(b)
}

func (c timeoutConn) Read(b []byte) (n int, err error) {
	if c.rdtimeoutd > 0 {
		if err := c.SetReadDeadline(time.Now().Add(c.rdtimeoutd)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}
