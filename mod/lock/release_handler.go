package lock

import (
	"net/http"
)

// releaseLockHandler deletes the lock.
func (h *handler) releaseLockHandler(w http.ResponseWriter, req *http.Request) {
	// TODO: h.client.Delete(key_with_index)
}

