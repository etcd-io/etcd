// Copyright 2016 CoreOS, Inc.
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

package clientv3

import (
	"fmt"
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	storagepb "github.com/coreos/etcd/storage/storagepb"
)

type Watcher interface {
	// Watch watches on a single key. The watched events will be returned
	// through the returned channel.
	// If the watch is slow or the required rev is compacted, the watch request
	// might be canceled from the server-side and the chan will be closed.
	Watch(cxt context.Context, key string, rev int64) <-chan WatchResponse

	// Watch watches on a prefix. The watched events will be returned
	// through the returned channel.
	// If the watch is slow or the required rev is compacted, the watch request
	// might be canceled from the server-side and the chan will be closed.
	WatchPrefix(cxt context.Context, prefix string, rev int64) <-chan WatchResponse

	// Close closes the watcher and cancels all watch requests.
	Close() error
}

type WatchResponse struct {
	Header pb.ResponseHeader
	Events []*storagepb.Event
}

// watcher implements the Watcher interface
type watcher struct {
	c      *Client
	conn   *grpc.ClientConn
	remote pb.WatchClient

	// ctx controls internal remote.Watch requests
	ctx    context.Context
	cancel context.CancelFunc

	// streams holds all active watchers
	streams map[int64]*watcherStream
	// mu protects the streams map
	mu sync.RWMutex

	// reqc sends a watch request from Watch() to the main goroutine
	reqc chan *watchRequest
	// respc receives data from the watch client
	respc chan *pb.WatchResponse
	// stopc is sent to the main goroutine to stop all processing
	stopc chan struct{}
	// donec closes to broadcast shutdown
	donec chan struct{}
	// errc transmits errors from grpc Recv
	errc chan error
}

// watchRequest is issued by the subscriber to start a new watcher
type watchRequest struct {
	ctx    context.Context
	key    string
	prefix string
	rev    int64
	// retc receives a chan WatchResponse once the watcher is established
	retc chan chan WatchResponse
}

// watcherStream represents a registered watcher
type watcherStream struct {
	initReq watchRequest

	// outc publishes watch responses to subscriber
	outc chan<- WatchResponse
	// recvc buffers watch responses before publishing
	recvc chan *WatchResponse
	id    int64

	// lastRev is revision last successfully sent over outc
	lastRev int64
	// resumec indicates the stream must recover at a given revision
	resumec chan int64
}

func NewWatcher(c *Client) Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	conn := c.ActiveConnection()

	w := &watcher{
		c:      c,
		conn:   conn,
		remote: pb.NewWatchClient(conn),

		ctx:     ctx,
		cancel:  cancel,
		streams: make(map[int64]*watcherStream),

		respc: make(chan *pb.WatchResponse),
		reqc:  make(chan *watchRequest),
		stopc: make(chan struct{}),
		donec: make(chan struct{}),
		errc:  make(chan error, 1),
	}
	go w.run()
	return w
}

func (w *watcher) Watch(ctx context.Context, key string, rev int64) <-chan WatchResponse {
	return w.watch(ctx, key, "", rev)
}

func (w *watcher) WatchPrefix(ctx context.Context, prefix string, rev int64) <-chan WatchResponse {
	return w.watch(ctx, "", prefix, rev)
}

func (w *watcher) Close() error {
	select {
	case w.stopc <- struct{}{}:
	case <-w.donec:
	}
	<-w.donec
	return <-w.errc
}

// watch posts a watch request to run() and waits for a new watcher channel
func (w *watcher) watch(ctx context.Context, key, prefix string, rev int64) <-chan WatchResponse {
	retc := make(chan chan WatchResponse, 1)
	wr := &watchRequest{ctx: ctx, key: key, prefix: prefix, rev: rev, retc: retc}
	// submit request
	select {
	case w.reqc <- wr:
	case <-wr.ctx.Done():
		return nil
	case <-w.donec:
		return nil
	}
	// receive channel
	select {
	case ret := <-retc:
		return ret
	case <-ctx.Done():
		return nil
	case <-w.donec:
		return nil
	}
}

