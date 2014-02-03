package v2

import (
	"fmt"
	"io"
	"net/http"

	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// getHandler retrieves the current leader.
func (h *handler) getHandler(w http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)

	// Proxy the request to the lock service.
	url := fmt.Sprintf("%s/mod/v2/lock/%s?field=value", h.addr, vars["key"])
	resp, err := h.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	return nil
}
