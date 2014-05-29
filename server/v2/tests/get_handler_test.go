package v2

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures that a value can be retrieve for a given key.
//
//   $ curl localhost:4001/v2/keys/foo/bar -> fail
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl localhost:4001/v2/keys/foo/bar
//
func TestV2GetKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar")
		resp, _ := tests.Get(fullURL)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		assert.Equal(t, resp.StatusCode, http.StatusNotFound)

		resp, _ = tests.PutForm(fullURL, v)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		tests.ReadBody(resp)

		resp, _ = tests.Get(fullURL)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "get", "")
		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo/bar", "")
		assert.Equal(t, node["value"], "XXX", "")
		assert.Equal(t, node["modifiedIndex"], 3, "")
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
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/x"), v)
		tests.ReadBody(resp)

		v.Set("value", "YYY")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/y/z"), v)
		tests.ReadBody(resp)

		resp, _ = tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?recursive=true"))
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "get", "")
		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo", "")
		assert.Equal(t, node["dir"], true, "")
		assert.Equal(t, node["modifiedIndex"], 3, "")
		assert.Equal(t, len(node["nodes"].([]interface{})), 2, "")

		node0 := node["nodes"].([]interface{})[0].(map[string]interface{})
		assert.Equal(t, node0["key"], "/foo/x", "")
		assert.Equal(t, node0["value"], "XXX", "")
		assert.Equal(t, node0["ttl"], 10, "")

		node1 := node["nodes"].([]interface{})[1].(map[string]interface{})
		assert.Equal(t, node1["key"], "/foo/y", "")
		assert.Equal(t, node1["dir"], true, "")

		node2 := node1["nodes"].([]interface{})[0].(map[string]interface{})
		assert.Equal(t, node2["key"], "/foo/y/z", "")
		assert.Equal(t, node2["value"], "YYY", "")
	})
}

// Ensures that a watcher can wait for a value to be set and return it to the client.
//
//   $ curl localhost:4001/v2/keys/foo/bar?wait=true
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//
func TestV2WatchKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// There exists a little gap between etcd ready to serve and
		// it actually serves the first request, which means the response
		// delay could be a little bigger.
		// This test is time sensitive, so it does one request to ensure
		// that the server is working.
		tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"))

		var watchResp *http.Response
		c := make(chan bool)
		go func() {
			watchResp, _ = tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?wait=true"))
			c <- true
		}()

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)

		// Set a value.
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)

		// A response should follow from the GET above.
		time.Sleep(1 * time.Millisecond)

		select {
		case <-c:

		default:
			t.Fatal("cannot get watch result")
		}

		body := tests.ReadBodyJSON(watchResp)
		assert.NotNil(t, body, "")
		assert.Equal(t, body["action"], "set", "")

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo/bar", "")
		assert.Equal(t, node["value"], "XXX", "")
		assert.Equal(t, node["modifiedIndex"], 3, "")
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
			resp, _ := tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?wait=true&waitIndex=4"))
			body = tests.ReadBodyJSON(resp)
			c <- true
		}()

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value (before given index).
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value (before given index).
		v.Set("value", "YYY")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
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

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo/bar", "")
		assert.Equal(t, node["value"], "YYY", "")
		assert.Equal(t, node["modifiedIndex"], 4, "")
	})
}

// Ensures that a watcher can wait for a value to be set after a given index.
//
//   $ curl localhost:4001/v2/keys/keyindir/bar?wait=true
//   $ curl -X PUT localhost:4001/v2/keys/keyindir -d dir=true -d ttl=1
//   $ curl -X PUT localhost:4001/v2/keys/keyindir/bar -d value=YYY
//
func TestV2WatchKeyInDir(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		var body map[string]interface{}
		c := make(chan bool)

		// Set a value (before given index).
		v := url.Values{}
		v.Set("dir", "true")
		v.Set("ttl", "1")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/keyindir"), v)
		tests.ReadBody(resp)

		// Set a value (before given index).
		v = url.Values{}
		v.Set("value", "XXX")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/keyindir/bar"), v)
		tests.ReadBody(resp)

		go func() {
			resp, _ := tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/keyindir/bar?wait=true"))
			body = tests.ReadBodyJSON(resp)
			c <- true
		}()

		// wait for expiration, we do have a up to 500 millisecond delay
		time.Sleep(2000 * time.Millisecond)

		select {
		case <-c:

		default:
			t.Fatal("cannot get watch result")
		}

		assert.NotNil(t, body, "")
		assert.Equal(t, body["action"], "expire", "")

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/keyindir", "")
	})
}

// Ensures that HEAD could work.
//
//   $ curl -I localhost:4001/v2/keys/foo/bar -> fail
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -I localhost:4001/v2/keys/foo/bar
//
func TestV2HeadKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar")
		resp, _ := tests.Head(fullURL)
		assert.Equal(t, resp.StatusCode, http.StatusNotFound)
		assert.Equal(t, resp.ContentLength, -1)

		resp, _ = tests.PutForm(fullURL, v)
		tests.ReadBody(resp)

		resp, _ = tests.Head(fullURL)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		assert.Equal(t, resp.ContentLength, -1)
	})
}
