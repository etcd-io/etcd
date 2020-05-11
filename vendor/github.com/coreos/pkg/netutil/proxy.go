package netutil

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// ProxyTCP proxies between two TCP connections.
// Because TLS connections don't have CloseRead() and CloseWrite() methods, our
// temporary solution is to use timeouts.
func ProxyTCP(conn1, conn2 net.Conn, tlsWriteDeadline, tlsReadDeadline time.Duration) {
	var wg sync.WaitGroup
	wg.Add(2)

	go copyBytes(conn1, conn2, &wg, tlsWriteDeadline, tlsReadDeadline)
	go copyBytes(conn2, conn1, &wg, tlsWriteDeadline, tlsReadDeadline)

	wg.Wait()
	conn1.Close()
	conn2.Close()
}

func copyBytes(dst, src net.Conn, wg *sync.WaitGroup, writeDeadline, readDeadline time.Duration) {
	defer wg.Done()
	_, err := io.Copy(dst, src)
	if err != nil {
		log.Printf("proxy i/o error: %v", err)
	}

	if cr, ok := src.(*net.TCPConn); ok {
		cr.CloseRead()
	} else {
		// For TLS connections.
		wto := time.Now().Add(writeDeadline)
		src.SetWriteDeadline(wto)
	}

	if cw, ok := dst.(*net.TCPConn); ok {
		cw.CloseWrite()
	} else {
		// For TLS connections.
		rto := time.Now().Add(readDeadline)
		dst.SetReadDeadline(rto)
	}
}
