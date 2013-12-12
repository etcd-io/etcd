package v2

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

// setHandler attempts to set the current leader.
func (h *handler) setHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	name := req.FormValue("name")
	if name == "" {
		http.Error(w, "leader name required", http.StatusInternalServerError)
		return
	}

	// Proxy the request to the the lock service.
	u, err := url.Parse(fmt.Sprintf("%s/mod/v2/lock/%s", h.addr, vars["key"]))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q := u.Query()
	q.Set("value", name)
	q.Set("ttl", req.FormValue("ttl"))
	q.Set("timeout", req.FormValue("timeout"))
	u.RawQuery = q.Encode()

	r, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Close request if this connection disconnects.
	closeNotifier, _ := w.(http.CloseNotifier)
	stopChan := make(chan bool)
	defer close(stopChan)
	go func() {
		select {
		case <-closeNotifier.CloseNotify():
			h.transport.CancelRequest(r)
		case <-stopChan:
		}
	}()

	// Read from the leader lock.
	resp, err := h.client.Do(r)
	if err != nil {
		http.Error(w, "set leader http error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		w.Write([]byte("set leader error: "))
	}
	io.Copy(w, resp.Body)
}
