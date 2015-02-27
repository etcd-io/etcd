package rafthttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

// TestStreamWriterAttachOutgoingConn tests that outgoingConn can be attached
// to streamWriter. After that, streamWriter can use it to send messages
// continuously, and closes it when stopped.
func TestStreamWriterAttachOutgoingConn(t *testing.T) {
	sw := startStreamWriter(&stats.FollowerStats{}, &nopProcessor{})
	// the expected initial state of streamWrite is not working
	if g := sw.isWorking(); g != false {
		t.Errorf("initial working status = %v, want false", g)
	}

	// repeatitive tests to ensure it can use latest connection
	var wfc *fakeWriteFlushCloser
	for i := 0; i < 3; i++ {
		prevwfc := wfc
		wfc = &fakeWriteFlushCloser{}
		sw.attach(&outgoingConn{t: streamTypeMessage, Writer: wfc, Flusher: wfc, Closer: wfc})
		testutil.ForceGosched()
		// previous attached connection should be closed
		if prevwfc != nil && prevwfc.closed != true {
			t.Errorf("#%d: close of previous connection = %v, want true", i, prevwfc.closed)
		}
		// starts working
		if g := sw.isWorking(); g != true {
			t.Errorf("#%d: working status = %v, want true", i, g)
		}

		sw.msgc <- raftpb.Message{}
		testutil.ForceGosched()
		// still working
		if g := sw.isWorking(); g != true {
			t.Errorf("#%d: working status = %v, want true", i, g)
		}
		if wfc.written == 0 {
			t.Errorf("#%d: failed to write to the underlying connection", i)
		}
	}

	sw.stop()
	// no longer in working status now
	if g := sw.isWorking(); g != false {
		t.Errorf("working status after stop = %v, want false", g)
	}
	if wfc.closed != true {
		t.Errorf("failed to close the underlying connection")
	}
}

// TestStreamWriterAttachBadOutgoingConn tests that streamWriter with bad
// outgoingConn will close the outgoingConn and fall back to non-working status.
func TestStreamWriterAttachBadOutgoingConn(t *testing.T) {
	sw := startStreamWriter(&stats.FollowerStats{}, &nopProcessor{})
	defer sw.stop()
	wfc := &fakeWriteFlushCloser{err: errors.New("blah")}
	sw.attach(&outgoingConn{t: streamTypeMessage, Writer: wfc, Flusher: wfc, Closer: wfc})

	sw.msgc <- raftpb.Message{}
	testutil.ForceGosched()
	// no longer working
	if g := sw.isWorking(); g != false {
		t.Errorf("working = %v, want false", g)
	}
	if wfc.closed != true {
		t.Errorf("failed to close the underlying connection")
	}
}

func TestStreamReaderDialRequest(t *testing.T) {
	for i, tt := range []streamType{streamTypeMsgApp, streamTypeMessage} {
		tr := &roundTripperRecorder{}
		sr := &streamReader{
			tr:         tr,
			u:          "http://localhost:7001",
			t:          tt,
			from:       types.ID(1),
			to:         types.ID(2),
			cid:        types.ID(1),
			msgAppTerm: 1,
		}
		sr.dial()

		req := tr.Request()
		var wurl string
		switch tt {
		case streamTypeMsgApp:
			wurl = "http://localhost:7001/raft/stream/1"
		case streamTypeMessage:
			wurl = "http://localhost:7001/raft/stream/message/1"
		}
		if req.URL.String() != wurl {
			t.Errorf("#%d: url = %s, want %s", i, req.URL.String(), wurl)
		}
		if w := "GET"; req.Method != w {
			t.Errorf("#%d: method = %s, want %s", i, req.Method, w)
		}
		if g := req.Header.Get("X-Etcd-Cluster-ID"); g != "1" {
			t.Errorf("#%d: header X-Etcd-Cluster-ID = %s, want 1", i, g)
		}
		if g := req.Header.Get("X-Raft-To"); g != "2" {
			t.Errorf("#%d: header X-Raft-To = %s, want 2", i, g)
		}
		if g := req.Header.Get("X-Raft-Term"); tt == streamTypeMsgApp && g != "1" {
			t.Errorf("#%d: header X-Raft-Term = %s, want 1", i, g)
		}
	}
}

// TestStreamReaderDialResult tests the result of the dial func call meets the
// HTTP response received.
func TestStreamReaderDialResult(t *testing.T) {
	tests := []struct {
		code int
		err  error
		wok  bool
	}{
		{0, errors.New("blah"), false},
		{http.StatusOK, nil, true},
		{http.StatusMethodNotAllowed, nil, false},
		{http.StatusNotFound, nil, false},
		{http.StatusPreconditionFailed, nil, false},
	}
	for i, tt := range tests {
		tr := newRespRoundTripper(tt.code, tt.err)
		sr := &streamReader{
			tr:   tr,
			u:    "http://localhost:7001",
			t:    streamTypeMessage,
			from: types.ID(1),
			to:   types.ID(2),
			cid:  types.ID(1),
		}

		_, err := sr.dial()
		if ok := err == nil; ok != tt.wok {
			t.Errorf("#%d: ok = %v, want %v", i, ok, tt.wok)
		}
	}
}

// TestStream tests that streamReader and streamWriter can build stream to
// send messages between each other.
func TestStream(t *testing.T) {
	tests := []struct {
		t    streamType
		term uint64
		m    raftpb.Message
	}{
		{
			streamTypeMessage,
			0,
			raftpb.Message{Type: raftpb.MsgProp, To: 2},
		},
		{
			streamTypeMsgApp,
			1,
			raftpb.Message{
				Type:    raftpb.MsgApp,
				From:    2,
				To:      1,
				Term:    1,
				LogTerm: 1,
				Index:   3,
				Entries: []raftpb.Entry{{Term: 1, Index: 4}},
			},
		},
	}
	for i, tt := range tests {
		h := &fakeStreamHandler{t: tt.t}
		srv := httptest.NewServer(h)
		defer srv.Close()

		sw := startStreamWriter(&stats.FollowerStats{}, &nopProcessor{})
		defer sw.stop()
		h.sw = sw

		recvc := make(chan raftpb.Message)
		sr := startStreamReader(&http.Transport{}, srv.URL, tt.t, types.ID(1), types.ID(2), types.ID(1), recvc)
		defer sr.stop()
		if tt.t == streamTypeMsgApp {
			sr.updateMsgAppTerm(tt.term)
		}

		sw.msgc <- tt.m
		m := <-recvc
		if !reflect.DeepEqual(m, tt.m) {
			t.Errorf("#%d: message = %+v, want %+v", i, m, tt.m)
		}
	}
}

type fakeWriteFlushCloser struct {
	err     error
	written int
	closed  bool
}

func (wfc *fakeWriteFlushCloser) Write(p []byte) (n int, err error) {
	wfc.written += len(p)
	return len(p), wfc.err
}
func (wfc *fakeWriteFlushCloser) Flush() {}
func (wfc *fakeWriteFlushCloser) Close() error {
	wfc.closed = true
	return wfc.err
}

type fakeStreamHandler struct {
	t  streamType
	sw *streamWriter
}

func (h *fakeStreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.(http.Flusher).Flush()
	c := newCloseNotifier()
	h.sw.attach(&outgoingConn{
		t:       h.t,
		termStr: r.Header.Get("X-Raft-Term"),
		Writer:  w,
		Flusher: w.(http.Flusher),
		Closer:  c,
	})
	<-c.closeNotify()
}
