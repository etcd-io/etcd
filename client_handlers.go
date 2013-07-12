package main

import (
	"github.com/coreos/etcd/store"
	"net/http"
	"strconv"
	"time"
)

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via raft client port
//-------------------------------------------------------------------

// Multiplex GET/POST/DELETE request to corresponding handlers
func Multiplexer(w http.ResponseWriter, req *http.Request) {

	if req.Method == "GET" {
		GetHttpHandler(&w, req)
	} else if req.Method == "POST" {
		SetHttpHandler(&w, req)
	} else if req.Method == "DELETE" {
		DeleteHttpHandler(&w, req)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

//--------------------------------------
// State sensitive handlers
// Set/Delte will dispatch to leader
//--------------------------------------

// Set Command Handler
func SetHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debug("[recv] POST http://%v/v1/keys/%s", raftServer.Name(), key)

	command := &SetCommand{}
	command.Key = key

	command.Value = req.FormValue("value")

	if len(command.Value) == 0 {
		(*w).WriteHeader(http.StatusBadRequest)

		(*w).Write(newJsonError(200, "Set"))
		return
	}

	strDuration := req.FormValue("ttl")

	var err error

	command.ExpireTime, err = durationToExpireTime(strDuration)

	if err != nil {

		(*w).WriteHeader(http.StatusBadRequest)

		(*w).Write(newJsonError(202, "Set"))
	}

	dispatch(command, w, req, true)

}

// TestAndSet handler
func TestAndSetHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/testAndSet/"):]

	debug("[recv] POST http://%v/v1/testAndSet/%s", raftServer.Name(), key)

	command := &TestAndSetCommand{}
	command.Key = key

	command.PrevValue = req.FormValue("prevValue")
	command.Value = req.FormValue("value")

	if len(command.Value) == 0 {
		w.WriteHeader(http.StatusBadRequest)

		w.Write(newJsonError(200, "TestAndSet"))

		return
	}

	if len(command.PrevValue) == 0 {
		w.WriteHeader(http.StatusBadRequest)

		w.Write(newJsonError(201, "TestAndSet"))
		return
	}

	strDuration := req.FormValue("ttl")

	var err error

	command.ExpireTime, err = durationToExpireTime(strDuration)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)

		w.Write(newJsonError(202, "TestAndSet"))
	}

	dispatch(command, &w, req, true)

}

// Delete Handler
func DeleteHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debug("[recv] DELETE http://%v/v1/keys/%s", raftServer.Name(), key)

	command := &DeleteCommand{}
	command.Key = key

	dispatch(command, w, req, true)
}

// Dispatch the command to leader
func dispatch(c Command, w *http.ResponseWriter, req *http.Request, client bool) {
	if raftServer.State() == "leader" {
		if body, err := raftServer.Do(c); err != nil {
			if _, ok := err.(store.NotFoundError); ok {
				http.NotFound((*w), req)
				return
			}

			if _, ok := err.(store.TestFail); ok {
				(*w).WriteHeader(http.StatusBadRequest)
				(*w).Write(newJsonError(101, err.Error()))
				return
			}
			(*w).WriteHeader(http.StatusInternalServerError)
			(*w).Write(newJsonError(300, "No Leader"))
			return
		} else {

			body, ok := body.([]byte)
			if !ok {
				panic("wrong type")
			}

			if body == nil {
				http.NotFound((*w), req)
			} else {
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
			url = scheme + raftTransporter.GetLeaderClientAddress() + path
		} else {
			url = scheme + raftServer.Leader() + path
		}

		debug("Redirect to %s", url)

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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(raftServer.Leader()))
}

// Get Handler
func GetHttpHandler(w *http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debug("[recv] GET http://%v/v1/keys/%s", raftServer.Name(), key)

	command := &GetCommand{}
	command.Key = key

	if body, err := command.Apply(raftServer); err != nil {

		if _, ok := err.(store.NotFoundError); ok {
			http.NotFound((*w), req)
			return
		}

		(*w).WriteHeader(http.StatusInternalServerError)
		(*w).Write(newJsonError(300, ""))
		return
	} else {
		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		(*w).WriteHeader(http.StatusOK)
		(*w).Write(body)

		return
	}

}

// List Handler
func ListHttpHandler(w http.ResponseWriter, req *http.Request) {
	prefix := req.URL.Path[len("/v1/list/"):]

	debug("[recv] GET http://%v/v1/list/%s", raftServer.Name(), prefix)

	command := &ListCommand{}
	command.Prefix = prefix

	if body, err := command.Apply(raftServer); err != nil {
		if _, ok := err.(store.NotFoundError); ok {
			http.NotFound(w, req)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(newJsonError(300, ""))
		return
	} else {
		w.WriteHeader(http.StatusOK)

		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		w.Write(body)
		return
	}

}

// Watch handler
func WatchHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/watch/"):]

	command := &WatchCommand{}
	command.Key = key

	if req.Method == "GET" {
		debug("[recv] GET http://%v/watch/%s", raftServer.Name(), key)
		command.SinceIndex = 0

	} else if req.Method == "POST" {
		// watch from a specific index

		debug("[recv] POST http://%v/watch/%s", raftServer.Name(), key)
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
		warn("Unable to do watch command: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		w.WriteHeader(http.StatusOK)

		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		w.Write(body)
		return
	}

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
