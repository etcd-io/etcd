package v2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

func GetHandler(w http.ResponseWriter, req *http.Request, s Server) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	recursive := (req.FormValue("recursive") == "true")
	sort := (req.FormValue("sorted") == "true")

	if req.FormValue("quorum") == "true" {
		c := s.Store().CommandFactory().CreateGetCommand(key, recursive, sort)
		return s.Dispatch(c, w, req)
	}

	// Help client to redirect the request to the current leader
	if req.FormValue("consistent") == "true" && s.State() != raft.Leader {
		leader := s.Leader()
		hostname, _ := s.ClientURL(leader)

		url, err := url.Parse(hostname)
		if err != nil {
			log.Warn("Redirect cannot parse hostName ", hostname)
			return err
		}
		url.RawQuery = req.URL.RawQuery
		url.Path = req.URL.Path

		log.Debugf("Redirect consistent get to %s", url.String())
		http.Redirect(w, req, url.String(), http.StatusTemporaryRedirect)
		return nil
	}

	waitIndex := req.FormValue("waitIndex")
	stream := (req.FormValue("stream") == "true")

	if req.FormValue("wait") == "true" {
		return handleWatch(key, recursive, stream, waitIndex, w, req, s)
	}

	return handleGet(key, recursive, sort, w, req, s)
}

func handleWatch(key string, recursive, stream bool, waitIndex string, w http.ResponseWriter, req *http.Request, s Server) error {
	// Create a command to watch from a given index (default 0).
	var sinceIndex uint64 = 0
	var err error

	if waitIndex != "" {
		sinceIndex, err = strconv.ParseUint(waitIndex, 10, 64)
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Watch From Index", s.Store().Index())
		}
	}

	watcher, err := s.Store().Watch(key, recursive, stream, sinceIndex)
	if err != nil {
		return err
	}

	cn, _ := w.(http.CloseNotifier)
	closeChan := cn.CloseNotify()

	writeHeaders(w, s)
	w.(http.Flusher).Flush()

	if stream {
		// watcher hub will not help to remove stream watcher
		// so we need to remove here
		defer watcher.Remove()
		for {
			select {
			case <-closeChan:
				return nil
			case event, ok := <-watcher.EventChan:
				if !ok {
					// If the channel is closed this may be an indication of
					// that notifications are much more than we are able to
					// send to the client in time. Then we simply end streaming.
					return nil
				}
				if req.Method == "HEAD" {
					continue
				}

				b, _ := json.Marshal(event)
				_, err := w.Write(b)
				if err != nil {
					return nil
				}
				w.(http.Flusher).Flush()
			}
		}
	}

	select {
	case <-closeChan:
		watcher.Remove()
	case event := <-watcher.EventChan:
		if req.Method == "HEAD" {
			return nil
		}
		b, _ := json.Marshal(event)
		w.Write(b)
	}
	return nil
}

func handleGet(key string, recursive, sort bool, w http.ResponseWriter, req *http.Request, s Server) error {
	event, err := s.Store().Get(key, recursive, sort)
	if err != nil {
		return err
	}

	if req.Method == "HEAD" {
		return nil
	}

	writeHeaders(w, s)
	b, _ := json.Marshal(event)
	w.Write(b)
	return nil
}

func writeHeaders(w http.ResponseWriter, s Server) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(s.Store().Index()))
	w.Header().Add("X-Raft-Index", fmt.Sprint(s.CommitIndex()))
	w.Header().Add("X-Raft-Term", fmt.Sprint(s.Term()))
	w.WriteHeader(http.StatusOK)
}
