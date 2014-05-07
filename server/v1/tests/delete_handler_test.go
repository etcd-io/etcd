package v1

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures that a key is deleted.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v1/keys/foo/bar
//
func TestV1DeleteKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), url.Values{})
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","key":"/foo/bar","prevValue":"XXX","index":4}`, "")
	})
}
