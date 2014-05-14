package v2

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures a unique value is added to the key's children.
//
//   $ curl -X POST localhost:4001/v2/keys/foo/bar
//   $ curl -X POST localhost:4001/v2/keys/foo/bar
//   $ curl -X POST localhost:4001/v2/keys/foo/baz
//
func TestV2CreateUnique(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// POST should add index to list.
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar")
		resp, _ := tests.PostForm(fullURL, nil)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "create", "")

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo/bar/3", "")
		assert.Nil(t, node["dir"], "")
		assert.Equal(t, node["modifiedIndex"], 3, "")

		// Second POST should add next index to list.
		resp, _ = tests.PostForm(fullURL, nil)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)
		body = tests.ReadBodyJSON(resp)

		node = body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo/bar/4", "")

		// POST to a different key should add index to that list.
		resp, _ = tests.PostForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/baz"), nil)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)
		body = tests.ReadBodyJSON(resp)

		node = body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo/baz/5", "")
	})
}
