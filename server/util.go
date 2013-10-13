package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/coreos/etcd/log"
)

func decodeJsonRequest(req *http.Request, data interface{}) error {
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&data); err != nil && err != io.EOF {
		log.Warnf("Malformed json request: %v", err)
		return fmt.Errorf("Malformed json request: %v", err)
	}
	return nil
}

func redirect(hostname string, w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	url := hostname + path
	log.Debugf("Redirect to %s", url)
	http.Redirect(w, req, url, http.StatusTemporaryRedirect)
}
