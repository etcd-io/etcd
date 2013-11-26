package v2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
	"github.com/gorilla/mux"
)

func GetHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	var err error
	var event *store.Event

	vars := mux.Vars(req)
	key := "/" + vars["key"]

	// Help client to redirect the request to the current leader
	if req.FormValue("consistent") == "true" && s.State() != raft.Leader {
		leader := s.Leader()
		hostname, _ := s.PeerURL(leader)
		url := hostname + req.URL.Path
		log.Debugf("Redirect consistent get to %s", url)
		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
		return nil
	}

	wait := req.FormValue("wait") == "true"
	recursive := (req.FormValue("recursive") == "true")
	stream := (req.FormValue("stream") == "true")
	sorted := (req.FormValue("sorted") == "true")

	if wait { // watch
		// Create a command to watch from a given index (default 0).
		var sinceIndex uint64 = 0

		waitIndex := req.FormValue("waitIndex")
		if waitIndex != "" {
			sinceIndex, err = strconv.ParseUint(string(req.FormValue("waitIndex")), 10, 64)
			if err != nil {
				return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Watch From Index", s.Store().Index())
			}
		}

		// Start the watcher on the store.
		watcher, err := s.Store().Watch(key, recursive, stream, sinceIndex)
		if err != nil {
			return etcdErr.NewError(500, key, s.Store().Index())
		}
		defer watcher.Cancel()

		cn := w.(http.CloseNotifier)
		closeChan := cn.CloseNotify()

		writeHeaders(w, s)

		if stream {
			chunkW := httputil.NewChunkedWriter(w)
			for {
				select {
				case <-closeChan:
					chunkW.Close()
					return nil
				case event = <-watcher.EventChan:
					b, _ := json.Marshal(event)
					chunkW.Write(b)
					w.(http.Flusher).Flush()
				}
			}

		} else { // single event
			select {
			case <-closeChan:
				return nil
			case event = <-watcher.EventChan:
				b, _ := json.Marshal(event)
				w.Write(b)
			}
		}

	} else { // get
		// Retrieve the key from the store.
		event, err = s.Store().Get(key, recursive, sorted)
		if err != nil {
			return err
		}

		writeHeaders(w, s)
		b, _ := json.Marshal(event)
		w.Write(b)
	}

	return nil
}

func writeHeaders(w http.ResponseWriter, s Server) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(s.Store().Index()))
	w.Header().Add("X-Raft-Index", fmt.Sprint(s.CommitIndex()))
	w.Header().Add("X-Raft-Term", fmt.Sprint(s.Term()))
	w.WriteHeader(http.StatusOK)
}
