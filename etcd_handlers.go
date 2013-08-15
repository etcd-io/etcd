package main

import (
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type HttpHandler func(w ResponseWriter, r *http.Request)

// errorHandler wraps the argument handler with an error-catcher that
// returns a 500 HTTP error if the request fails (calls check with err non-nil).
func errorHandler(fn HttpHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err, ok := recover().(error); ok {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}()
		ew := &DefaultEtcdResponseWriter{w: w, e: nil, n: 0}
		fn(ew, r)
		if ew.e != nil {
			warn(ew.e)
			http.Error(w, ew.e.Error(), http.StatusInternalServerError)
		}
	}
}

type ResponseWriter interface {
	http.ResponseWriter
	WriteError(status, errorcode int, msg string)
	WriteString(string)
	WriteJson(int, interface{})
	WriteText(int, interface{})
}

type DefaultEtcdResponseWriter struct {
	// Underlyng system
	w http.ResponseWriter
	// Last Error
	e error
	// how much data was written
	n int
}

func (r *DefaultEtcdResponseWriter) Header() http.Header {
	return r.w.Header()
}

func (r *DefaultEtcdResponseWriter) WriteHeader(code int) {
	if r.e != nil {
		return
	}
	r.w.WriteHeader(code)
}

func (r *DefaultEtcdResponseWriter) Write(data []byte) (int, error) {
	if r.e != nil {
		return -1, r.e
	}
	r.n, r.e = r.w.Write(data)
	return r.n, r.e
}

func (r *DefaultEtcdResponseWriter) WriteText(statusCode int, data interface{}) {
	if r.e != nil {
		return
	}

	r.Header().Set("Content-Type", "text/plain")
	r.WriteHeader(statusCode)

	body, ok := data.([]byte)
	// this should not happen
	if !ok {
		r.e = fmt.Errorf("Wrong type to WriteText: %s ", data)
		return
	}

	r.n, r.e = r.w.Write(body)
}

func (r *DefaultEtcdResponseWriter) WriteJson(statusCode int, data interface{}) {
	if r.e != nil {
		return
	}
	var b []byte

	r.Header().Set("Content-Type", "application/json")
	r.WriteHeader(statusCode)

	r.e = json.NewEncoder(r.w).Encode(data)
	if r.e != nil {
		return
	}

	r.n, r.e = r.w.Write(b)
	if r.e != nil {
		return
	}
}

func (r *DefaultEtcdResponseWriter) WriteError(statusCode, errorCode int, cause string) {
	r.WriteJson(statusCode, jsonError{
		ErrorCode: errorCode,
		Message:   errors[errorCode],
		Cause:     cause,
	})
}

func (r *DefaultEtcdResponseWriter) WriteString(msg string) {
	if r.e != nil {
		return
	}
	r.n, r.e = io.WriteString(r.w, msg)
}

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via etcd url
//-------------------------------------------------------------------

func NewEtcdMuxer() *http.ServeMux {
	// external commands
	etcdMux := http.NewServeMux()
	etcdMux.HandleFunc("/"+version+"/keys/", errorHandler(Multiplexer))
	etcdMux.HandleFunc("/"+version+"/watch/", errorHandler(WatchHttpHandler))
	etcdMux.HandleFunc("/leader", errorHandler(LeaderHttpHandler))
	etcdMux.HandleFunc("/machines", errorHandler(MachinesHttpHandler))
	etcdMux.HandleFunc("/version", errorHandler(VersionHttpHandler))
	etcdMux.HandleFunc("/stats", errorHandler(StatsHttpHandler))
	etcdMux.HandleFunc("/test/", errorHandler(TestHttpHandler))
	return etcdMux
}

