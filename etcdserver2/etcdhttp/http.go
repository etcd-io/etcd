package etcdhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/coreos/etcd/elog"
	etcdserver "github.com/coreos/etcd/etcdserver2"
	"github.com/coreos/etcd/etcdserver2/etcdserverpb"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

var errClosed = errors.New("etcdhttp: client closed connection")

const DefaultTimeout = 500 * time.Millisecond

type Handler struct {
	Timeout time.Duration
	Server  etcdserver.Server
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: set read/write timeout?

	timeout := h.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch {
	case strings.HasPrefix(r.URL.Path, "/raft"):
		h.serveRaft(ctx, w, r)
	case strings.HasPrefix(r.URL.Path, "/keys/"):
		h.serveKeys(ctx, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) serveKeys(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	rr := parseRequest(r)

	resp, err := h.Server.Do(ctx, rr)
	if err != nil {
		// TODO(bmizerany): switch on store errors and etcdserver.ErrUnknownMethod
		panic("TODO")
	}

	if err := encodeResponse(ctx, w, resp); err != nil {
		http.Error(w, "Timeout while waiting for response", 504)
	}
}

func (h Handler) serveRaft(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		elog.TODO()
	}
	var m raftpb.Message
	if err := m.Unmarshal(b); err != nil {
		elog.TODO()
	}
	if err := h.Server.Node.Step(ctx, m); err != nil {
		elog.TODO()
	}
}

func genId() int64 {
	panic("implement me")
}

func parseRequest(r *http.Request) etcdserverpb.Request {
	q := r.URL.Query()
	rr := etcdserverpb.Request{
		Id:        genId(),
		Method:    r.Method,
		Path:      r.URL.Path[len("/keys/"):],
		Val:       q.Get("value"),
		PrevValue: q.Get("prevValue"),
		PrevIndex: parseUint64(q.Get("prevIndex")),
		Recursive: parseBool(q.Get("recursive")),
		Since:     parseUint64(q.Get("waitIndex")),
		Sorted:    parseBool(q.Get("sorted")),
		Wait:      parseBool(q.Get("wait")),
	}

	// PrevExists is nullable, so we leave it null if prevExist wasn't
	// specified.
	_, ok := q["prevExists"]
	if ok {
		bv := parseBool(q.Get("prevExists"))
		rr.PrevExists = &bv
	}

	ttl := parseUint64(q.Get("ttl"))
	if ttl > 0 {
		expr := time.Duration(ttl) * time.Second
		rr.Expiration = time.Now().Add(expr).UnixNano()
	}

	return rr
}

func parseBool(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}

func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, resp etcdserver.Response) (err error) {
	var ev *store.Event
	switch {
	case resp.Event != nil:
		ev = resp.Event
	case resp.Watcher != nil:
		ev, err = waitForEvent(ctx, w, resp.Watcher)
		if err != nil {
			return err
		}
	default:
		panic("should not be reachable")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(ev.Index()))

	if ev.IsCreated() {
		w.WriteHeader(http.StatusCreated)
	}

	if err := json.NewEncoder(w).Encode(ev); err != nil {
		panic(err) // should never be reached
	}
	return nil
}

func waitForEvent(ctx context.Context, w http.ResponseWriter, wa *store.Watcher) (*store.Event, error) {
	// TODO(bmizerany): support streaming?
	defer wa.Remove()
	var nch <-chan bool
	if x, ok := w.(http.CloseNotifier); ok {
		nch = x.CloseNotify()
	}

	select {
	case ev := <-wa.EventChan:
		return ev, nil
	case <-nch:
		elog.TODO()
		return nil, errClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
