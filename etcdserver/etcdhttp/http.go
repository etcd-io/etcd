package etcdhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/elog"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

const (
	keysPrefix     = "/v2/keys"
	machinesPrefix = "/v2/machines"
	raftPrefix     = "/raft"

	DefaultTimeout = 500 * time.Millisecond
)

var errClosed = errors.New("etcdhttp: client closed connection")

// NewHandler generates a muxed http.Handler with the given parameters.
func NewHandler(server etcdserver.Server, peers Peers, timeout time.Duration) http.Handler {
	sh := &serverHandler{
		timeout: timeout,
		server:  server,
		peers:   peers,
	}
	if sh.timeout == 0 {
		sh.timeout = DefaultTimeout
	}
	mux := http.NewServeMux()
	mux.HandleFunc(raftPrefix, sh.serveRaft)
	mux.HandleFunc(keysPrefix, sh.serveKeys)
	mux.HandleFunc(keysPrefix+"/", sh.serveKeys)
	// TODO: dynamic configuration may make this outdated. take care of it.
	// TODO: dynamic configuration may introduce race also.
	mux.HandleFunc(machinesPrefix, sh.serveMachines)
	mux.HandleFunc("/", http.NotFound)
	return mux
}

// serverHandler provides http.Handlers for etcd client and raft communication.
type serverHandler struct {
	timeout time.Duration
	server  etcdserver.Server
	peers   Peers
}

func (h serverHandler) serveKeys(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET", "PUT", "POST", "DELETE") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	rr, err := parseRequest(r, etcdserver.GenID())
	if err != nil {
		writeError(w, err)
		return
	}

	resp, err := h.server.Do(ctx, rr)
	if err != nil {
		writeError(w, err)
		return
	}

	var ev *store.Event
	switch {
	case resp.Event != nil:
		ev = resp.Event
	case resp.Watcher != nil:
		if ev, err = waitForEvent(ctx, w, resp.Watcher); err != nil {
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
			return
		}
	default:
		writeError(w, errors.New("received response with no Event/Watcher!"))
		return
	}

	if err = writeEvent(w, ev); err != nil {
		// Should never be reached
		log.Println("error writing event: %v", err)
	}
}

// serveMachines responds address list in the format '0.0.0.0, 1.1.1.1'.
// TODO: rethink the format of machine list because it is not json format.
func (h serverHandler) serveMachines(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET", "HEAD") {
		return
	}
	endpoints := h.peers.Endpoints()
	w.Write([]byte(strings.Join(endpoints, ", ")))
}

