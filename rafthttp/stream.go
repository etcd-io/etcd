// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rafthttp

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

type streamType string

const (
	streamTypeMessage streamType = "message"
	streamTypeMsgApp  streamType = "msgapp"

	streamBufSize = 4096
)

var (
	// linkHeartbeatMessage is a special message used as heartbeat message in
	// link layer. It never conflicts with messages from raft because raft
	// doesn't send out messages without From and To fields.
	linkHeartbeatMessage = raftpb.Message{Type: raftpb.MsgHeartbeat}
)

func isLinkHeartbeatMessage(m raftpb.Message) bool {
	return m.Type == raftpb.MsgHeartbeat && m.From == 0 && m.To == 0
}

type outgoingConn struct {
	t       streamType
	termStr string
	io.Writer
	http.Flusher
	io.Closer
}

// streamWriter is a long-running worker that writes messages into the
// attached outgoingConn.
type streamWriter struct {
	fs *stats.FollowerStats

	mu      sync.Mutex // guard field working and closer
	closer  io.Closer
	working bool

	msgc  chan raftpb.Message
	connc chan *outgoingConn
	stopc chan struct{}
	done  chan struct{}
}

func startStreamWriter(fs *stats.FollowerStats) *streamWriter {
	w := &streamWriter{
		fs:    fs,
		msgc:  make(chan raftpb.Message, streamBufSize),
		connc: make(chan *outgoingConn),
		stopc: make(chan struct{}),
		done:  make(chan struct{}),
	}
	go w.run()
	return w
}

func (cw *streamWriter) run() {
	var msgc chan raftpb.Message
	var heartbeatc <-chan time.Time
	var t streamType
	var msgAppTerm uint64
	var enc encoder
	var flusher http.Flusher
	tickc := time.Tick(ConnReadTimeout / 3)

	for {
		select {
		case <-heartbeatc:
			if err := enc.encode(linkHeartbeatMessage); err != nil {
				log.Printf("rafthttp: failed to heartbeat on stream %s due to %v. waiting for a new stream to be established.", t, err)
				cw.resetCloser()
				heartbeatc, msgc = nil, nil
				continue
			}
			flusher.Flush()
		case m := <-msgc:
			if t == streamTypeMsgApp && m.Term != msgAppTerm {
				// TODO: reasonable retry logic
				if m.Term > msgAppTerm {
					cw.resetCloser()
					heartbeatc, msgc = nil, nil
				}
				continue
			}
			if err := enc.encode(m); err != nil {
				log.Printf("rafthttp: failed to send message on stream %s due to %v. waiting for a new stream to be established.", t, err)
				cw.resetCloser()
				heartbeatc, msgc = nil, nil
				continue
			}
			flusher.Flush()
		case conn := <-cw.connc:
			cw.resetCloser()
			t = conn.t
			switch conn.t {
			case streamTypeMsgApp:
				var err error
				msgAppTerm, err = strconv.ParseUint(conn.termStr, 10, 64)
				if err != nil {
					log.Panicf("rafthttp: unexpected parse term %s error: %v", conn.termStr, err)
				}
				enc = &msgAppEncoder{w: conn.Writer, fs: cw.fs}
			case streamTypeMessage:
				enc = &messageEncoder{w: conn.Writer}
			default:
				log.Panicf("rafthttp: unhandled stream type %s", conn.t)
			}
			flusher = conn.Flusher
			cw.mu.Lock()
			cw.closer = conn.Closer
			cw.working = true
			cw.mu.Unlock()
			heartbeatc, msgc = tickc, cw.msgc
		case <-cw.stopc:
			cw.resetCloser()
			close(cw.done)
			return
		}
	}
}

func (cw *streamWriter) isWorking() bool {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	return cw.working
}

func (cw *streamWriter) resetCloser() {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	if cw.working {
		cw.closer.Close()
	}
	cw.working = false
}

func (cw *streamWriter) attach(conn *outgoingConn) bool {
	select {
	case cw.connc <- conn:
		return true
	case <-cw.done:
		return false
	}
}

func (cw *streamWriter) stop() {
	close(cw.stopc)
	<-cw.done
}

// streamReader is a long-running go-routine that dials to the remote stream
// endponit and reads messages from the response body returned.
type streamReader struct {
	tr       http.RoundTripper
	u        string
	t        streamType
	from, to types.ID
	cid      types.ID
	recvc    chan<- raftpb.Message

	mu         sync.Mutex
	msgAppTerm uint64
	req        *http.Request
	closer     io.Closer
	stopc      chan struct{}
	done       chan struct{}
}

