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

// Ensures that an empty directory is deleted when dir is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo?dir=true
//   $ curl -X PUT localhost:4001/v2/keys/foo ->fail
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true
//
func TestV2DeleteEmptyDirectory(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true"), url.Values{})
		tests.ReadBody(resp)
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo"), url.Values{})
		bodyJson := tests.ReadBodyJSON(resp)
		assert.Equal(t, bodyJson["errorCode"], 102, "")
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true"), url.Values{})
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":2}}`, "")
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
		bodyJson := tests.ReadBodyJSON(resp)
		assert.Equal(t, bodyJson["errorCode"], 108, "")
		resp, err = tests.DeleteForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo?dir=true&recursive=true"), url.Values{})
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":2}}`, "")
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
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":2}}`, "")
	})
}
