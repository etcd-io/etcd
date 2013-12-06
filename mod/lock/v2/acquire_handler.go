package v2

import (
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gorilla/mux"
)

// acquireHandler attempts to acquire a lock on the given key.
// The "key" parameter specifies the resource to lock.
// The "ttl" parameter specifies how long the lock will persist for.
// The "timeout" parameter specifies how long the request should wait for the lock.
func (h *handler) acquireHandler(w http.ResponseWriter, req *http.Request) {
	h.client.SyncCluster()

	// Setup connection watcher.
	closeNotifier, _ := w.(http.CloseNotifier)
	closeChan := closeNotifier.CloseNotify()

	// Parse "key" and "ttl" query parameters.
	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		http.Error(w, "invalid ttl: " + err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Parse "timeout" parameter.
	var timeout int
	if len(req.FormValue("timeout")) == 0 {
		timeout = -1
	} else if timeout, err = strconv.Atoi(req.FormValue("timeout")); err != nil {
		http.Error(w, "invalid timeout: " + err.Error(), http.StatusInternalServerError)
		return
	}
	timeout = timeout + 1

	// Create an incrementing id for the lock.
	resp, err := h.client.AddChild(keypath, "-", uint64(ttl))
	if err != nil {
		http.Error(w, "add lock index error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	indexpath := resp.Node.Key

	// Keep updating TTL to make sure lock request is not expired before acquisition.
	stop := make(chan bool)
	go h.ttlKeepAlive(indexpath, ttl, stop)

	// Monitor for broken connection.
	stopWatchChan := make(chan bool)
	go func() {
		select {
		case <-closeChan:
			stopWatchChan <- true
		case <-stop:
			// Stop watching for connection disconnect.
		}
	}()

	// Extract the lock index.
	index, _ := strconv.Atoi(path.Base(resp.Node.Key))

	// Wait until we successfully get a lock or we get a failure.
	var success bool
	for {
		// Read all indices.
		resp, err = h.client.Get(keypath, true, true)
		if err != nil {
			http.Error(w, "lock children lookup error: " + err.Error(), http.StatusInternalServerError)
			break
		}
		indices := extractResponseIndices(resp)
		waitIndex := resp.Node.ModifiedIndex
		prevIndex := findPrevIndex(indices, index)

		// If there is no previous index then we have the lock.
		if prevIndex == 0 {
			success = true
			break
		}

		// Otherwise watch previous index until it's gone.
		_, err = h.client.Watch(path.Join(keypath, strconv.Itoa(prevIndex)), waitIndex, false, nil, stopWatchChan)
		if err == etcd.ErrWatchStoppedByUser {
			break
		} else if err != nil {
			http.Error(w, "lock watch error: " + err.Error(), http.StatusInternalServerError)
			break
		}
	}

	// Check for connection disconnect before we write the lock index.
	select {
	case <-stopWatchChan:
		success = false
	default:
	}

	// Stop the ttl keep-alive.
	close(stop)

	if success {
		// Write lock index to response body if we acquire the lock.
		h.client.Update(indexpath, "-", uint64(ttl))
		w.Write([]byte(strconv.Itoa(index)))
	} else {
		// Make sure key is deleted if we couldn't acquire.
		h.client.Delete(indexpath, false)
	}
}

// ttlKeepAlive continues to update a key's TTL until the stop channel is closed.
func (h *handler) ttlKeepAlive(k string, ttl int, stop chan bool) {
	for {
		select {
		case <-time.After(time.Duration(ttl / 2) * time.Second):
			h.client.Update(k, "-", uint64(ttl))
		case <-stop:
			return
		}
	}
}
