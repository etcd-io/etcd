package main

import (
	"github.com/coreos/etcd/store"
	"net/http"
	"strconv"
	"fmt"
	"time"
)

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via raft client port
//-------------------------------------------------------------------

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

	debugf("[recv] POST http://%v/v1/keys/%s", raftServer.Name(), key)

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
			Key: key,
			Value: value,
			PrevValue: prevValue,
			ExpireTime: expireTime,
		}

		dispatch(command, w, req, true)

	} else {
		command := &SetCommand{
			Key: key,
			Value: value,
			ExpireTime: expireTime,
		}

		dispatch(command, w, req, true)
	}

}

// Delete Handler
func DeleteHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE http://%v/v1/keys/%s", raftServer.Name(), key)

	command := &DeleteCommand{
		Key: key,
	}

	dispatch(command, w, req, true)
}

// Dispatch the command to leader
func dispatch(c Command, w *http.ResponseWriter, req *http.Request, client bool) {
	if raftServer.State() == "leader" {
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
		// current no leader
		if raftServer.Leader() == "" {
			(*w).WriteHeader(http.StatusInternalServerError)
			(*w).Write(newJsonError(300, ""))
			return
		}

		// tell the client where is the leader

		path := req.URL.Path

		var scheme string

		if scheme = req.URL.Scheme; scheme == "" {
			scheme = "http://"
		}

		var url string

		if client {
			clientAddr, _ := getClientAddr(raftServer.Leader())
			url = scheme + clientAddr + path
		} else {
			url = scheme + raftServer.Leader() + path
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

// Handler to return the current leader name
func LeaderHttpHandler(w http.ResponseWriter, req *http.Request) {
	leader := raftServer.Leader()

	if leader != "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(raftServer.Leader()))
	} else {

		// not likely, but it may happen
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(newJsonError(301, ""))
	}
}

// Handler to return all the known machines in the current cluster
func MachinesHttpHandler(w http.ResponseWriter, req *http.Request) {
	peers := raftServer.Peers()

	// Add itself to the machine list first
	// Since peer map does not contain the server itself
	machines, _ := getClientAddr(raftServer.Name())

	// Add all peers to the list and separate by comma
	// We do not use json here since we accept machines list
	// in the command line separate by comma.

	for peerName, _ := range peers {
		if addr, ok := getClientAddr(peerName); ok {
			machines = machines + "," + addr
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(machines))

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

	debugf("[recv] GET http://%v/v1/keys/%s", raftServer.Name(), key)

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
		debugf("[recv] GET http://%v/watch/%s", raftServer.Name(), key)
		command.SinceIndex = 0

	} else if req.Method == "POST" {
		// watch from a specific index

		debugf("[recv] POST http://%v/watch/%s", raftServer.Name(), key)
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

// Convert string duration to time format
func durationToExpireTime(strDuration string) (time.Time, error) {
	if strDuration != "" {
		duration, err := strconv.Atoi(strDuration)

		if err != nil {
			return time.Unix(0, 0), err
		}
		return time.Now().Add(time.Second * (time.Duration)(duration)), nil
	} else {
		return time.Unix(0, 0), nil
	}
}