func (w *watcher) addStream(resp *pb.WatchResponse, pendingReq *watchRequest) {
	if pendingReq == nil {
		// no pending request; ignore
		return
	} else if resp.WatchId == -1 || resp.Compacted {
		// failed; no channel
		pendingReq.retc <- nil
		return
	}

	ret := make(chan WatchResponse)
	ws := &watcherStream{
		initReq: *pendingReq,
		id:      resp.WatchId,
		outc:    ret,
		// buffered so unlikely to block on sending while holding mu
		recvc:   make(chan *WatchResponse, 4),
		resumec: make(chan int64),
	}

	w.mu.Lock()
	w.streams[ws.id] = ws
	w.mu.Unlock()

	// send messages to subscriber
	go w.serveStream(ws)

	// pass back the subscriber channel for the watcher
	pendingReq.retc <- ret
}

// closeStream closes the watcher resources and removes it
func (w *watcher) closeStream(ws *watcherStream) {
	// cancels request stream; subscriber receives nil channel
	close(ws.initReq.retc)
	// close subscriber's channel
	close(ws.outc)
	// shutdown serveStream
	close(ws.recvc)
	delete(w.streams, ws.id)
}

// run is the root of the goroutines for managing a watcher client
func (w *watcher) run() {
	defer func() {
		close(w.donec)
		w.cancel()
	}()

	// start a stream with the etcd grpc server
	wc, wcerr := w.newWatchClient()
	if wcerr != nil {
		w.errc <- wcerr
		return
	}

	var pendingReq, failedReq *watchRequest
	curReqC := w.reqc
	cancelSet := make(map[int64]struct{})

	for {
		select {
		// Watch() requested
		case pendingReq = <-curReqC:
			// no more watch requests until there's a response
			curReqC = nil
			if err := wc.Send(pendingReq.toPB()); err == nil {
				// pendingReq now waits on w.respc
				break
			}
			failedReq = pendingReq
		// New events from the watch client
		case pbresp := <-w.respc:
			switch {
			case pbresp.Canceled:
				delete(cancelSet, pbresp.WatchId)
			case pbresp.Compacted:
				w.mu.Lock()
				if ws, ok := w.streams[pbresp.WatchId]; ok {
					w.closeStream(ws)
				}
				w.mu.Unlock()
			case pbresp.Created:
				// response to pending req, try to add
				w.addStream(pbresp, pendingReq)
				pendingReq = nil
				curReqC = w.reqc
			default:
				// dispatch to appropriate watch stream
				if ok := w.dispatchEvent(pbresp); ok {
					break
				}
				// watch response on unexpected watch id; cancel id
				if _, ok := cancelSet[pbresp.WatchId]; ok {
					break
				}
				cancelSet[pbresp.WatchId] = struct{}{}
				cr := &pb.WatchRequest_CancelRequest{
					CancelRequest: &pb.WatchCancelRequest{
						WatchId: pbresp.WatchId,
					},
				}
				req := &pb.WatchRequest{RequestUnion: cr}
				wc.Send(req)
			}
		// watch client failed to recv; spawn another if possible
		// TODO report watch client errors from errc?
		case <-w.errc:
			if wc, wcerr = w.newWatchClient(); wcerr != nil {
				w.errc <- wcerr
				return
			}
			curReqC = w.reqc
			if pendingReq != nil {
				failedReq = pendingReq
			}
			cancelSet = make(map[int64]struct{})
		case <-w.stopc:
			w.errc <- nil
			return
		}

		// send failed; queue for retry
		if failedReq != nil {
			go func() {
				select {
				case w.reqc <- pendingReq:
				case <-pendingReq.ctx.Done():
				case <-w.donec:
				}
			}()
			failedReq = nil
			pendingReq = nil
		}
	}
}

// dispatchEvent sends a WatchResponse to the appropriate watcher stream
func (w *watcher) dispatchEvent(pbresp *pb.WatchResponse) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	ws, ok := w.streams[pbresp.WatchId]
	if ok {
		wr := &WatchResponse{*pbresp.Header, pbresp.Events}
		ws.recvc <- wr
	}
	return ok
}

// serveWatchClient forwards messages from the grpc stream to run()
func (w *watcher) serveWatchClient(wc pb.Watch_WatchClient) {
	for {
		resp, err := wc.Recv()
		if err != nil {
			select {
			case w.errc <- err:
			case <-w.donec:
			}
			return
		}
		select {
		case w.respc <- resp:
		case <-w.donec:
			return
		}
	}
}