// Multiplex GET/POST/DELETE request to corresponding handlers
func Multiplexer(w ResponseWriter, req *http.Request) {

	switch req.Method {
	case "GET":
		GetHttpHandler(w, req)
	case "POST":
		SetHttpHandler(w, req)
	case "DELETE":
		DeleteHttpHandler(w, req)
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
func SetHttpHandler(w ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	if store.CheckKeyword(key) {
		w.WriteError(http.StatusBadRequest, 400, "Set")
		return
	}

	debugf("[recv] POST %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	value := req.FormValue("value")

	if len(value) == 0 {
		w.WriteError(http.StatusBadRequest, 200, "Set")
		return
	}

	prevValue := req.FormValue("prevValue")

	strDuration := req.FormValue("ttl")

	expireTime, err := durationToExpireTime(strDuration)

	if err != nil {
		w.WriteError(http.StatusBadRequest, 202, "Set")
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
func DeleteHttpHandler(w ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &DeleteCommand{
		Key: key,
	}

	dispatch(command, w, req, true)
}

// Dispatch the command to leader
func dispatch(c Command, w ResponseWriter, req *http.Request, etcd bool) {

	if r.State() == raft.Leader {
		if body, err := r.Do(c); err != nil {

			if _, ok := err.(store.NotFoundError); ok {
				w.WriteHeader(http.StatusNotFound)
				w.Write(newJsonError(100, err.Error()))
				return
			}

			if _, ok := err.(store.TestFail); ok {
				w.WriteError(http.StatusBadRequest, 101, err.Error())
				return
			}

			if _, ok := err.(store.NotFile); ok {
				w.WriteError(http.StatusBadRequest, 102, err.Error())
				return
			}
			if err.Error() == errors[103] {
				w.WriteError(http.StatusBadRequest, 103, "")
				return
			}
			w.WriteError(http.StatusInternalServerError, 300, err.Error())
			return
		} else {
			if body == nil {
				w.WriteError(http.StatusNotFound, 300, "Empty result from raft")
			} else {
				body, ok := body.([]byte)
				// this should not happen
				if !ok {
					panic("wrong type")
				}
				w.WriteHeader(http.StatusOK)
				w.Write(body)
			}
			return
		}
	} else {
		leader := r.Leader()
		// current no leader
		if leader == "" {
			w.WriteError(http.StatusInternalServerError, 300, "")
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

		http.Redirect(w, req, url, http.StatusTemporaryRedirect)
		return
	}
	w.WriteError(http.StatusInternalServerError, 300, "")
	return
}

//--------------------------------------
// State non-sensitive handlers
// will not dispatch to leader
// TODO: add sensitive version for these
// command?
//--------------------------------------

// Handler to return the current leader's raft address
func LeaderHttpHandler(w ResponseWriter, req *http.Request) {
	leader := r.Leader()

	if leader != "" {
		w.WriteHeader(http.StatusOK)
		raftURL, _ := nameToRaftURL(leader)
		w.WriteString(raftURL)
	} else {

		// not likely, but it may happen
		w.WriteError(http.StatusInternalServerError, 301, "")
	}
}

// Handler to return all the known machines in the current cluster
func MachinesHttpHandler(w ResponseWriter, req *http.Request) {
	machines := getMachines()

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, strings.Join(machines, ", "))
}

// Handler to return the current version of etcd
func VersionHttpHandler(w ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.WriteString(fmt.Sprintf("etcd %s", releaseVersion))
	w.WriteString(fmt.Sprintf("etcd API %s", version))
}

// Handler to return the basic stats of etcd
func StatsHttpHandler(w ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write(etcdStore.Stats())
}

// Get Handler
func GetHttpHandler(w ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] GET %s/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &GetCommand{
		Key: key,
	}

	if body, err := command.Apply(r.Server); err != nil {

		if _, ok := err.(store.NotFoundError); ok {
			w.WriteError(http.StatusNotFound, 100, err.Error())
			return
		}

		w.WriteError(http.StatusInternalServerError, 300, "")

	} else {
		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		w.WriteHeader(http.StatusOK)
		w.Write(body)

	}

}

// Watch handler
func WatchHttpHandler(w ResponseWriter, req *http.Request) {
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
			w.WriteError(http.StatusBadRequest, 203, "Watch From Index")
		}
		command.SinceIndex = sinceIndex

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if body, err := command.Apply(r.Server); err != nil {
		w.WriteError(http.StatusInternalServerError, 500, key)
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
func TestHttpHandler(w ResponseWriter, req *http.Request) {
	testType := req.URL.Path[len("/test/"):]

	if testType == "speed" {
		directSet()
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "speed test success")
		return
	}

	w.WriteHeader(http.StatusBadRequest)
}
