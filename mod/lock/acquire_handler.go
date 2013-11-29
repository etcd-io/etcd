package lock

import (
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// acquireHandler attempts to acquire a lock on the given key.
func (h *handler) acquireHandler(w http.ResponseWriter, req *http.Request) {
	h.client.SyncCluster()

	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		http.Error(w, "invalid ttl: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Create an incrementing id for the lock.
	resp, err := h.client.AddChild(keypath, "-", uint64(ttl))
	if err != nil {
		http.Error(w, "add lock index error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	indexpath := resp.Key

	// Keep updating TTL to make sure lock request is not expired before acquisition.
	stopChan := make(chan bool)
	defer close(stopChan)
	go func(k string) {
		stopped := false
		for {
			select {
			case <-time.After(time.Duration(ttl / 2) * time.Second):
			case <-stopChan:
				stopped = true
			}
			h.client.Update(k, "-", uint64(ttl))
			if stopped {
				break
			}
		}
	}(indexpath)

	// Extract the lock index.
	index, _ := strconv.Atoi(path.Base(resp.Key))

	for {
		// Read all indices.
		resp, err = h.client.GetAll(keypath, true)
		if err != nil {
			http.Error(w, "lock children lookup error: " + err.Error(), http.StatusInternalServerError)
			return
		}
		indices := extractResponseIndices(resp)
		waitIndex := resp.ModifiedIndex
		prevIndex := findPrevIndex(indices, index)

		// If there is no previous index then we have the lock.
		if prevIndex == 0 {
			break
		}

		// Otherwise watch previous index until it's gone.
		_, err = h.client.Watch(path.Join(keypath, strconv.Itoa(prevIndex)), waitIndex, nil, nil)
		if err != nil {
			http.Error(w, "lock watch error: " + err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Write lock index to response body.
	w.Write([]byte(strconv.Itoa(index)))
}
