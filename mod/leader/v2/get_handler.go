package v2

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

// getHandler retrieves the current leader.
func (h *handler) getHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	// Proxy the request to the lock service.
	url := fmt.Sprintf("%s/mod/v2/lock/%s?field=value", h.addr, vars["key"])
	resp, err := h.client.Get(url)
	if err != nil {
		http.Error(w, "read leader error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.Write([]byte("get leader error: "))
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
