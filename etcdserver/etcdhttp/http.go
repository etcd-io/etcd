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
	"strconv"
	"strings"
	"time"

	crand "crypto/rand"
	"math/rand"

	"github.com/coreos/etcd/elog"
	etcderrors "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

type Peers map[int64][]string

func (ps Peers) Pick(id int64) string {
	addrs := ps[id]
	if len(addrs) == 0 {
		return ""
	}
	return fmt.Sprintf("http://%s", addrs[rand.Intn(len(addrs))])
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
	return "todo"
}

func (ps Peers) Ids() []int64 {
	var ids []int64
	for id, _ := range ps {
		ids = append(ids, id)
	}
	return ids
}

var errClosed = errors.New("etcdhttp: client closed connection")

const DefaultTimeout = 500 * time.Millisecond

func Sender(p Peers) func(msgs []raftpb.Message) {
	return func(msgs []raftpb.Message) {
		for _, m := range msgs {
			// TODO: create workers that deal with message sending
			// concurrently as to not block progress
			for {
				url := p.Pick(m.To)
				if url == "" {
					// TODO: unknown peer id.. what do we do? I
					// don't think his should ever happen, need to
					// look into this further.
					log.Println("etcdhttp: no addr for %d", m.To)
					break
				}

				url += "/raft"

				// TODO: don't block. we should be able to have 1000s
				// of messages out at a time.
				data, err := m.Marshal()
				if err != nil {
					log.Println("etcdhttp: dropping message:", err)
					break // drop bad message
				}
				if httpPost(url, data) {
					break // success
				}

				// TODO: backoff
			}
		}
	}
}

func httpPost(url string, data []byte) bool {
	// TODO: set timeouts
	resp, err := http.Post(url, "application/protobuf", bytes.NewBuffer(data))
	if err != nil {
		elog.TODO()
		return false
	}
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
	case strings.HasPrefix(r.URL.Path, "/v2/keys/"):
		h.serveKeys(ctx, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) serveKeys(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	rr, err := parseRequest(r)
	if err != nil {
		log.Println(err) // reading of body failed
		return
	}

	resp, err := h.Server.Do(ctx, rr)
	switch e := err.(type) {
	case nil:
	case *etcderrors.Error:
		// TODO: gross. this should be handled in encodeResponse
		log.Println(err)
		e.Write(w)
		return
	default:
		log.Println(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	if err := encodeResponse(ctx, w, resp); err != nil {
		http.Error(w, "Timeout while waiting for response", 504)
		return
	}
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

// genId generates a random id that is: n < 0 < n.
func genId() int64 {
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

func parseRequest(r *http.Request) (etcdserverpb.Request, error) {
	if err := r.ParseForm(); err != nil {
		return etcdserverpb.Request{}, err
	}

	q := r.URL.Query()
	rr := etcdserverpb.Request{
		Id:        genId(),
		Method:    r.Method,
		Val:       r.FormValue("value"),
		Path:      r.URL.Path[len("/v2/keys"):],
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

	return rr, nil
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
