package v2

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// acquireHandler attempts to acquire a lock on the given key.
// The "key" parameter specifies the resource to lock.
// The "value" parameter specifies a value to associate with the lock.
// The "ttl" parameter specifies how long the lock will persist for.
// The "timeout" parameter specifies how long the request should wait for the lock.
func (h *handler) acquireHandler(w http.ResponseWriter, req *http.Request) error {
	h.client.SyncCluster()

	// Setup connection watcher.
	closeNotifier, _ := w.(http.CloseNotifier)
	closeChan := closeNotifier.CloseNotify()
	stopChan := make(chan bool)

	// Parse the lock "key".
	vars := mux.Vars(req)
	keypath := path.Join(prefix, vars["key"])
	value := req.FormValue("value")

	// Parse "timeout" parameter.
	var timeout int
	var err error
	if req.FormValue("timeout") == "" {
		timeout = -1
	} else if timeout, err = strconv.Atoi(req.FormValue("timeout")); err != nil {
		return etcdErr.NewError(etcdErr.EcodeTimeoutNaN, "Acquire", 0)
	}
	timeout = timeout + 1

	// Parse TTL.
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Acquire", 0)
	}

	// If node exists then just watch it. Otherwise create the node and watch it.
	node, index, pos := h.findExistingNode(keypath, value)
	if index > 0 {
		if pos == 0 {
			// If lock is already acquired then update the TTL.
			h.client.Update(node.Key, node.Value, uint64(ttl))
		} else {
			// Otherwise watch until it becomes acquired (or errors).
			err = h.watch(keypath, index, nil)
		}
	} else {
		index, err = h.createNode(keypath, value, ttl, closeChan, stopChan)
	}

	// Stop all goroutines.
	close(stopChan)

	// Check for an error.
	if err != nil {
		return err
	}

	// Write response.
	w.Write([]byte(strconv.Itoa(index)))
	return nil
}

// createNode creates a new lock node and watches it until it is acquired or acquisition fails.
func (h *handler) createNode(keypath string, value string, ttl int, closeChan <-chan bool, stopChan chan bool) (int, error) {
	// Default the value to "-" if it is blank.
	if len(value) == 0 {
		value = "-"
	}

	// Create an incrementing id for the lock.
	resp, err := h.client.AddChild(keypath, value, uint64(ttl))
	if err != nil {
		return 0, err
	}
	indexpath := resp.Node.Key
	index, _ := strconv.Atoi(path.Base(indexpath))

	// Keep updating TTL to make sure lock request is not expired before acquisition.
	go h.ttlKeepAlive(indexpath, value, ttl, stopChan)

	// Watch until we acquire or fail.
	err = h.watch(keypath, index, closeChan)

	// Check for connection disconnect before we write the lock index.
	if err != nil {
		select {
		case <-closeChan:
			err = errors.New("user interrupted")
		default:
		}
	}

	// Update TTL one last time if acquired. Otherwise delete.
	if err == nil {
		h.client.Update(indexpath, value, uint64(ttl))
	} else {
		h.client.Delete(indexpath, false)
	}

	return index, err
}

// findExistingNode search for a node on the lock with the given value.
func (h *handler) findExistingNode(keypath string, value string) (*etcd.Node, int, int) {
	if len(value) > 0 {
		resp, err := h.client.Get(keypath, true, true)
		if err == nil {
			nodes := lockNodes{resp.Node.Nodes}
			if node, pos := nodes.FindByValue(value); node != nil {
				index, _ := strconv.Atoi(path.Base(node.Key))
				return node, index, pos
			}
		}
	}
	return nil, 0, 0
}

// ttlKeepAlive continues to update a key's TTL until the stop channel is closed.
func (h *handler) ttlKeepAlive(k string, value string, ttl int, stopChan chan bool) {
	for {
		select {
		case <-time.After(time.Duration(ttl/2) * time.Second):
			h.client.Update(k, value, uint64(ttl))
		case <-stopChan:
			return
		}
	}
}

// watch continuously waits for a given lock index to be acquired or until lock fails.
// Returns a boolean indicating success.
func (h *handler) watch(keypath string, index int, closeChan <-chan bool) error {
	// Wrap close chan so we can pass it to Client.Watch().
	stopWatchChan := make(chan bool)
	stopWrapChan := make(chan bool)
	go func() {
		select {
		case <-closeChan:
			stopWatchChan <- true
		case <- stopWrapChan:
			stopWatchChan <- true
		case <- stopWatchChan:
		}
	}()
	defer close(stopWrapChan)

	for {
		// Read all nodes for the lock.
		resp, err := h.client.Get(keypath, true, true)
		if err != nil {
			return fmt.Errorf("lock watch lookup error: %s", err.Error())
		}
		nodes := lockNodes{resp.Node.Nodes}
		prevIndex := nodes.PrevIndex(index)

		// If there is no previous index then we have the lock.
		if prevIndex == 0 {
			return nil
		}

		// Watch previous index until it's gone.
		waitIndex := resp.Node.ModifiedIndex

		// Since event store has only 1000 histories we should use first node's CreatedIndex if available
		if firstNode := nodes.First(); firstNode != nil {
			waitIndex = firstNode.CreatedIndex
		}

		_, err = h.client.Watch(path.Join(keypath, strconv.Itoa(prevIndex)), waitIndex, false, nil, stopWatchChan)
		if err == etcd.ErrWatchStoppedByUser {
			return fmt.Errorf("lock watch closed")
		} else if err != nil {
			return fmt.Errorf("lock watch error: %s", err.Error())
		}
	}
}
