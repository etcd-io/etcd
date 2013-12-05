package v2

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/stretchr/testify/assert"
)

// Ensures that a key is deleted.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar
//
func TestV2DeleteKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), url.Values{})
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo/bar","prevValue":"XXX","modifiedIndex":3,"createdIndex":2}}`, "")
	})
}
