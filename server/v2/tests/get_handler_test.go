package v2

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/stretchr/testify/assert"
)

// Ensures that a value can be retrieve for a given key.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl localhost:4001/v2/keys/foo/bar
//
func TestV2GetKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.Get(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"))
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "get", "")
		assert.Equal(t, body["key"], "/foo/bar", "")
		assert.Equal(t, body["value"], "XXX", "")
		assert.Equal(t, body["modifiedIndex"], 1, "")
	})
}

// Ensures that a directory of values can be recursively retrieved for a given key.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/x -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/y/z -d value=YYY
//   $ curl localhost:4001/v2/keys/foo -d recursive=true
//
func TestV2GetKeyRecursively(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("ttl", "10")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/x"), v)
		tests.ReadBody(resp)

		v.Set("value", "YYY")
		resp, _ = tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/y/z"), v)
		tests.ReadBody(resp)

		resp, _ = tests.Get(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo?recursive=true"))
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "get", "")
		assert.Equal(t, body["key"], "/foo", "")
		assert.Equal(t, body["dir"], true, "")
		assert.Equal(t, body["modifiedIndex"], 1, "")
		assert.Equal(t, len(body["kvs"].([]interface{})), 2, "")

		kv0 := body["kvs"].([]interface{})[0].(map[string]interface{})
		assert.Equal(t, kv0["key"], "/foo/x", "")
		assert.Equal(t, kv0["value"], "XXX", "")
		assert.Equal(t, kv0["ttl"], 10, "")

		kv1 := body["kvs"].([]interface{})[1].(map[string]interface{})
		assert.Equal(t, kv1["key"], "/foo/y", "")
		assert.Equal(t, kv1["dir"], true, "")

		kvs2 := kv1["kvs"].([]interface{})[0].(map[string]interface{})
		assert.Equal(t, kvs2["key"], "/foo/y/z", "")
		assert.Equal(t, kvs2["value"], "YYY", "")
	})
}

// Ensures that a watcher can wait for a value to be set and return it to the client.
//
//   $ curl localhost:4001/v2/keys/foo/bar?wait=true
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//
func TestV2WatchKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		var body map[string]interface{}
		c := make(chan bool)
		go func() {
			resp, _ := tests.Get(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?wait=true"))
			body = tests.ReadBodyJSON(resp)
			c <- true
		}()

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value.
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)

		// A response should follow from the GET above.
		time.Sleep(1 * time.Millisecond)

		select {
		case <-c:

		default:
			t.Fatal("cannot get watch result")
		}

		assert.NotNil(t, body, "")
		assert.Equal(t, body["action"], "set", "")
		assert.Equal(t, body["key"], "/foo/bar", "")
		assert.Equal(t, body["value"], "XXX", "")
		assert.Equal(t, body["modifiedIndex"], 1, "")
	})
}

// Ensures that a watcher can wait for a value to be set after a given index.
//
//   $ curl localhost:4001/v2/keys/foo/bar?wait=true&waitIndex=4
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY
//
func TestV2WatchKeyWithIndex(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		var body map[string]interface{}
		c := make(chan bool)
		go func() {
			resp, _ := tests.Get(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?wait=true&waitIndex=2"))
			body = tests.ReadBodyJSON(resp)
			c <- true
		}()

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value (before given index).
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value (before given index).
		v.Set("value", "YYY")
		resp, _ = tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)

		// A response should follow from the GET above.
		time.Sleep(1 * time.Millisecond)

		select {
		case <-c:

		default:
			t.Fatal("cannot get watch result")
		}

		assert.NotNil(t, body, "")
		assert.Equal(t, body["action"], "set", "")
		assert.Equal(t, body["key"], "/foo/bar", "")
		assert.Equal(t, body["value"], "YYY", "")
		assert.Equal(t, body["modifiedIndex"], 2, "")
	})
}
