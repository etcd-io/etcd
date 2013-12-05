package v2

import (
	"encoding/json"
	"fmt"
	"net/http"
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
		hostname, _ := s.ClientURL(leader)
		url := hostname + req.URL.Path
		log.Debugf("Redirect consistent get to %s", url)
		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
		return nil
	}

	recursive := (req.FormValue("recursive") == "true")
	sorted := (req.FormValue("sorted") == "true")

	//Stream trumps wait.
	if req.FormValue("stream") == "true" {
		return startEventStream(w, req, s, key, recursive)
	}

	if req.FormValue("wait") == "true" { // watch
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
		eventChan, err := s.Store().Watch(key, recursive, sinceIndex)
		if err != nil {
			return etcdErr.NewError(500, key, s.Store().Index())
		}

		cn, _ := w.(http.CloseNotifier)
		closeChan := cn.CloseNotify()

		select {
		case <-closeChan:
			return nil
		case event = <-eventChan:
		}

	} else { //get
		// Retrieve the key from the store.
		event, err = s.Store().Get(key, recursive, sorted)
		if err != nil {
			return err
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(s.Store().Index()))
	w.Header().Add("X-Raft-Index", fmt.Sprint(s.CommitIndex()))
	w.Header().Add("X-Raft-Term", fmt.Sprint(s.Term()))
	w.WriteHeader(http.StatusOK)
	b, _ := json.Marshal(event)

	w.Write(b)

	return nil
}

func startEventStream(w http.ResponseWriter, req *http.Request, s Server, key string, recursive bool) error {
	var err error

	f, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("Streaming Not Supported")
	}

	//Set some basic headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a command to watch from a given index (default 0).
	var sinceIndex uint64 = 0

	//Last-Event-ID header takes precedence over streamIndex Query param
	lastEventId := req.Header.Get("Last-Event-Id")
	if lastEventId == "" {
		lastEventId = req.FormValue("streamIndex")
	}
	if lastEventId != "" {
		sinceIndex, err = strconv.ParseUint(lastEventId, 10, 64)
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Stream From Index", s.Store().Index())
		}
	}

	//finish when the client disconnects.
	cn, _ := w.(http.CloseNotifier)
	closeChan := cn.CloseNotify()

	var event *store.Event
	var eventChan <-chan *store.Event
	// Set Content type header
	w.Header().Set("Content-Type", "text/event-stream")

	//For nginx to allow streaming
	w.Header().Set("X-Accel-Buffering", "no")

	//The main loop where, until the client disconnects, we stream all events they
	//are interested in. This is a loop here as the main "watcher" processes are
	//very much designed to be single use.
	for {
		eventChan, err = s.Store().Watch(key, recursive, sinceIndex)
		if err != nil {
			return etcdErr.NewError(500, key, s.Store().Index())
		}

		select {
		case <-closeChan:
			return nil
		case event = <-eventChan:
		}

		b, _ := json.Marshal(event)
		fmt.Fprintf(w, "id: %d\ndata: %s\n\n", event.Node.ModifiedIndex, b)
		f.Flush()

		sinceIndex = event.Node.ModifiedIndex + 1
	}

}
