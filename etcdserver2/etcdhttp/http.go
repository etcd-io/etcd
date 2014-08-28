package etcdhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"code.google.com/p/go.net/context"
	"github.com/coreos/etcd/elog"
	etcdserver "github.com/coreos/etcd/etcdserver2"
	"github.com/coreos/etcd/raft"
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
	case strings.HasPrefix(r.URL.Path, "/keys"):
		h.serveKeys(ctx, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) serveKeys(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	rr, err := parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

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
	var m raft.Message
	if err := m.Unmarshal(b); err != nil {
		elog.TODO()
	}
	if err := h.Server.Node.Step(ctx, m); err != nil {
		elog.TODO()
	}
}

func parseRequest(r *http.Request) (etcdserver.Request, error) {
	return etcdserver.Request{}, nil
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
	} else {
		w.WriteHeader(http.StatusOK)
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