func (h serverHandler) serveRaft(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "POST") {
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("etcdhttp: error reading raft message:", err)
		http.Error(w, "error reading raft message", http.StatusBadRequest)
		return
	}
	var m raftpb.Message
	if err := m.Unmarshal(b); err != nil {
		log.Println("etcdhttp: error unmarshaling raft message:", err)
		http.Error(w, "error unmarshaling raft message", http.StatusBadRequest)
		return
	}
	log.Printf("etcdhttp: raft recv message from %#x: %+v", m.From, m)
	if err := h.server.Process(context.TODO(), m); err != nil {
		log.Println("etcdhttp: error processing raft message:", err)
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseRequest converts a received http.Request to a server Request,
// performing validation of supplied fields as appropriate.
// If any validation fails, an empty Request and non-nil error is returned.
func parseRequest(r *http.Request, id int64) (etcdserverpb.Request, error) {
	emptyReq := etcdserverpb.Request{}

	err := r.ParseForm()
	if err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidForm,
			err.Error(),
		)
	}

	if !strings.HasPrefix(r.URL.Path, keysPrefix) {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidForm,
			"incorrect key prefix",
		)
	}
	p := r.URL.Path[len(keysPrefix):]

	var pIdx, wIdx, ttl uint64
	if pIdx, err = getUint64(r.Form, "prevIndex"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeIndexNaN,
			`invalid value for "prevIndex"`,
		)
	}
	if wIdx, err = getUint64(r.Form, "waitIndex"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeIndexNaN,
			`invalid value for "waitIndex"`,
		)
	}
	if ttl, err = getUint64(r.Form, "ttl"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeTTLNaN,
			`invalid value for "ttl"`,
		)
	}

	var rec, sort, wait bool
	if rec, err = getBool(r.Form, "recursive"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`invalid value for "recursive"`,
		)
	}
	if sort, err = getBool(r.Form, "sorted"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`invalid value for "sorted"`,
		)
	}
	if wait, err = getBool(r.Form, "wait"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`invalid value for "wait"`,
		)
	}

	// prevExists is nullable, so leave it null if not specified
	var pe *bool
	if _, ok := r.Form["prevExists"]; ok {
		bv, err := getBool(r.Form, "prevExists")
		if err != nil {
			return emptyReq, etcdErr.NewRequestError(
				etcdErr.EcodeInvalidField,
				"invalid value for prevExists",
			)
		}
		pe = &bv
	}

	rr := etcdserverpb.Request{
		Id:         id,
		Method:     r.Method,
		Path:       p,
		Val:        r.FormValue("value"),
		PrevValue:  r.FormValue("prevValue"),
		PrevIndex:  pIdx,
		PrevExists: pe,
		Recursive:  rec,
		Since:      wIdx,
		Sorted:     sort,
		Wait:       wait,
	}

	if pe != nil {
		rr.PrevExists = pe
	}

	// TODO(jonboulle): use fake clock instead of time module
	// https://github.com/coreos/etcd/issues/1021
	if ttl > 0 {
		expr := time.Duration(ttl) * time.Second
		rr.Expiration = time.Now().Add(expr).UnixNano()
	}

	return rr, nil
}

// getUint64 extracts a uint64 by the given key from a Form. If the key does
// not exist in the form, 0 is returned. If the key exists but the value is
// badly formed, an error is returned. If multiple values are present only the
// first is considered.
func getUint64(form url.Values, key string) (i uint64, err error) {
	if vals, ok := form[key]; ok {
		i, err = strconv.ParseUint(vals[0], 10, 64)
	}
	return
}

// getBool extracts a bool by the given key from a Form. If the key does not
// exist in the form, false is returned. If the key exists but the value is
// badly formed, an error is returned. If multiple values are present only the
// first is considered.
func getBool(form url.Values, key string) (b bool, err error) {
	if vals, ok := form[key]; ok {
		b, err = strconv.ParseBool(vals[0])
	}
	return
}

// writeError logs and writes the given Error to the ResponseWriter
// If Error is an etcdErr, it is rendered to the ResponseWriter
// Otherwise, it is assumed to be an InternalServerError
func writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	log.Println(err)
	if e, ok := err.(*etcdErr.Error); ok {
		e.Write(w)
	} else {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// writeEvent serializes the given Event and writes the resulting JSON to the
// given ResponseWriter
func writeEvent(w http.ResponseWriter, ev *store.Event) error {
	if ev == nil {
		return errors.New("cannot write empty Event!")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(ev.Index()))

	if ev.IsCreated() {
		w.WriteHeader(http.StatusCreated)
	}

	return json.NewEncoder(w).Encode(ev)
}

// waitForEvent waits for a given Watcher to return its associated
// event. It returns a non-nil error if the given Context times out
// or the given ResponseWriter triggers a CloseNotify.
func waitForEvent(ctx context.Context, w http.ResponseWriter, wa store.Watcher) (*store.Event, error) {
	// TODO(bmizerany): support streaming?
	defer wa.Remove()
	var nch <-chan bool
	if x, ok := w.(http.CloseNotifier); ok {
		nch = x.CloseNotify()
	}
	select {
	case ev := <-wa.EventChan():
		return ev, nil
	case <-nch:
		elog.TODO()
		return nil, errClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// allowMethod verifies that the given method is one of the allowed methods,
// and if not, it writes an error to w.  A boolean is returned indicating
// whether or not the method is allowed.
func allowMethod(w http.ResponseWriter, m string, ms ...string) bool {
	for _, meth := range ms {
		if m == meth {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(ms, ","))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}
