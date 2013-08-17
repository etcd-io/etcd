package main

import (
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"net/http"
	"strconv"
	"strings"
)

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via etcd url
//-------------------------------------------------------------------

func NewEtcdMuxer() *http.ServeMux {
	// external commands
	etcdMux := http.NewServeMux()
	etcdMux.Handle("/"+version+"/keys/", etcdHandler(Multiplexer))
	etcdMux.Handle("/"+version+"/watch/", etcdHandler(WatchHttpHandler))
	etcdMux.Handle("/leader", etcdHandler(LeaderHttpHandler))
	etcdMux.Handle("/machines", etcdHandler(MachinesHttpHandler))
	etcdMux.Handle("/version", etcdHandler(VersionHttpHandler))
	etcdMux.Handle("/stats", etcdHandler(StatsHttpHandler))
	etcdMux.HandleFunc("/test/", TestHttpHandler)
	return etcdMux
}

type etcdHandler func(http.ResponseWriter, *http.Request) *etcdError

func (fn etcdHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil {
		// 3xx is reft internal error
		if e.ErrorCode/100 == 3 {
			http.Error(w, string(e.toJson()), http.StatusInternalServerError)
		} else {
			http.Error(w, string(e.toJson()), http.StatusBadRequest)
		}
	}
}

// Multiplex GET/POST/DELETE request to corresponding handlers
func Multiplexer(w http.ResponseWriter, req *http.Request) *etcdError {

	switch req.Method {
	case "GET":
		return GetHttpHandler(w, req)
	case "POST":
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
func SetHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	key := req.URL.Path[len("/v1/keys/"):]

	if store.CheckKeyword(key) {
		return newEtcdError(400, "Set")
	}

	debugf("[recv] POST %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	value := req.FormValue("value")

	if len(value) == 0 {
		return newEtcdError(200, "Set")
	}

	prevValue := req.FormValue("prevValue")

	strDuration := req.FormValue("ttl")

	expireTime, err := durationToExpireTime(strDuration)

	if err != nil {
		return newEtcdError(202, "Set")
	}

	if len(prevValue) != 0 {
		command := &TestAndSetCommand{
			Key:        key,
			Value:      value,
			PrevValue:  prevValue,
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
func DeleteHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &DeleteCommand{
		Key: key,
	}

	return dispatch(command, w, req, true)
}

// Dispatch the command to leader
func dispatch(c Command, w http.ResponseWriter, req *http.Request, etcd bool) *etcdError {

	if r.State() == raft.Leader {
		if body, err := r.Do(c); err != nil {

			// store error
			if _, ok := err.(store.NotFoundError); ok {
				return newEtcdError(100, err.Error())
			}

			if _, ok := err.(store.TestFail); ok {
				return newEtcdError(101, err.Error())
			}

			if _, ok := err.(store.NotFile); ok {
				return newEtcdError(102, err.Error())
			}

			// join error
			if err.Error() == errors[103] {
				return newEtcdError(103, "")
			}

			// raft internal error
			return newEtcdError(300, err.Error())

		} else {
			if body == nil {
				return newEtcdError(300, "Empty result from raft")
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
			return newEtcdError(300, "")
		}

		// tell the client where is the leader
		path := req.URL.Path

		var url string

		if etcd {
			etcdAddr, _ := nameToEtcdURL(leader)
			url = etcdAddr + path
		} else {
			raftAddr, _ := nameToRaftURL(leader)
			url = raftAddr + path
		}

		debugf("Redirect to %s", url)

		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
		return nil
	}
	return newEtcdError(300, "")
}

//--------------------------------------
// State non-sensitive handlers
// will not dispatch to leader
// TODO: add sensitive version for these
// command?
//--------------------------------------

// Handler to return the current leader's raft address
func LeaderHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	leader := r.Leader()

	if leader != "" {
		w.WriteHeader(http.StatusOK)
		raftURL, _ := nameToRaftURL(leader)
		w.Write([]byte(raftURL))
		return nil
	} else {
		return newEtcdError(301, "")
	}
}

// Handler to return all the known machines in the current cluster
func MachinesHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	machines := getMachines()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(machines, ", ")))
	return nil
}

// Handler to return the current version of etcd
func VersionHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "etcd %s", releaseVersion)
	fmt.Fprintf(w, "etcd API %s", version)
	return nil
}

// Handler to return the basic stats of etcd
func StatsHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	w.WriteHeader(http.StatusOK)
	w.Write(etcdStore.Stats())
	return nil
}

// Get Handler
func GetHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] GET %s/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &GetCommand{
		Key: key,
	}

	if body, err := command.Apply(r.Server); err != nil {

		if _, ok := err.(store.NotFoundError); ok {
			return newEtcdError(100, err.Error())
		}

		return newEtcdError(300, "")

	} else {
		body, _ := body.([]byte)
		w.WriteHeader(http.StatusOK)
		w.Write(body)

		return nil
	}

}

// Watch handler
func WatchHttpHandler(w http.ResponseWriter, req *http.Request) *etcdError {
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
			return newEtcdError(203, "Watch From Index")
		}
		command.SinceIndex = sinceIndex

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if body, err := command.Apply(r.Server); err != nil {
		return newEtcdError(500, key)
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
