package v2

import (
	"fmt"
	"testing"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/stretchr/testify/assert"
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
		resp, _ := tests.PostForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), nil)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "create", "")
		assert.Equal(t, body["key"], "/foo/bar/1", "")
		assert.Equal(t, body["dir"], true, "")
		assert.Equal(t, body["modifiedIndex"], 1, "")

		// Second POST should add next index to list.
		resp, _ = tests.PostForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), nil)
		body = tests.ReadBodyJSON(resp)
		assert.Equal(t, body["key"], "/foo/bar/2", "")

		// POST to a different key should add index to that list.
		resp, _ = tests.PostForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/baz"), nil)
		body = tests.ReadBodyJSON(resp)
		assert.Equal(t, body["key"], "/foo/baz/3", "")
	})
}
