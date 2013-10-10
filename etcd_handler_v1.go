package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

//-------------------------------------------------------------------
// Handlers to handle etcd-store related request via etcd url
//-------------------------------------------------------------------
// Multiplex GET/POST/DELETE request to corresponding handlers
func (e *etcdServer) MultiplexerV1(w http.ResponseWriter, req *http.Request) error {

	switch req.Method {
	case "GET":
		return e.GetHttpHandlerV1(w, req)
	case "POST":
		return e.SetHttpHandlerV1(w, req)
	case "PUT":
		return e.SetHttpHandlerV1(w, req)
	case "DELETE":
		return e.DeleteHttpHandlerV1(w, req)
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
func (e *etcdServer) SetHttpHandlerV1(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] POST %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	req.ParseForm()

	value := req.Form.Get("value")

	if len(value) == 0 {
		return etcdErr.NewError(200, "Set", store.UndefIndex, store.UndefTerm)
	}

	strDuration := req.Form.Get("ttl")

	expireTime, err := durationToExpireTime(strDuration)

	if err != nil {
		return etcdErr.NewError(202, "Set", store.UndefIndex, store.UndefTerm)
	}

	if prevValueArr, ok := req.Form["prevValue"]; ok && len(prevValueArr) > 0 {
		command := &TestAndSetCommand{
			Key:        key,
			Value:      value,
			PrevValue:  prevValueArr[0],
			ExpireTime: expireTime,
		}

		return dispatchEtcdCommandV1(command, w, req)

	} else {
		command := &CreateCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
			Force:      true,
		}

		return dispatchEtcdCommandV1(command, w, req)
	}
}

// Delete Handler
func (e *etcdServer) DeleteHttpHandlerV1(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/keys/"):]

	debugf("[recv] DELETE %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &DeleteCommand{
		Key: key,
	}

	return dispatchEtcdCommandV1(command, w, req)
}

//--------------------------------------
// State non-sensitive handlers
// will not dispatch to leader
// TODO: add sensitive version for these
// command?
//--------------------------------------

// Get Handler
func (e *etcdServer) GetHttpHandlerV1(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/keys/"):]

	r := e.raftServer
	debugf("[recv] GET %s/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	command := &GetCommand{
		Key: key,
	}

	if event, err := command.Apply(r.Server); err != nil {
		return err
	} else {
		event, _ := event.(*store.Event)

		response := eventToResponse(event)
		bytes, _ := json.Marshal(response)

		w.WriteHeader(http.StatusOK)

		w.Write(bytes)

		return nil
	}

}

// Watch handler
func (e *etcdServer) WatchHttpHandlerV1(w http.ResponseWriter, req *http.Request) error {
	key := req.URL.Path[len("/v1/watch/"):]

	command := &WatchCommand{
		Key: key,
	}
	r := e.raftServer
	if req.Method == "GET" {
		debugf("[recv] GET %s/watch/%s [%s]", e.url, key, req.RemoteAddr)
		command.SinceIndex = 0

	} else if req.Method == "POST" {
		// watch from a specific index

		debugf("[recv] POST %s/watch/%s [%s]", e.url, key, req.RemoteAddr)
		content := req.FormValue("index")

		sinceIndex, err := strconv.ParseUint(string(content), 10, 64)
		if err != nil {
			return etcdErr.NewError(203, "Watch From Index", store.UndefIndex, store.UndefTerm)
		}
		command.SinceIndex = sinceIndex

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if event, err := command.Apply(r.Server); err != nil {
		return etcdErr.NewError(500, key, store.UndefIndex, store.UndefTerm)
	} else {
		event, _ := event.(*store.Event)

		response := eventToResponse(event)
		bytes, _ := json.Marshal(response)

		w.WriteHeader(http.StatusOK)

		w.Write(bytes)
		return nil
	}

}

// Dispatch the command to leader
func dispatchEtcdCommandV1(c Command, w http.ResponseWriter, req *http.Request) error {
	return dispatchV1(c, w, req, nameToEtcdURL)
}

func dispatchV1(c Command, w http.ResponseWriter, req *http.Request, toURL func(name string) (string, bool)) error {
	r := e.raftServer
	if r.State() == raft.Leader {
		if event, err := r.Do(c); err != nil {
			return err
		} else {
			if event == nil {
				return etcdErr.NewError(300, "Empty result from raft", store.UndefIndex, store.UndefTerm)
			}

			event, _ := event.(*store.Event)

			response := eventToResponse(event)
			bytes, _ := json.Marshal(response)

			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return nil

		}

	} else {
		leader := r.Leader()
		// current no leader
		if leader == "" {
			return etcdErr.NewError(300, "", store.UndefIndex, store.UndefTerm)
		}
		url, _ := toURL(leader)

		redirect(url, w, req)

		return nil
	}
}

func eventToResponse(event *store.Event) interface{} {
	if !event.Dir {
		response := &store.Response{
			Action:     event.Action,
			Key:        event.Key,
			Value:      event.Value,
			PrevValue:  event.PrevValue,
			Index:      event.Index,
			TTL:        event.TTL,
			Expiration: event.Expiration,
		}

		if response.Action == store.Create || response.Action == store.Update {
			response.Action = "set"
			if response.PrevValue == "" {
				response.NewKey = true
			}
		}

		return response
	} else {
		responses := make([]*store.Response, len(event.KVPairs))

		for i, kv := range event.KVPairs {
			responses[i] = &store.Response{
				Action: event.Action,
				Key:    kv.Key,
				Value:  kv.Value,
				Dir:    kv.Dir,
				Index:  event.Index,
			}
		}
		return responses
	}
}
