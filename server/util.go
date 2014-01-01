package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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
	originalURL := req.URL
	redirectURL, _ := url.Parse(hostname)

	// we need the original path and raw query
	redirectURL.Path = originalURL.Path
	redirectURL.RawQuery = originalURL.RawQuery
	redirectURL.Fragment = originalURL.Fragment

	log.Debugf("Redirect to %s", redirectURL.String())
	http.Redirect(w, req, redirectURL.String(), http.StatusTemporaryRedirect)
}

// trimsplit slices s into all substrings separated by sep and returns a
// slice of the substrings between the separator with all leading and trailing
// white space removed, as defined by Unicode.
func trimsplit(s, sep string) []string {
	trimmed := strings.Split(s, sep)
	for i := range trimmed {
		trimmed[i] = strings.TrimSpace(trimmed[i])
	}
	return trimmed
}
