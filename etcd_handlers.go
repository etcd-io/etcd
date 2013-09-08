package main

import (
	"fmt"
	etcdErr "github.com/coreos/etcd/error"
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
	etcdMux.Handle("/"+version+"/keys/", errorHandler(Multiplexer))
	etcdMux.Handle("/"+version+"/watch/", errorHandler(WatchHttpHandler))
	etcdMux.Handle("/"+version+"/leader", errorHandler(LeaderHttpHandler))
	etcdMux.Handle("/"+version+"/machines", errorHandler(MachinesHttpHandler))
	etcdMux.Handle("/"+version+"/stats", errorHandler(StatsHttpHandler))
	etcdMux.Handle("/version", errorHandler(VersionHttpHandler))
	etcdMux.HandleFunc("/test/", TestHttpHandler)
	return etcdMux
}

type errorHandler func(http.ResponseWriter, *http.Request) error

func (fn errorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		return etcdErr.NewError(etcdErr.EcodeKeyIsPreserved, "Set")
	}

	debugf("[recv] POST %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	value := req.FormValue("value")

	if len(value) == 0 {
		return etcdErr.NewError(etcdErr.EcodeValueRequired, "Set")
	}

	prevValue := req.FormValue("prevValue")

	strDuration := req.FormValue("ttl")

	expireTime, err := durationToExpireTime(strDuration)

	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Set")
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
				return etcdErr.NewError(etcdErr.EcodeRaftInternal, "Empty result from raft")
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
			return etcdErr.NewError(etcdErr.EcodeRaftInternal, "")
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
	return etcdErr.NewError(etcdErr.EcodeRaftInternal, "")
}

//--------------------------------------
// State non-sensitive handlers
// will not dispatch to leader
// TODO: add sensitive version for these
// command?
//--------------------------------------

// Handler to return the current leader's raft address
// func LeaderHttpHandler(w http.ResponseWriter, req *http.Request) error {
// 	leader := r.Leader()

// 	if leader != "" {
// 		w.WriteHeader(http.StatusOK)
// 		raftURL, _ := nameToRaftURL(leader)
// 		w.Write([]byte(raftURL))
// 		return nil
// 	} else {
// 		return etcdErr.NewError(etcdErr.EcodeLeaderElect, "")
// 	}
// }

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
	w.WriteHeader(http.StatusOK)
	w.Write(etcdStore.Stats())
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
			return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Watch From Index")
		}
		command.SinceIndex = sinceIndex

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if body, err := command.Apply(r.Server); err != nil {
		return etcdErr.NewError(etcdErr.EcodeWatcherCleared, key)
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