func startStreamReader(tr http.RoundTripper, u string, t streamType, from, to, cid types.ID, recvc chan<- raftpb.Message) *streamReader {
	r := &streamReader{
		tr:    tr,
		u:     u,
		t:     t,
		from:  from,
		to:    to,
		cid:   cid,
		recvc: recvc,
		stopc: make(chan struct{}),
		done:  make(chan struct{}),
	}
	go r.run()
	return r
}

func (cr *streamReader) run() {
	for {
		rc, err := cr.roundtrip()
		if err != nil {
			log.Printf("rafthttp: roundtripping error: %v", err)
		} else {
			err := cr.decodeLoop(rc)
			if err != io.EOF && !isClosedConnectionError(err) {
				log.Printf("rafthttp: failed to read message on stream %s due to %v", cr.t, err)
			}
		}
		select {
		// Wait 100ms to create a new stream, so it doesn't bring too much
		// overhead when retry.
		case <-time.After(100 * time.Millisecond):
		case <-cr.stopc:
			close(cr.done)
			return
		}
	}
}

func (cr *streamReader) decodeLoop(rc io.ReadCloser) error {
	var dec decoder
	cr.mu.Lock()
	switch cr.t {
	case streamTypeMsgApp:
		dec = &msgAppDecoder{r: rc, local: cr.from, remote: cr.to, term: cr.msgAppTerm}
	case streamTypeMessage:
		dec = &messageDecoder{r: rc}
	default:
		log.Panicf("rafthttp: unhandled stream type %s", cr.t)
	}
	cr.closer = rc
	cr.mu.Unlock()

	for {
		m, err := dec.decode()
		switch {
		case err != nil:
			cr.mu.Lock()
			cr.resetCloser()
			cr.mu.Unlock()
			return err
		case isLinkHeartbeatMessage(m):
			// do nothing for linkHeartbeatMessage
		default:
			select {
			case cr.recvc <- m:
			default:
				log.Printf("rafthttp: dropping %s from %x because receive buffer is blocked",
					m.Type, m.From)
			}
		}
	}
}

func (cr *streamReader) update(u string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.u = u
	cr.resetCloser()
}

func (cr *streamReader) updateMsgAppTerm(term uint64) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if cr.msgAppTerm == term {
		return
	}
	cr.msgAppTerm = term
	cr.resetCloser()
}

// TODO: always cancel in-flight dial and decode
func (cr *streamReader) stop() {
	close(cr.stopc)
	cr.mu.Lock()
	cr.cancelRequest()
	cr.resetCloser()
	cr.mu.Unlock()
	<-cr.done
}

func (cr *streamReader) isWorking() bool {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return cr.closer != nil
}

func (cr *streamReader) roundtrip() (io.ReadCloser, error) {
	cr.mu.Lock()
	u := cr.u
	term := cr.msgAppTerm
	cr.mu.Unlock()

	uu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("parse url %s error: %v", u, err)
	}
	switch cr.t {
	case streamTypeMsgApp:
		// for backward compatibility of v2.0
		uu.Path = path.Join(RaftStreamPrefix, cr.from.String())
	case streamTypeMessage:
		uu.Path = path.Join(RaftStreamPrefix, string(streamTypeMessage), cr.from.String())
	default:
		log.Panicf("rafthttp: unhandled stream type %v", cr.t)
	}
	req, err := http.NewRequest("GET", uu.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("new request to %s error: %v", u, err)
	}
	req.Header.Set("X-Etcd-Cluster-ID", cr.cid.String())
	req.Header.Set("X-Raft-To", cr.to.String())
	if cr.t == streamTypeMsgApp {
		req.Header.Set("X-Raft-Term", strconv.FormatUint(term, 10))
	}
	cr.mu.Lock()
	cr.req = req
	cr.mu.Unlock()
	resp, err := cr.tr.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("error roundtripping to %s: %v", req.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unhandled http status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (cr *streamReader) cancelRequest() {
	if canceller, ok := cr.tr.(*http.Transport); ok {
		canceller.CancelRequest(cr.req)
	}
}

func (cr *streamReader) resetCloser() {
	if cr.closer != nil {
		cr.closer.Close()
	}
	cr.closer = nil
}

func canUseMsgAppStream(m raftpb.Message) bool {
	return m.Type == raftpb.MsgApp && m.Term == m.LogTerm
}

func isClosedConnectionError(err error) bool {
	operr, ok := err.(*net.OpError)
	return ok && operr.Err.Error() == "use of closed network connection"
}
