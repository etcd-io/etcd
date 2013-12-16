package v2

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

// deleteHandler remove a given leader leader.
func (h *handler) deleteHandler(w http.ResponseWriter, req *http.Request) {
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
	u.RawQuery = q.Encode()

	r, err := http.NewRequest("DELETE", u.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Read from the leader lock.
	resp, err := h.client.Do(r)
	if err != nil {
		http.Error(w, "delete leader http error: " + err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		w.Write([]byte("delete leader error: "))
	}
	io.Copy(w, resp.Body)
}
