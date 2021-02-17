package transport

import (
	"golang.org/x/sys/unix"
	"syscall"
)

type SocketOpts []func(network, addr string, conn syscall.RawConn) error

func (sopts SocketOpts) Control(network, addr string, conn syscall.RawConn) error {
	for _, s := range sopts {
		if err := s(network, addr, conn); err != nil {
			return err
		}
	}
	return nil
}

func SetReuseAddress(network, address string, conn syscall.RawConn) error {
	return conn.Control(func(fd uintptr) {
		syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
}
