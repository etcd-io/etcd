package v2

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/gorilla/mux"
)

// deleteHandler remove a given leader.
func (h *handler) deleteHandler(w http.ResponseWriter, req *http.Request) error {
	vars := mux.Vars(req)
	name := req.FormValue("name")
	if name == "" {
		return etcdErr.NewError(etcdErr.EcodeNameRequired, "Delete", 0)
	}

	// Proxy the request to the the lock service.
	u, err := url.Parse(fmt.Sprintf("%s/mod/v2/lock/%s", h.addr, vars["key"]))
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("value", name)
	u.RawQuery = q.Encode()

	r, err := http.NewRequest("DELETE", u.String(), nil)
	if err != nil {
		return err
	}

	// Read from the leader lock.
	resp, err := h.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	return nil
}
