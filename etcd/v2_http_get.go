package etcd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
)

func (s *Server) GetHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v2/keys"):]
	// TODO(xiangli): handle consistent get
	recursive := (req.FormValue("recursive") == "true")
	sort := (req.FormValue("sorted") == "true")
	waitIndex := req.FormValue("waitIndex")
	stream := (req.FormValue("stream") == "true")
	if req.FormValue("wait") == "true" {
		return s.handleWatch(key, recursive, stream, waitIndex, w, req)
	}
	return s.handleGet(key, recursive, sort, w, req)
}

func (s *Server) handleWatch(key string, recursive, stream bool, waitIndex string, w http.ResponseWriter, req *http.Request) error {
	// Create a command to watch from a given index (default 0).
	var sinceIndex uint64 = 0
	var err error

	if waitIndex != "" {
		sinceIndex, err = strconv.ParseUint(waitIndex, 10, 64)
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Watch From Index", s.Store.Index())
		}
	}

	watcher, err := s.Store.Watch(key, recursive, stream, sinceIndex)
	if err != nil {
		return err
	}

	cn, _ := w.(http.CloseNotifier)
	closeChan := cn.CloseNotify()

	s.writeHeaders(w)

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

func (s *Server) handleGet(key string, recursive, sort bool, w http.ResponseWriter, req *http.Request) error {
	event, err := s.Store.Get(key, recursive, sort)
	if err != nil {
		return err
	}
	s.writeHeaders(w)
	if req.Method == "HEAD" {
		return nil
	}
	b, err := json.Marshal(event)
	if err != nil {
		panic(fmt.Sprintf("handleGet: ", err))
	}
	w.Write(b)
	return nil
}

func (s *Server) writeHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(s.Store.Index()))
	// TODO(xiangli): raft-index and term
	w.WriteHeader(http.StatusOK)
}
