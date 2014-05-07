package v1

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

		assert.Equal(t, string(body), `{"action":"set","key":"/foo/bar","value":"XXX","newKey":true,"index":3}`, "")
	})
}

// Ensures that a time-to-live is added to a key.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX -d ttl=20
//
func TestV1SetKeyWithTTL(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		t0 := time.Now()
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("ttl", "20")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["ttl"], 20, "")

		// Make sure the expiration date is correct.
		expiration, _ := time.Parse(time.RFC3339Nano, body["expiration"].(string))
		assert.Equal(t, expiration.Sub(t0)/time.Second, 20, "")
	})
}

// Ensures that an invalid time-to-live is returned as an error.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX -d ttl=bad_ttl
//
func TestV1SetKeyWithBadTTL(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("ttl", "bad_ttl")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
		assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 202, "")
		assert.Equal(t, body["message"], "The given TTL in POST form is not a number", "")
		assert.Equal(t, body["cause"], "Set", "")
	})
}

// Ensures that a key is conditionally set if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX -d prevValue=
//
func TestV1CreateKeySuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("prevValue", "")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar"), v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["value"], "XXX", "")
	})
}

// Ensures that a key is not conditionally set because it previously existed.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX -d prevValue=
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX -d prevValue= -> fail
//
func TestV1CreateKeyFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("prevValue", "")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar")
		resp, _ := tests.PutForm(fullURL, v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		tests.ReadBody(resp)
		resp, _ = tests.PutForm(fullURL, v)
		assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 105, "")
		assert.Equal(t, body["message"], "Key already exists", "")
		assert.Equal(t, body["cause"], "/foo/bar", "")
	})
}

// Ensures that a key is set only if the previous value matches.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=YYY -d prevValue=XXX
//
func TestV1SetKeyCASOnValueSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar")
		resp, _ := tests.PutForm(fullURL, v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		tests.ReadBody(resp)
		v.Set("value", "YYY")
		v.Set("prevValue", "XXX")
		resp, _ = tests.PutForm(fullURL, v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "testAndSet", "")
		assert.Equal(t, body["value"], "YYY", "")
		assert.Equal(t, body["index"], 4, "")
	})
}

// Ensures that a key is not set if the previous value does not match.
//
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v1/keys/foo/bar -d value=YYY -d prevValue=AAA
//
func TestV1SetKeyCASOnValueFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		fullURL := fmt.Sprintf("%s%s", s.URL(), "/v1/keys/foo/bar")
		resp, _ := tests.PutForm(fullURL, v)
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		tests.ReadBody(resp)
		v.Set("value", "YYY")
		v.Set("prevValue", "AAA")
		resp, _ = tests.PutForm(fullURL, v)
		assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 101, "")
		assert.Equal(t, body["message"], "Compare failed", "")
		assert.Equal(t, body["cause"], "[AAA != XXX]", "")
		assert.Equal(t, body["index"], 3, "")
	})
}
