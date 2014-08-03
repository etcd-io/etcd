/*
Copyright 2014 CoreOS Inc.

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

package etcd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
)

func (p *participant) GetHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v2/keys"):]
	// TODO(xiangli): handle consistent get
	recursive := (req.FormValue("recursive") == "true")
	sort := (req.FormValue("sorted") == "true")
	waitIndex := req.FormValue("waitIndex")
	stream := (req.FormValue("stream") == "true")
	if req.FormValue("quorum") == "true" {
		return p.handleQuorumGet(key, recursive, sort, w, req)
	}
	if req.FormValue("wait") == "true" {
		return p.handleWatch(key, recursive, stream, waitIndex, w, req)
	}
	return p.handleGet(key, recursive, sort, w, req)
}

func (p *participant) handleWatch(key string, recursive, stream bool, waitIndex string, w http.ResponseWriter, req *http.Request) error {
	// Create a command to watch from a given index (default 0).
	var sinceIndex uint64 = 0
	var err error

	if waitIndex != "" {
		sinceIndex, err = strconv.ParseUint(waitIndex, 10, 64)
		if err != nil {
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Watch From Index", p.Store.Index())
		}
	}

	watcher, err := p.Store.Watch(key, recursive, stream, sinceIndex)
	if err != nil {
		return err
	}

	cn, _ := w.(http.CloseNotifier)
	closeChan := cn.CloseNotify()

	p.writeHeaders(w)
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

func (p *participant) handleGet(key string, recursive, sort bool, w http.ResponseWriter, req *http.Request) error {
	event, err := p.Store.Get(key, recursive, sort)
	if err != nil {
		return err
	}
	p.writeHeaders(w)
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

func (p *participant) handleQuorumGet(key string, recursive, sort bool, w http.ResponseWriter, req *http.Request) error {
	if req.Method == "HEAD" {
		return fmt.Errorf("not support HEAD")
	}
	event, err := p.Get(key, recursive, sort)
	if err != nil {
		return err
	}
	p.handleRet(w, event)
	return nil
}

func (p *participant) writeHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Add("X-Etcd-Index", fmt.Sprint(p.Store.Index()))
	// TODO(xiangli): raft-index and term
	w.WriteHeader(http.StatusOK)
}
