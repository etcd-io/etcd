/*
Copyright 2013 CoreOS Inc.

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

package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/mod"
	"github.com/coreos/go-raft"
)

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via etcd url
//-------------------------------------------------------------------

func NewEtcdMuxer() *http.ServeMux {
	// external commands
	etcdMux := http.NewServeMux()
	etcdMux.Handle("/"+version+"/keys/", errorHandler(Multiplexer))
	etcdMux.Handle("/"+version+"/watch/", errorHandler(WatchHttpHandler))
	etcdMux.Handle("/"+version+"/leader", errorHandler(LeaderHttpHandler))
	etcdMux.Handle("/"+version+"/machines", errorHandler(MachinesHttpHandler))
	etcdMux.Handle("/"+version+"/stats/", errorHandler(StatsHttpHandler))
	etcdMux.Handle("/version", errorHandler(VersionHttpHandler))
	etcdMux.HandleFunc("/test/", TestHttpHandler)
	// TODO: Use a mux in 0.2 that can handle this
	etcdMux.Handle("/etcd/mod/dashboard/", *mod.ServeMux)
	return etcdMux
}

type errorHandler func(http.ResponseWriter, *http.Request) error

// addCorsHeader parses the request Origin header and loops through the user
// provided allowed origins and sets the Access-Control-Allow-Origin header if
// there is a match.
func addCorsHeader(w http.ResponseWriter, r *http.Request) {
	val, ok := corsList["*"]
	if val && ok {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		return
	}

	requestOrigin := r.Header.Get("Origin")
	val, ok = corsList[requestOrigin]
	if val && ok {
		w.Header().Add("Access-Control-Allow-Origin", requestOrigin)
		return
	}
}

func (fn errorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addCorsHeader(w, r)
	if e := fn(w, r); e != nil {
		if etcdErr, ok := e.(etcdErr.Error); ok {
			debug("Return error: ", etcdErr.Error())
			etcdErr.Write(w)
		} else {
			http.Error(w, e.Error(), http.StatusInternalServerError)
		}
	}
}

// Multiplex GET/POST/DELETE request to corresponding handlers
func Multiplexer(w http.ResponseWriter, req *http.Request) error {

	switch req.Method {
	case "GET":
		return GetHttpHandler(w, req)
	case "POST":
		return SetHttpHandler(w, req)
	case "PUT":
		return SetHttpHandler(w, req)
	case "DELETE":
		return DeleteHttpHandler(w, req)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

//--------------------------------------
// State sensitive handlers
// Set/Delete will dispatch to leader
//--------------------------------------

// Set Command Handler
func SetHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/keys/"):]

	if store.CheckKeyword(key) {
		return etcdErr.NewError(400, "Set")
	}

	debugf("[recv] POST %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	req.ParseForm()

	value := req.Form.Get("value")

	if len(value) == 0 {
		return etcdErr.NewError(200, "Set")
	}

	strDuration := req.Form.Get("ttl")

	expireTime, err := durationToExpireTime(strDuration)

	if err != nil {
		return etcdErr.NewError(202, "Set")
	}

	if prevValueArr, ok := req.Form["prevValue"]; ok && len(prevValueArr) > 0 {
		command := &TestAndSetCommand{
			Key:        key,
			Value:      value,
			PrevValue:  prevValueArr[0],
			ExpireTime: expireTime,
		}

		return dispatch(command, w, req, true)

	} else {
		command := &SetCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
		}

		return dispatch(command, w, req, true)
	}
}

// Delete Handler
func DeleteHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &DeleteCommand{
		Key: key,
	}

	return dispatch(command, w, req, true)
}

// Dispatch the command to leader
func dispatch(c Command, w http.ResponseWriter, req *http.Request, etcd bool) error {

	if r.State() == raft.Leader {
		if body, err := r.Do(c); err != nil {
			return err
		} else {
			if body == nil {
				return etcdErr.NewError(300, "Empty result from raft")
			} else {
				body, _ := body.([]byte)
				w.WriteHeader(http.StatusOK)
				w.Write(body)
				return nil
			}
		}

	} else {
		leader := r.Leader()
		// current no leader
		if leader == "" {
			return etcdErr.NewError(300, "")
		}

		redirect(leader, etcd, w, req)

		return nil
	}
	return etcdErr.NewError(300, "")
}

//--------------------------------------
// State non-sensitive handlers
// will not dispatch to leader
// TODO: add sensitive version for these
// command?
//--------------------------------------

// Handler to return the current leader's raft address
func LeaderHttpHandler(w http.ResponseWriter, req *http.Request) error {
	leader := r.Leader()

	if leader != "" {
		w.WriteHeader(http.StatusOK)
		raftURL, _ := nameToRaftURL(leader)
		w.Write([]byte(raftURL))
		return nil
	} else {
		return etcdErr.NewError(301, "")
	}
}

// Handler to return all the known machines in the current cluster
func MachinesHttpHandler(w http.ResponseWriter, req *http.Request) error {
	machines := getMachines(nameToEtcdURL)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(machines, ", ")))
	return nil
}

// Handler to return the current version of etcd
func VersionHttpHandler(w http.ResponseWriter, req *http.Request) error {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "etcd %s", releaseVersion)
	return nil
}

// Handler to return the basic stats of etcd
func StatsHttpHandler(w http.ResponseWriter, req *http.Request) error {
	option := req.URL.Path[len("/v1/stats/"):]

	switch option {
	case "self":
		w.WriteHeader(http.StatusOK)
		w.Write(r.Stats())
	case "leader":
		if r.State() == raft.Leader {
			w.Write(r.PeerStats())
		} else {
			leader := r.Leader()
			// current no leader
			if leader == "" {
				return etcdErr.NewError(300, "")
			}
			redirect(leader, true, w, req)
		}
	case "store":
		w.WriteHeader(http.StatusOK)
		w.Write(etcdStore.Stats())
	}

	return nil
}

// Get Handler
func GetHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] GET %s/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &GetCommand{
		Key: key,
	}

	if body, err := command.Apply(r.Server); err != nil {
		return err
	} else {
		body, _ := body.([]byte)
		w.WriteHeader(http.StatusOK)
		w.Write(body)

		return nil
	}

}

// Watch handler
func WatchHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/watch/"):]

	command := &WatchCommand{
		Key: key,
	}

	if req.Method == "GET" {
		debugf("[recv] GET %s/watch/%s [%s]", e.url, key, req.RemoteAddr)
		command.SinceIndex = 0

	} else if req.Method == "POST" {
		// watch from a specific index

		debugf("[recv] POST %s/watch/%s [%s]", e.url, key, req.RemoteAddr)
		content := req.FormValue("index")

		sinceIndex, err := strconv.ParseUint(string(content), 10, 64)
		if err != nil {
			return etcdErr.NewError(203, "Watch From Index")
		}
		command.SinceIndex = sinceIndex

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if body, err := command.Apply(r.Server); err != nil {
		return etcdErr.NewError(500, key)
	} else {
		w.WriteHeader(http.StatusOK)

		body, _ := body.([]byte)
		w.Write(body)
		return nil
	}

}

// TestHandler
func TestHttpHandler(w http.ResponseWriter, req *http.Request) {
	testType := req.URL.Path[len("/test/"):]

	if testType == "speed" {
		directSet()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("speed test success"))
		return
	}

	w.WriteHeader(http.StatusBadRequest)
}
