package server

import (
	"net"
	"time"
)

type QueuedListener struct {
	*net.TCPListener
	acceptQ chan bool
}

func NewQueuedListener(addr string, acceptLimit int) (QueuedListener, error) {
	tcpaddr, _ := net.ResolveTCPAddr("tcp", addr)
	l, e := net.ListenTCP("tcp", tcpaddr)
	if e != nil {
		return QueuedListener{}, e
	}
	acceptQ := make(chan bool, acceptLimit)
	return QueuedListener{l, acceptQ}, nil
}

func (self QueuedListener) Accept() (net.Conn, error) {
	self.acceptQ <- true

	tcpConn, err := self.TCPListener.AcceptTCP()
	if err != nil {
		return nil, err
	}

	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(2 * time.Second)

	qConn := QueuedConn{tcpConn, self.acceptQ}

	return qConn, nil
}

type QueuedConn struct {
	*net.TCPConn
	queue chan bool
}

func (self QueuedConn) Close() error {
	err := self.TCPConn.Close()
	if err != nil {
		return err
	}

	<-self.queue

	return nil
}
