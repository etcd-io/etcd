package transport

import (
	"net"
	"testing"
	"time"
)

func TestWriteReadTimeoutListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unexpected listen error: %v", err)
	}
	wln := rwTimeoutListener{
		Listener:   ln,
		wtimeoutd:  10 * time.Millisecond,
		rdtimeoutd: 10 * time.Millisecond,
	}
	stop := make(chan struct{})

	blocker := func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatalf("unexpected dail error: %v", err)
		}
		defer conn.Close()
		// block the receiver until the writer timeout
		<-stop
	}
	go blocker()

	conn, err := wln.Accept()
	if err != nil {
		t.Fatalf("unexpected accept error: %v", err)
	}
	defer conn.Close()

	// fill the socket buffer
	data := make([]byte, 1024*1024)
	timer := time.AfterFunc(wln.wtimeoutd*5, func() {
		t.Fatal("wait timeout")
	})
	defer timer.Stop()

	_, err = conn.Write(data)
	if operr, ok := err.(*net.OpError); !ok || operr.Op != "write" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}
	stop <- struct{}{}

	timer.Reset(wln.rdtimeoutd * 5)
	go blocker()

	conn, err = wln.Accept()
	if err != nil {
		t.Fatalf("unexpected accept error: %v", err)
	}
	buf := make([]byte, 10)
	_, err = conn.Read(buf)
	if operr, ok := err.(*net.OpError); !ok || operr.Op != "read" || !operr.Timeout() {
		t.Errorf("err = %v, want write i/o timeout error", err)
	}
	stop <- struct{}{}
}
