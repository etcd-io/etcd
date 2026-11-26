package transport

import (
	"errors"
	"net"
	"testing"
	"time"
)

// RED: closing a full LimitListener must unblock a pending Accept.
func TestLimitListenerCloseUnblocksAccept(t *testing.T) {
	fake := &fakeListener2{ch: make(chan net.Conn, 1)}
	fake.ch <- &fakeConn{}
	l := LimitListener(fake, 1)
	c1, err := l.Accept()
	if err != nil || c1 == nil {
		t.Fatalf("first Accept: conn=%v err=%v", c1, err)
	}
	done := make(chan struct{})
	go func() {
		l.Accept()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	l.Close() // close the LimitListener wrapper
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Accept still blocked after listener close")
	}
}

type fakeListener2 struct{ ch chan net.Conn }

func (f *fakeListener2) Accept() (net.Conn, error) {
	c, ok := <-f.ch
	if !ok {
		return nil, errors.New("closed")
	}
	return c, nil
}
func (f *fakeListener2) Close() error  { close(f.ch); return nil }
func (f *fakeListener2) Addr() net.Addr { return nil }

type fakeConn struct{}

func (*fakeConn) Read([]byte) (int, error)         { return 0, nil }
func (*fakeConn) Write([]byte) (int, error)        { return 0, nil }
func (*fakeConn) Close() error                     { return nil }
func (*fakeConn) LocalAddr() net.Addr              { return nil }
func (*fakeConn) RemoteAddr() net.Addr             { return nil }
func (*fakeConn) SetDeadline(time.Time) error      { return nil }
func (*fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (*fakeConn) SetWriteDeadline(time.Time) error { return nil }
