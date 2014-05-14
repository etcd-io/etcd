package v1

import (
	"encoding/json"
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
//   $ curl localhost:4001/v1/keys/foo/bar -> fail
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//   $ curl localhost:4001/v1/keys/foo/bar
//
func TestV1GetKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar")
		resp, _ := tests.Get(fullURL)
		assert.Equal(t, resp.StatusCode, http.StatusNotFound)

		resp, _ = tests.PutForm(fullURL, v)
		tests.ReadBody(resp)

		resp, _ = tests.Get(fullURL)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "get", "")
		assert.Equal(t, body["key"], "/foo/bar", "")
		assert.Equal(t, body["value"], "XXX", "")
		assert.Equal(t, body["index"], 3, "")
	})
}

// Ensures that a directory of values can be retrieved for a given key.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/x -d value=XXX
//   $ curl -X PUT localhost:4001/v1/keys/foo/y/z -d value=YYY
//   $ curl localhost:4001/v1/keys/foo
//
func TestV1GetKeyDir(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/x"), v)
		tests.ReadBody(resp)

		v.Set("value", "YYY")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/y/z"), v)
		tests.ReadBody(resp)

		resp, _ = tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo"))
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		nodes := make([]interface{}, 0)
		if err := json.Unmarshal(body, &nodes); err != nil {
			panic(fmt.Sprintf("HTTP body JSON parse error: %v", err))
		}
		assert.Equal(t, len(nodes), 2, "")

		node0 := nodes[0].(map[string]interface{})
		assert.Equal(t, node0["action"], "get", "")
		assert.Equal(t, node0["key"], "/foo/x", "")
		assert.Equal(t, node0["value"], "XXX", "")

		node1 := nodes[1].(map[string]interface{})
		assert.Equal(t, node1["action"], "get", "")
		assert.Equal(t, node1["key"], "/foo/y", "")
		assert.Equal(t, node1["dir"], true, "")
	})
}

// Ensures that a watcher can wait for a value to be set and return it to the client.
//
//   $ curl localhost:4001/v1/watch/foo/bar
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//
func TestV1WatchKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// There exists a little gap between etcd ready to serve and
		// it actually serves the first request, which means the response
		// delay could be a little bigger.
		// This test is time sensitive, so it does one request to ensure
		// that the server is working.
		tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"))

		var watchResp *http.Response
		c := make(chan bool)
		go func() {
			watchResp, _ = tests.Get(fmt.Sprintf("%s%s", s.URL(), "/v1/watch/foo/bar"))
			c <- true
		}()

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)

		// Set a value.
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
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

		assert.Equal(t, body["key"], "/foo/bar", "")
		assert.Equal(t, body["value"], "XXX", "")
		assert.Equal(t, body["index"], 3, "")
	})
}

// Ensures that a watcher can wait for a value to be set after a given index.
//
//   $ curl -X POST localhost:4001/v1/watch/foo/bar -d index=4
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=YYY
//
func TestV1WatchKeyWithIndex(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		var body map[string]interface{}
		c := make(chan bool)
		go func() {
			v := url.Values{}
			v.Set("index", "4")
			resp, _ := tests.PostForm(fmt.Sprintf("%s%s", s.URL(), "/v1/watch/foo/bar"), v)
			body = tests.ReadBodyJSON(resp)
			c <- true
		}()

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value (before given index).
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
		tests.ReadBody(resp)

		// Make sure response didn't fire early.
		time.Sleep(1 * time.Millisecond)
		assert.Nil(t, body, "")

		// Set a value (before given index).
		v.Set("value", "YYY")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
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
		assert.Equal(t, body["index"], 4, "")
	})
}

// Ensures that HEAD works.
//
//   $ curl -I localhost:4001/v1/keys/foo/bar -> fail
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//   $ curl -I localhost:4001/v1/keys/foo/bar
//
func TestV1HeadKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar")
		resp, _ := tests.Get(fullURL)
		assert.Equal(t, resp.StatusCode, http.StatusNotFound)
		assert.Equal(t, resp.ContentLength, -1)

		resp, _ = tests.PutForm(fullURL, v)
		tests.ReadBody(resp)

		resp, _ = tests.Get(fullURL)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		assert.Equal(t, resp.ContentLength, -1)
	})
}
