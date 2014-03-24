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

	// Wrap closeChan so we can pass it to subsequent components
	timeoutChan := make(chan bool)
	stopChan := make(chan bool)
	go func() {
		select {
		case <-closeChan:
			// Client closed connection
			stopChan <- true
		case <-timeoutChan:
			// Timeout expired
			stopChan <- true
		case <-stopChan:
		}
		close(stopChan)
	}()

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

	// Parse TTL.
	ttl, err := strconv.Atoi(req.FormValue("ttl"))
	if err != nil {
		return etcdErr.NewError(etcdErr.EcodeTTLNaN, "Acquire", 0)
	}

	// Search for the node
	_, index, pos := h.findExistingNode(keypath, value)
	if index == 0 {
		// Node doesn't exist; Create it
		pos = -1 // Invalidate previous position
		index, err = h.createNode(keypath, value, ttl)
		if err != nil {
			return err
		}
	}

	indexpath := path.Join(keypath, strconv.Itoa(index))

	// If pos != 0, we do not already have the lock
	if pos != 0 {
		if timeout == 0 {
			// Attempt to get lock once, no waiting
			err = h.get(keypath, index)
		} else {
			// Keep updating TTL while we wait
			go h.ttlKeepAlive(keypath, value, ttl, stopChan)

			// Start timeout
			go h.timeoutExpire(timeout, timeoutChan, stopChan)

			// wait for lock
			err = h.watch(keypath, index, stopChan)
		}
	}

	// Return on error, deleting our lock request on the way
	if err != nil {
		if index > 0 {
			h.client.Delete(indexpath, false)
		}
		return err
	}

	// Check for connection disconnect before we write the lock index.
	select {
	case <-stopChan:
		err = errors.New("user interrupted")
	default:
	}

	// Update TTL one last time if lock was acquired. Otherwise delete.
	if err == nil {
		h.client.Update(indexpath, value, uint64(ttl))
	} else {
		h.client.Delete(indexpath, false)
	}

	// Write response.
	w.Write([]byte(strconv.Itoa(index)))
	return nil
}

// createNode creates a new lock node and watches it until it is acquired or acquisition fails.
func (h *handler) createNode(keypath string, value string, ttl int) (int, error) {
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
	index, err := strconv.Atoi(path.Base(indexpath))
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

// timeoutExpire sets the countdown timer is a positive integer
// cancels on stopChan, sends true on timeoutChan after timer expires
func (h *handler) timeoutExpire(timeout int, timeoutChan chan bool, stopChan chan bool) {
	// Set expiration timer if timeout is 1 or higher
	if timeout < 1 {
		timeoutChan = nil
		return
	}
	select {
	case <-stopChan:
		return
	case <-time.After(time.Duration(timeout) * time.Second):
		timeoutChan <- true
		return
	}
}

func (h *handler) getLockIndex(keypath string, index int) (int, int, error) {
	// Read all nodes for the lock.
	resp, err := h.client.Get(keypath, true, true)
	if err != nil {
		return 0, 0, fmt.Errorf("lock watch lookup error: %s", err.Error())
	}
	nodes := lockNodes{resp.Node.Nodes}
	prevIndex, modifiedIndex := nodes.PrevIndex(index)
	return prevIndex, modifiedIndex, nil
}

// get tries once to get the lock; no waiting
func (h *handler) get(keypath string, index int) error {
	prevIndex, _, err := h.getLockIndex(keypath, index)
	if err != nil {
		return err
	}
	if prevIndex == 0 {
		// Lock acquired
		return nil
	}
	return fmt.Errorf("failed to acquire lock")
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
		case <-stopWrapChan:
			stopWatchChan <- true
		case <-stopWatchChan:
		}
	}()
	defer close(stopWrapChan)

	for {
		prevIndex, modifiedIndex, err := h.getLockIndex(keypath, index)
		// If there is no previous index then we have the lock.
		if prevIndex == 0 {
			return nil
		}

		// Wait from the last modification of the node.
		waitIndex := modifiedIndex + 1

		_, err = h.client.Watch(path.Join(keypath, strconv.Itoa(prevIndex)), uint64(waitIndex), false, nil, stopWatchChan)
		if err == etcd.ErrWatchStoppedByUser {
			return fmt.Errorf("lock watch closed")
		} else if err != nil {
			return fmt.Errorf("lock watch error: %s", err.Error())
		}
		return nil
	}
}
