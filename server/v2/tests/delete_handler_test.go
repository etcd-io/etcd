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
		resp, err := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), url.Values{})
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","key":"/foo/bar","prevValue":"XXX","modifiedIndex":2}`, "")
	})
}

// Ensures that a directory is deleted.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?recursive=true
//
func TestV2DeleteDirectory(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo?recursive=true"), url.Values{})
		assert.Nil(t, err, "")
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "delete", "")
		assert.Equal(t, body["dir"], true, "")
		assert.Equal(t, body["modifiedIndex"], 2, "")
	})
}

// Ensures that a directory is deleted if the previous index matches
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?recursive=true&prevIndex=1
//
func TestV2DeleteDirectoryCADOnIndexSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo?recursive=true&prevIndex=1"), url.Values{})
		assert.Nil(t, err, "")
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndDelete", "")
		assert.Equal(t, body["dir"], true, "")
		assert.Equal(t, body["modifiedIndex"], 2, "")
	})
}

// Ensures that a directory is not deleted if the previous index does not match
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?recursive=true&prevIndex=100
//
func TestV2DeleteDirectoryCADOnIndexFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo?recursive=true&prevIndex=100"), url.Values{})
		assert.Nil(t, err, "")
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 101)
	})
}

// Ensures that a key is deleted only if the previous index matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevIndex=1
//
func TestV2DeleteKeyCADOnIndexSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?prevIndex=1"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndDelete", "")
		assert.Equal(t, body["prevValue"], "XXX", "")
		assert.Equal(t, body["modifiedIndex"], 2, "")
	})
}

// Ensures that a key is not deleted if the previous index does not matche.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevIndex=2
//
func TestV2DeleteKeyCADOnIndexFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?prevIndex=100"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 101)
	})
}

// Ensures that an error is thrown if an invalid previous index is provided.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevIndex=bad_index
//
func TestV2DeleteKeyCADWithInvalidIndex(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?prevIndex=bad_index"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 203)
	})
}

// Ensures that a key is deleted only if the previous value matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevValue=XXX
//
func TestV2DeleteKeyCADOnValueSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?prevValue=XXX"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndDelete", "")
		assert.Equal(t, body["prevValue"], "XXX", "")
		assert.Equal(t, body["modifiedIndex"], 2, "")
	})
}

// Ensures that a key is not deleted if the previous value does not matche.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevValue=YYY
//
func TestV2DeleteKeyCADOnValueFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?prevValue=YYY"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 101)
	})
}

// Ensures that an error is thrown if an invalid previous value is provided.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevIndex=
//
func TestV2DeleteKeyCADWithInvalidValue(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("http://%s%s", s.URL(), "/v2/keys/foo/bar?prevValue="), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 201)
	})
}
