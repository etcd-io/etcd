package v2

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures that a key is set to a given value.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//
func TestV1SetKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")

		assert.Equal(t, string(body), `{"action":"set","key":"/foo/bar","value":"XXX","newKey":true,"index":2}`, "")
	})
}
