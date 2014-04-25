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
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo/bar","modifiedIndex":4,"createdIndex":3},"prevNode":{"key":"/foo/bar","value":"XXX","modifiedIndex":3,"createdIndex":3}}`, "")
	})
}

// Ensures that an empty directory is deleted when dir is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo?dir=true
//   $ curl -X DELETE localhost:4001/v2/keys/foo ->fail
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true
//
func TestV2DeleteEmptyDirectory(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true"), url.Values{})
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo"), url.Values{})
		assert.Equal(t, resp.StatusCode, http.StatusForbidden)
		bodyJson := tests.ReadBodyJSON(resp)
		assert.Equal(t, bodyJson["errorCode"], 102, "")
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true"), url.Values{})
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":4,"createdIndex":3},"prevNode":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":3}}`, "")
	})
}

// Ensures that a not-empty directory is deleted when dir is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar?dir=true
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true ->fail
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true&recursive=true
//
func TestV2DeleteNonEmptyDirectory(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?dir=true"), url.Values{})
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true"), url.Values{})
		assert.Equal(t, resp.StatusCode, http.StatusForbidden)
		bodyJson := tests.ReadBodyJSON(resp)
		assert.Equal(t, bodyJson["errorCode"], 108, "")
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true&recursive=true"), url.Values{})
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":4,"createdIndex":3},"prevNode":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":3}}`, "")
	})
}

// Ensures that a directory is deleted when recursive is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo?dir=true
//   $ curl -X DELETE localhost:4001/v2/keys/foo?recursive=true
//
func TestV2DeleteDirectoryRecursiveImpliesDir(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true"), url.Values{})
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?recursive=true"), url.Values{})
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":4,"createdIndex":3},"prevNode":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":3}}`, "")
	})
}

// Ensures that a key is deleted if the previous index matches
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?prevIndex=3
//
func TestV2DeleteKeyCADOnIndexSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?prevIndex=3"), url.Values{})
		assert.Nil(t, err, "")
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndDelete", "")

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["key"], "/foo", "")
		assert.Equal(t, node["modifiedIndex"], 4, "")
	})
}

// Ensures that a key is not deleted if the previous index does not match
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?prevIndex=100
//
func TestV2DeleteKeyCADOnIndexFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo"), v)
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?prevIndex=100"), url.Values{})
		assert.Nil(t, err, "")
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
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?prevIndex=bad_index"), v)
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
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?prevValue=XXX"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndDelete", "")

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["modifiedIndex"], 4, "")
	})
}

// Ensures that a key is not deleted if the previous value does not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevValue=YYY
//
func TestV2DeleteKeyCADOnValueFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?prevValue=YYY"), v)
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
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar?prevValue="), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 201)
	})
}
