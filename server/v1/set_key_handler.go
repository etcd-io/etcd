package v1

import (
	"encoding/json"
	"github.com/coreos/etcd/store"
	"net/http"
)

// Sets the value for a given key.
func setKeyHandler(w http.ResponseWriter, req *http.Request, e *etcdServer) error {
	vars := mux.Vars(req)
	key := "/" + vars["key"]

	debugf("[recv] POST %v/v1/keys/%s [%s]", e.url, key, req.RemoteAddr)

	req.ParseForm()

	// Parse non-blank value.
	value := req.Form.Get("value")
	if len(value) == 0 {
		return error.NewError(200, "Set", store.UndefIndex, store.UndefTerm)
	}

	// Convert time-to-live to an expiration time.
	expireTime, err := durationToExpireTime(req.Form.Get("ttl"))
	if err != nil {
		return etcdErr.NewError(202, "Set", store.UndefIndex, store.UndefTerm)
	}

	// If the "prevValue" is specified then test-and-set. Otherwise create a new key.
	var c command.Command
	if prevValueArr, ok := req.Form["prevValue"]; ok && len(prevValueArr) > 0 {
		c = &TestAndSetCommand{
			Key:        key,
			Value:      value,
			PrevValue:  prevValueArr[0],
			ExpireTime: expireTime,
		}

	} else {
		c = &CreateCommand{
			Key:        key,
			Value:      value,
			ExpireTime: expireTime,
			Force:      true,
		}
	}

	return dispatchEtcdCommand(command, w, req)
}
