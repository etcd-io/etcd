package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/go-raft"
)

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via etcd url
//-------------------------------------------------------------------

func NewEtcdMuxer() *http.ServeMux {
	// external commands
	etcdMux := http.NewServeMux()
	etcdMux.Handle("/"+version+"/keys/", errorHandler(Multiplexer))
	etcdMux.Handle("/"+version+"/leader", errorHandler(LeaderHttpHandler))
	etcdMux.Handle("/"+version+"/machines", errorHandler(MachinesHttpHandler))
	etcdMux.Handle("/"+version+"/stats", errorHandler(StatsHttpHandler))
	etcdMux.Handle("/version", errorHandler(VersionHttpHandler))
	etcdMux.HandleFunc("/test/", TestHttpHandler)
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
		return CreateHttpHandler(w, req)
	case "PUT":
		return UpdateHttpHandler(w, req)
	case "DELETE":
		return DeleteHttpHandler(w, req)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	return nil
}

//--------------------------------------
// State sensitive handlers
// Set/Delete will dispatch to leader
//--------------------------------------

func CreateHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v2/keys"):]

	debugf("recv.post[%v] [%v%v]\n", req.RemoteAddr, req.Host, req.URL)

	req.ParseForm()

	value := req.Form.Get("value")

	ttl := req.FormValue("ttl")

	expireTime, err := durationToExpireTime(ttl)

	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Create")
	}

	command := &CreateCommand{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
	}

	return dispatch(command, w, req, true)

}

func UpdateHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v2/keys"):]

	debugf("recv.put[%v] [%v%v]\n", req.RemoteAddr, req.Host, req.URL)

	value := req.FormValue("value")

	ttl := req.FormValue("ttl")

	expireTime, err := durationToExpireTime(ttl)

	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Update")
	}

	// TODO: update should give at least one option
	if value == "" && ttl == "" {
		return nil
	}

	prevValue := req.FormValue("prevValue")

	prevIndexStr := req.FormValue("prevIndex")

	if prevValue == "" && prevIndexStr == "" { // update without test
		command := &UpdateCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
		}

		return dispatch(command, w, req, true)

	} else { // update with test
		var prevIndex uint64

		if prevIndexStr != "" {
			prevIndex, err = strconv.ParseUint(prevIndexStr, 10, 64)
		}

		// TODO: add error type
		if err != nil {
			return nil
		}

		command := &TestAndSetCommand{
			Key:       key,
			Value:     value,
			PrevValue: prevValue,
			PrevIndex: prevIndex,
		}

		return dispatch(command, w, req, true)
	}

}

// Delete Handler
func DeleteHttpHandler(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v2/keys"):]

	debugf("recv.delete[%v] [%v%v]\n", req.RemoteAddr, req.Host, req.URL)

	command := &DeleteCommand{
		Key: key,
	}

	if req.FormValue("recursive") == "true" {
		command.Recursive = true
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
func LeaderHttpHandler(w http.ResponseWriter, req *http.Request) error {
	leader := r.Leader()

	if leader != "" {
		w.WriteHeader(http.StatusOK)
		raftURL, _ := nameToRaftURL(leader)
		w.Write([]byte(raftURL))
		return nil
	} else {
		return etcdErr.NewError(etcdErr.EcodeLeaderElect, "")
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
	w.WriteHeader(http.StatusOK)
	w.Write(etcdStore.Stats())
	w.Write(r.Stats())
	return nil
}

func GetHttpHandler(w http.ResponseWriter, req *http.Request) error {
	var err error
	var event interface{}
	key := req.URL.Path[len("/v1/keys"):]

	debugf("recv.get[%v] [%v%v]\n", req.RemoteAddr, req.Host, req.URL)

	recursive := req.FormValue("recursive")

	if req.FormValue("wait") == "true" {
		command := &WatchCommand{
			Key: key,
		}

		if recursive == "true" {
			command.Recursive = true
		}

		indexStr := req.FormValue("wait_index")

		if indexStr != "" {
			sinceIndex, err := strconv.ParseUint(indexStr, 10, 64)

			if err != nil {
				return etcdErr.NewError(etcdErr.EcodeIndexNaN, "Watch From Index")
			}

			command.SinceIndex = sinceIndex
		}

		event, err = command.Apply(r.Server)

	} else {
		command := &GetCommand{
			Key: key,
		}

		sorted := req.FormValue("sorted")

		if sorted == "true" {
			command.Sorted = true
		}

		if recursive == "true" {
			command.Recursive = true
		}

		event, err = command.Apply(r.Server)
	}

	if err != nil {
		return err
	} else {
		event, _ := event.([]byte)
		w.WriteHeader(http.StatusOK)
		w.Write(event)

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