// serveStream forwards watch responses from run() to the subscriber
func (w *watcher) serveStream(ws *watcherStream) {
	emptyWr := &WatchResponse{}
	wrs := []*WatchResponse{}
	resuming := false
	closing := false
	for !closing {
		curWr := emptyWr
		outc := ws.outc
		if len(wrs) > 0 {
			curWr = wrs[0]
		} else {
			outc = nil
		}
		select {
		case outc <- *curWr:
			newRev := wrs[0].Events[len(wrs[0].Events)-1].Kv.ModRevision
			if newRev != ws.lastRev {
				ws.lastRev = newRev
			}
			wrs[0] = nil
			wrs = wrs[1:]
		case wr, ok := <-ws.recvc:
			if !ok {
				// shutdown from closeStream
				return
			}
			// resume up to last seen event if disconnected
			if resuming {
				resuming = false
				// trim events already seen
				for i := 0; i < len(wr.Events); i++ {
					if wr.Events[i].Kv.ModRevision > ws.lastRev {
						wr.Events = wr.Events[i:]
						break
					}
				}
				// only forward new events
				if wr.Events[0].Kv.ModRevision == ws.lastRev {
					break
				}
			}
			// TODO don't keep buffering if subscriber stops reading
			wrs = append(wrs, wr)
		case resumeRev := <-ws.resumec:
			if resumeRev != ws.lastRev {
				panic("unexpected resume revision")
			}
			wrs = nil
			resuming = true
		case <-w.donec:
			closing = true
		case <-ws.initReq.ctx.Done():
			closing = true
		}
	}
	w.mu.Lock()
	w.closeStream(ws)
	w.mu.Unlock()
	// lazily send cancel message if events on missing id
}

func (w *watcher) newWatchClient() (pb.Watch_WatchClient, error) {
	ws, rerr := w.resume()
	if rerr != nil {
		return nil, rerr
	}
	go w.serveWatchClient(ws)
	return ws, nil
}

// resume creates a new WatchClient with all current watchers reestablished
func (w *watcher) resume() (ws pb.Watch_WatchClient, err error) {
	for {
		if ws, err = w.openWatchClient(); err != nil {
			break
		} else if err = w.resumeWatchers(ws); err == nil {
			break
		}
	}
	return ws, err
}

// openWatchClient retries opening a watchclient until retryConnection fails
func (w *watcher) openWatchClient() (ws pb.Watch_WatchClient, err error) {
	for {
		if ws, err = w.remote.Watch(w.ctx); ws != nil {
			break
		} else if isRPCError(err) {
			return nil, err
		}
		newConn, nerr := w.c.retryConnection(w.conn, nil)
		if nerr != nil {
			return nil, nerr
		}
		w.conn = newConn
		w.remote = pb.NewWatchClient(w.conn)
	}
	return ws, nil
}

// resumeWatchers rebuilds every registered watcher on a new client
func (w *watcher) resumeWatchers(wc pb.Watch_WatchClient) error {
	streams := []*watcherStream{}
	w.mu.RLock()
	for _, ws := range w.streams {
		streams = append(streams, ws)
	}
	w.mu.RUnlock()

	for _, ws := range streams {
		// reconstruct watcher from initial request
		if ws.lastRev != 0 {
			ws.initReq.rev = ws.lastRev
		}
		if err := wc.Send(ws.initReq.toPB()); err != nil {
			return err
		}

		// wait for request ack
		resp, err := wc.Recv()
		if err != nil {
			return err
		} else if len(resp.Events) != 0 || resp.Created != true {
			return fmt.Errorf("watcher: unexpected response (%+v)", resp)
		}

		// id may be different since new remote watcher; update map
		w.mu.Lock()
		delete(w.streams, ws.id)
		ws.id = resp.WatchId
		w.streams[ws.id] = ws
		w.mu.Unlock()

		ws.resumec <- ws.lastRev
	}
	return nil
}

// toPB converts an internal watch request structure to its protobuf messagefunc (wr *watchRequest)
func (wr *watchRequest) toPB() *pb.WatchRequest {
	req := &pb.WatchCreateRequest{StartRevision: wr.rev}
	if wr.key != "" {
		req.Key = []byte(wr.key)
	} else {
		req.Prefix = []byte(wr.prefix)
	}
	cr := &pb.WatchRequest_CreateRequest{CreateRequest: req}
	return &pb.WatchRequest{RequestUnion: cr}
}
