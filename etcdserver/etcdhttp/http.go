/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/jonboulle/clockwork"
	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/store"
)

const (
	keysPrefix               = "/v2/keys"
	deprecatedMachinesPrefix = "/v2/machines"
	adminMembersPrefix       = "/v2/admin/members/"
	raftPrefix               = "/raft"
	statsPrefix              = "/v2/stats"

	// time to wait for response from EtcdServer requests
	defaultServerTimeout = 500 * time.Millisecond

	// time to wait for a Watch request
	defaultWatchTimeout = 5 * time.Minute
)

var errClosed = errors.New("etcdhttp: client closed connection")

// NewClientHandler generates a muxed http.Handler with the given parameters to serve etcd client requests.
func NewClientHandler(server *etcdserver.EtcdServer) http.Handler {
	sh := &serverHandler{
		server:       server,
		clusterStore: server.ClusterStore,
		stats:        server,
		timer:        server,
		timeout:      defaultServerTimeout,
		clock:        clockwork.NewRealClock(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(keysPrefix, sh.serveKeys)
	mux.HandleFunc(keysPrefix+"/", sh.serveKeys)
	mux.HandleFunc(statsPrefix+"/store", sh.serveStoreStats)
	mux.HandleFunc(statsPrefix+"/self", sh.serveSelfStats)
	mux.HandleFunc(statsPrefix+"/leader", sh.serveLeaderStats)
	// TODO: dynamic configuration may make this outdated. take care of it.
	// TODO: dynamic configuration may introduce race also.
	// TODO: add serveMembers
	mux.HandleFunc(deprecatedMachinesPrefix, sh.serveMachines)
	mux.HandleFunc(adminMembersPrefix, sh.serveAdminMembers)
	mux.HandleFunc("/", http.NotFound)
	return mux
}

// NewPeerHandler generates an http.Handler to handle etcd peer (raft) requests.
func NewPeerHandler(server *etcdserver.EtcdServer) http.Handler {
	sh := &serverHandler{
		server:       server,
		stats:        server,
		clusterStore: server.ClusterStore,
		clock:        clockwork.NewRealClock(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(raftPrefix, sh.serveRaft)
	mux.HandleFunc("/", http.NotFound)
	return mux
}

// serverHandler provides http.Handlers for etcd client and raft communication.
type serverHandler struct {
	timeout      time.Duration
	server       etcdserver.Server
	stats        etcdserver.Stats
	timer        etcdserver.RaftTimer
	clusterStore etcdserver.ClusterStore
	clock        clockwork.Clock
}

func (h serverHandler) serveKeys(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET", "PUT", "POST", "DELETE") {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	rr, err := parseKeyRequest(r, etcdserver.GenID(), clockwork.NewRealClock())
	if err != nil {
		writeError(w, err)
		return
	}

	resp, err := h.server.Do(ctx, rr)
	if err != nil {
		writeError(w, err)
		return
	}

	switch {
	case resp.Event != nil:
		if err := writeKeyEvent(w, resp.Event, h.timer); err != nil {
			// Should never be reached
			log.Printf("error writing event: %v", err)
		}
	case resp.Watcher != nil:
		ctx, cancel := context.WithTimeout(context.Background(), defaultWatchTimeout)
		defer cancel()
		handleKeyWatch(ctx, w, resp.Watcher, rr.Stream, h.timer)
	default:
		writeError(w, errors.New("received response with no Event/Watcher!"))
	}
}

// serveMachines responds address list in the format '0.0.0.0, 1.1.1.1'.
func (h serverHandler) serveMachines(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET", "HEAD") {
		return
	}
	endpoints := h.clusterStore.Get().ClientURLs()
	w.Write([]byte(strings.Join(endpoints, ", ")))
}

func (h serverHandler) serveAdminMembers(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "POST", "DELETE") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultServerTimeout)
	defer cancel()

	switch r.Method {
	case "POST":
		ctype := r.Header.Get("Content-Type")
		if ctype != "application/json" {
			http.Error(w, fmt.Sprintf("bad Content-Type %s, accept application/json", ctype), http.StatusBadRequest)
			return
		}
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		raftAttr := etcdserver.RaftAttributes{}
		if err := json.Unmarshal(b, &raftAttr); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		validURLs, err := types.NewURLs(raftAttr.PeerURLs)
		if err != nil {
			http.Error(w, "bad peer urls", http.StatusBadRequest)
			return
		}
		now := h.clock.Now()
		m := etcdserver.NewMember("", validURLs, "", &now)
		if err := h.server.AddMember(ctx, *m); err != nil {
			log.Printf("etcdhttp: error adding node %x: %v", m.ID, err)
			writeError(w, err)
			return
		}
		log.Printf("etcdhttp: added node %x with peer urls %v", m.ID, raftAttr.PeerURLs)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(m); err != nil {
			log.Printf("etcdhttp: %v", err)
		}
	case "DELETE":
		idStr := strings.TrimPrefix(r.URL.Path, adminMembersPrefix)
		id, err := strconv.ParseUint(idStr, 16, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("etcdhttp: remove node %x", id)
		if err := h.server.RemoveMember(ctx, id); err != nil {
			log.Printf("etcdhttp: error removing node %x: %v", id, err)
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h serverHandler) serveStoreStats(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET") {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(h.stats.StoreStats())
}

func (h serverHandler) serveSelfStats(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET") {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(h.stats.SelfStats())
}

func (h serverHandler) serveLeaderStats(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "GET") {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(h.stats.LeaderStats())
}

func (h serverHandler) serveRaft(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r.Method, "POST") {
		return
	}

	wcid := strconv.FormatUint(h.clusterStore.Get().ID(), 16)
	w.Header().Set("X-Etcd-Cluster-ID", wcid)

	gcid := r.Header.Get("X-Etcd-Cluster-ID")
	if gcid != wcid {
		log.Printf("etcdhttp: request ignored due to cluster ID mismatch got %s want %s", gcid, wcid)
		http.Error(w, "clusterID mismatch", http.StatusPreconditionFailed)
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
		switch err {
		case etcdserver.ErrRemoved:
			http.Error(w, "cannot process message from removed node", http.StatusForbidden)
		default:
			writeError(w, err)
		}
		return
	}
	if m.Type == raftpb.MsgApp {
		h.stats.UpdateRecvApp(m.From, r.ContentLength)
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseKeyRequest converts a received http.Request on keysPrefix to
// a server Request, performing validation of supplied fields as appropriate.
// If any validation fails, an empty Request and non-nil error is returned.
func parseKeyRequest(r *http.Request, id uint64, clock clockwork.Clock) (etcdserverpb.Request, error) {
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
	p := path.Join(etcdserver.StoreKeysPrefix, r.URL.Path[len(keysPrefix):])

	var pIdx, wIdx uint64
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

	var rec, sort, wait, dir, stream bool
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
	// TODO(jonboulle): define what parameters dir is/isn't compatible with?
	if dir, err = getBool(r.Form, "dir"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`invalid value for "dir"`,
		)
	}
	if stream, err = getBool(r.Form, "stream"); err != nil {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`invalid value for "stream"`,
		)
	}

	if wait && r.Method != "GET" {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`"wait" can only be used with GET requests`,
		)
	}

	pV := r.FormValue("prevValue")
	if _, ok := r.Form["prevValue"]; ok && pV == "" {
		return emptyReq, etcdErr.NewRequestError(
			etcdErr.EcodeInvalidField,
			`"prevValue" cannot be empty`,
		)
	}

	// TTL is nullable, so leave it null if not specified
	// or an empty string
	var ttl *uint64
	if len(r.FormValue("ttl")) > 0 {
		i, err := getUint64(r.Form, "ttl")
		if err != nil {
			return emptyReq, etcdErr.NewRequestError(
				etcdErr.EcodeTTLNaN,
				`invalid value for "ttl"`,
			)
		}
		ttl = &i
	}

	// prevExist is nullable, so leave it null if not specified
	var pe *bool
	if _, ok := r.Form["prevExist"]; ok {
		bv, err := getBool(r.Form, "prevExist")
		if err != nil {
			return emptyReq, etcdErr.NewRequestError(
				etcdErr.EcodeInvalidField,
				"invalid value for prevExist",
			)
		}
		pe = &bv
	}

	rr := etcdserverpb.Request{
		ID:        id,
		Method:    r.Method,
		Path:      p,
		Val:       r.FormValue("value"),
		Dir:       dir,
		PrevValue: pV,
		PrevIndex: pIdx,
		PrevExist: pe,
		Recursive: rec,
		Since:     wIdx,
		Sorted:    sort,
		Stream:    stream,
		Wait:      wait,
	}

	if pe != nil {
		rr.PrevExist = pe
	}

	// Null TTL is equivalent to unset Expiration
	if ttl != nil {
		expr := time.Duration(*ttl) * time.Second
		rr.Expiration = clock.Now().Add(expr).UnixNano()
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

// writeKeyEvent trims the prefix of key path in a single Event under
// StoreKeysPrefix, serializes it and writes the resulting JSON to the given
// ResponseWriter, along with the appropriate headers.
func writeKeyEvent(w http.ResponseWriter, ev *store.Event, rt etcdserver.RaftTimer) error {
	if ev == nil {
		return errors.New("cannot write empty Event!")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Etcd-Index", fmt.Sprint(ev.EtcdIndex))
	w.Header().Set("X-Raft-Index", fmt.Sprint(rt.Index()))
	w.Header().Set("X-Raft-Term", fmt.Sprint(rt.Term()))

	if ev.IsCreated() {
		w.WriteHeader(http.StatusCreated)
	}

	ev = trimEventPrefix(ev, etcdserver.StoreKeysPrefix)
	return json.NewEncoder(w).Encode(ev)
}

func handleKeyWatch(ctx context.Context, w http.ResponseWriter, wa store.Watcher, stream bool, rt etcdserver.RaftTimer) {
	defer wa.Remove()
	ech := wa.EventChan()
	var nch <-chan bool
	if x, ok := w.(http.CloseNotifier); ok {
		nch = x.CloseNotify()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Etcd-Index", fmt.Sprint(wa.StartIndex()))
	w.Header().Set("X-Raft-Index", fmt.Sprint(rt.Index()))
	w.Header().Set("X-Raft-Term", fmt.Sprint(rt.Term()))
	w.WriteHeader(http.StatusOK)

	// Ensure headers are flushed early, in case of long polling
	w.(http.Flusher).Flush()

	for {
		select {
		case <-nch:
			// Client closed connection. Nothing to do.
			return
		case <-ctx.Done():
			// Timed out. net/http will close the connection for us, so nothing to do.
			return
		case ev, ok := <-ech:
			if !ok {
				// If the channel is closed this may be an indication of
				// that notifications are much more than we are able to
				// send to the client in time. Then we simply end streaming.
				return
			}
			ev = trimEventPrefix(ev, etcdserver.StoreKeysPrefix)
			if err := json.NewEncoder(w).Encode(ev); err != nil {
				// Should never be reached
				log.Printf("error writing event: %v\n", err)
				return
			}
			if !stream {
				return
			}
			w.(http.Flusher).Flush()
		}
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

func trimEventPrefix(ev *store.Event, prefix string) *store.Event {
	if ev == nil {
		return nil
	}
	ev.Node = trimNodeExternPrefix(ev.Node, prefix)
	ev.PrevNode = trimNodeExternPrefix(ev.PrevNode, prefix)
	return ev
}

func trimNodeExternPrefix(n *store.NodeExtern, prefix string) *store.NodeExtern {
	if n == nil {
		return nil
	}
	n.Key = strings.TrimPrefix(n.Key, prefix)
	for _, nn := range n.Nodes {
		nn = trimNodeExternPrefix(nn, prefix)
	}
	return n
}
