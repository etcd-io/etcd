package etcdhttp

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	crand "crypto/rand"
	"math/rand"

	"compress/gzip"
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
)

type Peers map[int64][]string

func (ps Peers) Pick(id int64) string {
	addrs := ps[id]
	if len(addrs) == 0 {
		return ""
	}
	return addScheme(addrs[rand.Intn(len(addrs))])
}

// TODO: improve this when implementing TLS
func addScheme(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

// Set parses command line sets of names to ips formatted like:
// a=1.1.1.1&a=1.1.1.2&b=2.2.2.2
func (ps *Peers) Set(s string) error {
	m := make(map[int64][]string)
	v, err := url.ParseQuery(s)
	if err != nil {
		return err
	}
	for k, v := range v {
		id, err := strconv.ParseInt(k, 0, 64)
		if err != nil {
			return err
		}
		m[id] = v
	}
	*ps = m
	return nil
}

func (ps *Peers) String() string {
	v := url.Values{}
	for k, vv := range *ps {
		for i := range vv {
			v.Add(strconv.FormatInt(k, 16), vv[i])
		}
	}
	return v.Encode()
}

func (ps Peers) IDs() []int64 {
	var ids []int64
	for id := range ps {
		ids = append(ids, id)
	}
	return ids
}

// Endpoints returns a list of all peer addresses. Each address is
// prefixed with "http://". The returned list is sorted (asc).
func (ps Peers) Endpoints() []string {
	endpoints := make([]string, 0)
	for _, addrs := range ps {
		for _, addr := range addrs {
			endpoints = append(endpoints, addScheme(addr))
		}
	}
	sort.Strings(endpoints)

	return endpoints
}

var errClosed = errors.New("etcdhttp: client closed connection")

const DefaultTimeout = 500 * time.Millisecond

func Sender(p Peers) func(msgs []raftpb.Message) {
	return func(msgs []raftpb.Message) {
		for _, m := range msgs {
			// TODO: reuse go routines
			// limit the number of outgoing connections for the same receiver
			go send(p, m)
		}
	}
}

func send(p Peers, m raftpb.Message) {
	// TODO (xiangli): reasonable retry logic
	for i := 0; i < 3; i++ {
		url := p.Pick(m.To)
		if url == "" {
			// TODO: unknown peer id.. what do we do? I
			// don't think his should ever happen, need to
			// look into this further.
			log.Println("etcdhttp: no addr for %d", m.To)
			return
		}

		url += "/raft"

		// TODO: don't block. we should be able to have 1000s
		// of messages out at a time.
		data, err := m.Marshal()
		if err != nil {
			log.Println("etcdhttp: dropping message:", err)
			return // drop bad message
		}
		if httpPost(url, data) {
			return // success
		}
		// TODO: backoff
	}
}

func httpPost(url string, data []byte) bool {
	// TODO: set timeouts
	resp, err := http.Post(url, "application/protobuf", bytes.NewBuffer(data))
	if err != nil {
		elog.TODO()
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		elog.TODO()
		return false
	}
	return true
}

// Handler implements the http.Handler interface and serves etcd client and
// raft communication.
type Handler struct {
	Timeout time.Duration
	Server  *etcdserver.Server
	// TODO: dynamic configuration may make this outdated. take care of it.
	// TODO: dynamic configuration may introduce race also.
	Peers Peers
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
	case strings.HasPrefix(r.URL.Path, keysPrefix):
		h.serveKeys(ctx, w, r)
	case strings.HasPrefix(r.URL.Path, machinesPrefix):
		h.serveMachines(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) serveKeys(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	rr, err := parseRequest(r, genID())
	if err != nil {
		writeError(w, err)
		return
	}

	resp, err := h.Server.Do(ctx, rr)
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

	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
	}

	if err = writeEvent(w, ev); err != nil {
		// Should never be reached
		log.Println("error writing event: %v", err)
	}
}

// serveMachines responds address list in the format '0.0.0.0, 1.1.1.1'.
// TODO: rethink the format of machine list because it is not json format.
func (h Handler) serveMachines(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		allow(w, "GET", "HEAD")
		return
	}
	endpoints := h.Peers.Endpoints()
	w.Write([]byte(strings.Join(endpoints, ", ")))
}

func (h Handler) serveRaft(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("etcdhttp: error reading raft message:", err)
	}
	var m raftpb.Message
	if err := m.Unmarshal(b); err != nil {
		log.Println("etcdhttp: error unmarshaling raft message:", err)
	}
	log.Printf("etcdhttp: raft recv message from %#x: %+v", m.From, m)
	if err := h.Server.Node.Step(ctx, m); err != nil {
		log.Println("etcdhttp: error stepping raft messages:", err)
	}
}

// genID generates a random id that is: n < 0 < n.
func genID() int64 {
	for {
		b := make([]byte, 8)
		if _, err := io.ReadFull(crand.Reader, b); err != nil {
			panic(err) // really bad stuff happened
		}
		n := int64(binary.BigEndian.Uint64(b))
		if n != 0 {
			return n
		}
	}
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

	if ttl > 0 {
		expr := time.Duration(ttl) * time.Second
		// TODO(jonboulle): use fake clock instead of time module
		// https://github.com/coreos/etcd/issues/1021
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
	var writer io.Writer
	if w.Header().Get("Content-Encoding") == "gzip" {
		gzw := gzip.NewWriter(w)
		writer = gzw
		defer gzw.Close()
	} else {
		writer = w
	}

	return json.NewEncoder(writer).Encode(ev)
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

// allow writes response for the case that Method Not Allowed
func allow(w http.ResponseWriter, m ...string) {
	w.Header().Set("Allow", strings.Join(m, ","))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}
