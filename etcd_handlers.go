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
	etcdMux.HandleFunc("/"+version+"/keys/", Multiplexer)
	etcdMux.HandleFunc("/"+version+"/watch/", WatchHttpHandler)
	etcdMux.HandleFunc("/leader", LeaderHttpHandler)
	etcdMux.HandleFunc("/machines", MachinesHttpHandler)
	etcdMux.HandleFunc("/", VersionHttpHandler)
	etcdMux.HandleFunc("/stats", StatsHttpHandler)
	etcdMux.HandleFunc("/test/", TestHttpHandler)
	return etcdMux
}

// Multiplex GET/POST/DELETE request to corresponding handlers
func Multiplexer(w http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case "GET":
		GetHttpHandler(&w, req)
	case "POST":
		SetHttpHandler(&w, req)
	case "DELETE":
		DeleteHttpHandler(&w, req)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

//--------------------------------------
// State sensitive handlers
// Set/Delete will dispatch to leader
//--------------------------------------

// Set Command Handler
func SetHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	if store.CheckKeyword(key) {

		(*w).WriteHeader(http.StatusBadRequest)

		(*w).Write(newJsonError(400, "Set"))
		return
	}

	debugf("[recv] POST %v/v1/keys/%s [%s]", info.EtcdURL, key, req.RemoteAddr)

	value := req.FormValue("value")

	if len(value) == 0 {
		(*w).WriteHeader(http.StatusBadRequest)

		(*w).Write(newJsonError(200, "Set"))
		return
	}

	prevValue := req.FormValue("prevValue")

	strDuration := req.FormValue("ttl")

	expireTime, err := durationToExpireTime(strDuration)

	if err != nil {

		(*w).WriteHeader(http.StatusBadRequest)

		(*w).Write(newJsonError(202, "Set"))
		return
	}

	if len(prevValue) != 0 {
		command := &TestAndSetCommand{
			Key:        key,
			Value:      value,
			PrevValue:  prevValue,
			ExpireTime: expireTime,
		}

		dispatch(command, w, req, true)

	} else {
		command := &SetCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
		}

		dispatch(command, w, req, true)
	}

}

// Delete Handler
func DeleteHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE %v/v1/keys/%s [%s]", info.EtcdURL, key, req.RemoteAddr)

	command := &DeleteCommand{
		Key: key,
	}

	dispatch(command, w, req, true)
}

// Dispatch the command to leader
func dispatch(c Command, w *http.ResponseWriter, req *http.Request, etcd bool) {

	if raftServer.State() == raft.Leader {
		if body, err := raftServer.Do(c); err != nil {

			if _, ok := err.(store.NotFoundError); ok {
				(*w).WriteHeader(http.StatusNotFound)
				(*w).Write(newJsonError(100, err.Error()))
				return
			}

			if _, ok := err.(store.TestFail); ok {
				(*w).WriteHeader(http.StatusBadRequest)
				(*w).Write(newJsonError(101, err.Error()))
				return
			}

			if _, ok := err.(store.NotFile); ok {
				(*w).WriteHeader(http.StatusBadRequest)
				(*w).Write(newJsonError(102, err.Error()))
				return
			}
			if err.Error() == errors[103] {
				(*w).WriteHeader(http.StatusBadRequest)
				(*w).Write(newJsonError(103, ""))
				return
			}
			(*w).WriteHeader(http.StatusInternalServerError)
			(*w).Write(newJsonError(300, err.Error()))
			return
		} else {

			if body == nil {
				(*w).WriteHeader(http.StatusNotFound)
				(*w).Write(newJsonError(300, "Empty result from raft"))
			} else {
				body, ok := body.([]byte)
				// this should not happen
				if !ok {
					panic("wrong type")
				}
				(*w).WriteHeader(http.StatusOK)
				(*w).Write(body)
			}
			return
		}
	} else {
		leader := raftServer.Leader()
		// current no leader
		if leader == "" {
			(*w).WriteHeader(http.StatusInternalServerError)
			(*w).Write(newJsonError(300, ""))
			return
		}

		// tell the client where is the leader

		path := req.URL.Path

		var url string

		if etcd {
			etcdAddr, _ := nameToEtcdURL(leader)
			if etcdAddr == "" {
				panic(leader)
			}
			url = etcdAddr + path
		} else {
			raftAddr, _ := nameToRaftURL(leader)
			url = raftAddr + path
		}

		debugf("Redirect to %s", url)

		http.Redirect(*w, req, url, http.StatusTemporaryRedirect)
		return
	}
	(*w).WriteHeader(http.StatusInternalServerError)
	(*w).Write(newJsonError(300, ""))
	return
}

//--------------------------------------
// State non-sensitive handlers
// will not dispatch to leader
// TODO: add sensitive version for these
// command?
//--------------------------------------

// Handler to return the current leader's raft address
func LeaderHttpHandler(w http.ResponseWriter, req *http.Request) {
	leader := raftServer.Leader()

	if leader != "" {
		w.WriteHeader(http.StatusOK)
		raftURL, _ := nameToRaftURL(leader)
		w.Write([]byte(raftURL))
	} else {

		// not likely, but it may happen
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(newJsonError(301, ""))
	}
}

// Handler to return all the known machines in the current cluster
func MachinesHttpHandler(w http.ResponseWriter, req *http.Request) {
	machines := getMachines()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.Join(machines, ", ")))
}

// Handler to return the current version of etcd
func VersionHttpHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("etcd %s", releaseVersion)))
}

// Handler to return the basic stats of etcd
func StatsHttpHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write(etcdStore.Stats())
}

// Get Handler
func GetHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] GET %s/v1/keys/%s [%s]", info.EtcdURL, key, req.RemoteAddr)

	command := &GetCommand{
		Key: key,
	}

	if body, err := command.Apply(raftServer); err != nil {

		if _, ok := err.(store.NotFoundError); ok {
			(*w).WriteHeader(http.StatusNotFound)
			(*w).Write(newJsonError(100, err.Error()))
			return
		}

		(*w).WriteHeader(http.StatusInternalServerError)
		(*w).Write(newJsonError(300, ""))

	} else {
		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		(*w).WriteHeader(http.StatusOK)
		(*w).Write(body)

	}

}

// Watch handler
func WatchHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/watch/"):]

	command := &WatchCommand{
		Key: key,
	}

	if req.Method == "GET" {
		debugf("[recv] GET %s/watch/%s [%s]", info.EtcdURL, key, req.RemoteAddr)
		command.SinceIndex = 0

	} else if req.Method == "POST" {
		// watch from a specific index

		debugf("[recv] POST %s/watch/%s [%s]", info.EtcdURL, key, req.RemoteAddr)
		content := req.FormValue("index")

		sinceIndex, err := strconv.ParseUint(string(content), 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write(newJsonError(203, "Watch From Index"))
		}
		command.SinceIndex = sinceIndex

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if body, err := command.Apply(raftServer); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(newJsonError(500, key))
	} else {
		w.WriteHeader(http.StatusOK)

		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		w.Write(body)
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
