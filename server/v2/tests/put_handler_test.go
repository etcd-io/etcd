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

// Ensures that a key is set to a given value.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//
func TestV2SetKey(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, err := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBody(resp)
		assert.Nil(t, err, "")
		assert.Equal(t, string(body), `{"action":"set","node":{"key":"/foo/bar","value":"XXX","modifiedIndex":2,"createdIndex":2}}`, "")
	})
}

// Ensures that a time-to-live is added to a key.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d ttl=20
//
func TestV2SetKeyWithTTL(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		t0 := time.Now()
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("ttl", "20")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["ttl"], 20, "")

		// Make sure the expiration date is correct.
		expiration, _ := time.Parse(time.RFC3339Nano, node["expiration"].(string))
		assert.Equal(t, expiration.Sub(t0)/time.Second, 20, "")
	})
}

// Ensures that an invalid time-to-live is returned as an error.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d ttl=bad_ttl
//
func TestV2SetKeyWithBadTTL(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("ttl", "bad_ttl")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 202, "")
		assert.Equal(t, body["message"], "The given TTL in POST form is not a number", "")
		assert.Equal(t, body["cause"], "Update", "")
	})
}

// Ensures that a key is conditionally set only if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=false
//
func TestV2CreateKeySuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("prevExist", "false")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["value"], "XXX", "")
	})
}

// Ensures that a key is not conditionally because it previously existed.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=false
//
func TestV2CreateKeyFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("prevExist", "false")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 105, "")
		assert.Equal(t, body["message"], "Already exists", "")
		assert.Equal(t, body["cause"], "/foo/bar", "")
	})
}

// Ensures that a key is conditionally set only if it previously did exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevExist=true
//
func TestV2UpdateKeySuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}

		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)

		v.Set("value", "YYY")
		v.Set("prevExist", "true")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "update", "")

		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["prevValue"], "XXX", "")
	})
}

// Ensures that a key is not conditionally set if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=true
//
func TestV2UpdateKeyFailOnValue(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo"), v)

		v.Set("value", "YYY")
		v.Set("prevExist", "true")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 100, "")
		assert.Equal(t, body["message"], "Key Not Found", "")
		assert.Equal(t, body["cause"], "/foo/bar", "")
	})
}

// Ensures that a key is not conditionally set if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX -d prevExist=true
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=true
//
func TestV2UpdateKeyFailOnMissingDirectory(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "YYY")
		v.Set("prevExist", "true")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 100, "")
		assert.Equal(t, body["message"], "Key Not Found", "")
		assert.Equal(t, body["cause"], "/foo", "")
	})
}

// Ensures that a key is set only if the previous index matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevIndex=1
//
func TestV2SetKeyCASOnIndexSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		v.Set("value", "YYY")
		v.Set("prevIndex", "2")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndSwap", "")
		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["prevValue"], "XXX", "")
		assert.Equal(t, node["value"], "YYY", "")
		assert.Equal(t, node["modifiedIndex"], 3, "")
	})
}

// Ensures that a key is not set if the previous index does not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevIndex=10
//
func TestV2SetKeyCASOnIndexFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		v.Set("value", "YYY")
		v.Set("prevIndex", "10")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 101, "")
		assert.Equal(t, body["message"], "Test Failed", "")
		assert.Equal(t, body["cause"], "[ != XXX] [10 != 2]", "")
		assert.Equal(t, body["index"], 2, "")
	})
}

// Ensures that an error is thrown if an invalid previous index is provided.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevIndex=bad_index
//
func TestV2SetKeyCASWithInvalidIndex(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "YYY")
		v.Set("prevIndex", "bad_index")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 203, "")
		assert.Equal(t, body["message"], "The given index in POST form is not a number", "")
		assert.Equal(t, body["cause"], "CompareAndSwap", "")
	})
}

// Ensures that a key is set only if the previous value matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=XXX
//
func TestV2SetKeyCASOnValueSuccess(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		v.Set("value", "YYY")
		v.Set("prevValue", "XXX")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["action"], "compareAndSwap", "")
		node := body["node"].(map[string]interface{})
		assert.Equal(t, node["prevValue"], "XXX", "")
		assert.Equal(t, node["value"], "YYY", "")
		assert.Equal(t, node["modifiedIndex"], 3, "")
	})
}

// Ensures that a key is not set if the previous value does not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=AAA
//
func TestV2SetKeyCASOnValueFail(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		tests.ReadBody(resp)
		v.Set("value", "YYY")
		v.Set("prevValue", "AAA")
		resp, _ = tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 101, "")
		assert.Equal(t, body["message"], "Test Failed", "")
		assert.Equal(t, body["cause"], "[AAA != XXX] [0 != 2]", "")
		assert.Equal(t, body["index"], 2, "")
	})
}

// Ensures that an error is returned if a blank prevValue is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevValue=
//
func TestV2SetKeyCASWithMissingValueFails(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "XXX")
		v.Set("prevValue", "")
		resp, _ := tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo/bar"), v)
		body := tests.ReadBodyJSON(resp)
		assert.Equal(t, body["errorCode"], 201, "")
		assert.Equal(t, body["message"], "PrevValue is Required in POST form", "")
		assert.Equal(t, body["cause"], "CompareAndSwap", "")
	})
}
