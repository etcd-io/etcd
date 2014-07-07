package etcd

// Ensures that a value can be retrieve for a given key.

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures that a directory is created
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar?dir=true
//
func TestV2SetDirectory(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body := ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, string(body), `{"action":"set","node":{"key":"/foo","dir":true,"modifiedIndex":2,"createdIndex":2}}`, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a time-to-live is added to a key.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d ttl=20
//
func TestV2SetKeyWithTTL(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	t0 := time.Now()
	v := url.Values{}
	v.Set("value", "XXX")
	v.Set("ttl", "20")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body := ReadBodyJSON(resp)
	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["ttl"], 20, "")

	// Make sure the expiration date is correct.
	expiration, _ := time.Parse(time.RFC3339Nano, node["expiration"].(string))
	assert.Equal(t, expiration.Sub(t0)/time.Second, 20, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that an invalid time-to-live is returned as an error.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d ttl=bad_ttl
//
func TestV2SetKeyWithBadTTL(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	v.Set("ttl", "bad_ttl")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 202, "")
	assert.Equal(t, body["message"], "The given TTL in POST form is not a number", "")
	assert.Equal(t, body["cause"], "Update", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is conditionally set if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=false
//
func TestV2CreateKeySuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	v.Set("prevExist", "false")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body := ReadBodyJSON(resp)
	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["value"], "XXX", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not conditionally set because it previously existed.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=false
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=false -> fail
//
func TestV2CreateKeyFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	v.Set("prevExist", "false")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 105, "")
	assert.Equal(t, body["message"], "Key already exists", "")
	assert.Equal(t, body["cause"], "/foo/bar", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is conditionally set only if it previously did exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevExist=true
//
func TestV2UpdateKeySuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}

	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)

	v.Set("value", "YYY")
	v.Set("prevExist", "true")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "update", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not conditionally set if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo?dir=true
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevExist=true
//
func TestV2UpdateKeyFailOnValue(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true"), v)
	resp.Body.Close()

	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	v.Set("value", "YYY")
	v.Set("prevExist", "true")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 100, "")
	assert.Equal(t, body["message"], "Key not found", "")
	assert.Equal(t, body["cause"], "/foo/bar", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not conditionally set if it previously did not exist.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=YYY -d prevExist=true -> fail
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevExist=true -> fail
//
func TestV2UpdateKeyFailOnMissingDirectory(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "YYY")
	v.Set("prevExist", "true")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 100, "")
	assert.Equal(t, body["message"], "Key not found", "")
	assert.Equal(t, body["cause"], "/foo", "")

	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	body = ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 100, "")
	assert.Equal(t, body["message"], "Key not found", "")
	assert.Equal(t, body["cause"], "/foo", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key could update TTL.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX -d ttl=1000 -d prevExist=true
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX -d ttl= -d prevExist=true
//
func TestV2UpdateKeySuccessWithTTL(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	node := (ReadBodyJSON(resp)["node"]).(map[string]interface{})
	createdIndex := node["createdIndex"]

	v.Set("ttl", "1000")
	v.Set("prevExist", "true")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	node = (ReadBodyJSON(resp)["node"]).(map[string]interface{})
	assert.Equal(t, node["value"], "XXX", "")
	assert.Equal(t, node["ttl"], 1000, "")
	assert.NotEqual(t, node["expiration"], "", "")
	assert.Equal(t, node["createdIndex"], createdIndex, "")

	v.Del("ttl")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	node = (ReadBodyJSON(resp)["node"]).(map[string]interface{})
	assert.Equal(t, node["value"], "XXX", "")
	assert.Equal(t, node["ttl"], nil, "")
	assert.Equal(t, node["expiration"], nil, "")
	assert.Equal(t, node["createdIndex"], createdIndex, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is set only if the previous index matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevIndex=1
//
func TestV2SetKeyCASOnIndexSuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)

	v.Set("value", "YYY")
	v.Set("prevIndex", "2")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "compareAndSwap", "")
	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["value"], "YYY", "")
	assert.Equal(t, node["modifiedIndex"], 3, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not set if the previous index does not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevIndex=10
//
func TestV2SetKeyCASOnIndexFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	v.Set("value", "YYY")
	v.Set("prevIndex", "10")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101, "")
	assert.Equal(t, body["message"], "Compare failed", "")
	assert.Equal(t, body["cause"], "[10 != 2]", "")
	assert.Equal(t, body["index"], 2, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that an error is thrown if an invalid previous index is provided.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevIndex=bad_index
//
func TestV2SetKeyCASWithInvalidIndex(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "YYY")
	v.Set("prevIndex", "bad_index")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 203, "")
	assert.Equal(t, body["message"], "The given index in POST form is not a number", "")
	assert.Equal(t, body["cause"], "CompareAndSwap", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is set only if the previous value matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=XXX
//
func TestV2SetKeyCASOnValueSuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	v.Set("value", "YYY")
	v.Set("prevValue", "XXX")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "compareAndSwap", "")
	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["value"], "YYY", "")
	assert.Equal(t, node["modifiedIndex"], 3, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not set if the previous value does not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=AAA
//
func TestV2SetKeyCASOnValueFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	v.Set("value", "YYY")
	v.Set("prevValue", "AAA")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101, "")
	assert.Equal(t, body["message"], "Compare failed", "")
	assert.Equal(t, body["cause"], "[AAA != XXX]", "")
	assert.Equal(t, body["index"], 2, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that an error is returned if a blank prevValue is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX -d prevValue=
//
func TestV2SetKeyCASWithMissingValueFails(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	v.Set("prevValue", "")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 201, "")
	assert.Equal(t, body["message"], "PrevValue is Required in POST form", "")
	assert.Equal(t, body["cause"], "CompareAndSwap", "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not set if both previous value and index do not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=AAA -d prevIndex=4
//
func TestV2SetKeyCASOnValueAndIndexFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	v.Set("value", "YYY")
	v.Set("prevValue", "AAA")
	v.Set("prevIndex", "4")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101, "")
	assert.Equal(t, body["message"], "Compare failed", "")
	assert.Equal(t, body["cause"], "[AAA != XXX] [4 != 2]", "")
	assert.Equal(t, body["index"], 2, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not set if previous value match but index does not.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=XXX -d prevIndex=4
//
func TestV2SetKeyCASOnValueMatchAndIndexFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	v.Set("value", "YYY")
	v.Set("prevValue", "XXX")
	v.Set("prevIndex", "4")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101, "")
	assert.Equal(t, body["message"], "Compare failed", "")
	assert.Equal(t, body["cause"], "[4 != 2]", "")
	assert.Equal(t, body["index"], 2, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not set if previous index matches but value does not.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY -d prevValue=AAA -d prevIndex=3
//
func TestV2SetKeyCASOnIndexMatchAndValueFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	ReadBody(resp)
	v.Set("value", "YYY")
	v.Set("prevValue", "AAA")
	v.Set("prevIndex", "2")
	resp, _ = PutForm(fullURL, v)
	assert.Equal(t, resp.StatusCode, http.StatusPreconditionFailed)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101, "")
	assert.Equal(t, body["message"], "Compare failed", "")
	assert.Equal(t, body["cause"], "[AAA != XXX]", "")
	assert.Equal(t, body["index"], 2, "")
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensure that we can set an empty value
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=
//
func TestV2SetKeyCASWithEmptyValueSuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL
	v := url.Values{}
	v.Set("value", "")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body := ReadBody(resp)
	assert.Equal(t, string(body), `{"action":"set","node":{"key":"/foo/bar","value":"","modifiedIndex":2,"createdIndex":2}}`)
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

func TestV2SetKey(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body := ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, string(body), `{"action":"set","node":{"key":"/foo/bar","value":"XXX","modifiedIndex":2,"createdIndex":2}}`, "")

	resp.Body.Close()
	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

func TestV2SetKeyRedirect(t *testing.T) {
	es, hs := buildCluster(3)
	waitCluster(t, es)
	u := hs[1].URL
	ru := fmt.Sprintf("%s%s", hs[0].URL, "/v2/keys/foo/bar")

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	assert.Equal(t, resp.StatusCode, http.StatusTemporaryRedirect)
	location, err := resp.Location()
	if err != nil {
		t.Errorf("want err = %, want nil", err)
	}

	if location.String() != ru {
		t.Errorf("location = %v, want %v", location.String(), ru)
	}

	resp.Body.Close()
	for i := range es {
		es[len(es)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
	afterTest(t)
}

// Ensures that a key is deleted.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar
//
func TestV2DeleteKey(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	resp.Body.Close()
	ReadBody(resp)

	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo/bar","modifiedIndex":3,"createdIndex":2},"prevNode":{"key":"/foo/bar","value":"XXX","modifiedIndex":2,"createdIndex":2}}`, "")
	resp.Body.Close()

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that an empty directory is deleted when dir is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo?dir=true
//   $ curl -X DELETE localhost:4001/v2/keys/foo ->fail
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true
//
func TestV2DeleteEmptyDirectory(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true"), url.Values{})
	resp.Body.Close()

	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusForbidden)
	bodyJson := ReadBodyJSON(resp)
	assert.Equal(t, bodyJson["errorCode"], 102, "")
	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":2},"prevNode":{"key":"/foo","dir":true,"modifiedIndex":2,"createdIndex":2}}`, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a not-empty directory is deleted when dir is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar?dir=true
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true ->fail
//   $ curl -X DELETE localhost:4001/v2/keys/foo?dir=true&recursive=true
//
func TestV2DeleteNonEmptyDirectory(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?dir=true"), url.Values{})
	ReadBody(resp)
	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusForbidden)
	bodyJson := ReadBodyJSON(resp)
	assert.Equal(t, bodyJson["errorCode"], 108, "")
	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true&recursive=true"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":2},"prevNode":{"key":"/foo","dir":true,"modifiedIndex":2,"createdIndex":2}}`, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a directory is deleted when recursive is set.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo?dir=true
//   $ curl -X DELETE localhost:4001/v2/keys/foo?recursive=true
//
func TestV2DeleteDirectoryRecursiveImpliesDir(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?dir=true"), url.Values{})
	ReadBody(resp)
	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?recursive=true"), url.Values{})
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, string(body), `{"action":"delete","node":{"key":"/foo","dir":true,"modifiedIndex":3,"createdIndex":2},"prevNode":{"key":"/foo","dir":true,"modifiedIndex":2,"createdIndex":2}}`, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is deleted if the previous index matches
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?prevIndex=3
//
func TestV2DeleteKeyCADOnIndexSuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	ReadBody(resp)
	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?prevIndex=2"), url.Values{})
	assert.Nil(t, err, "")
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "compareAndDelete", "")

	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo", "")
	assert.Equal(t, node["modifiedIndex"], 3, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not deleted if the previous index does not match
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo?prevIndex=100
//
func TestV2DeleteKeyCADOnIndexFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, err := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	ReadBody(resp)
	resp, err = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo?prevIndex=100"), url.Values{})
	assert.Nil(t, err, "")
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101)

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that an error is thrown if an invalid previous index is provided.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevIndex=bad_index
//
func TestV2DeleteKeyCADWithInvalidIndex(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)
	resp, _ = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?prevIndex=bad_index"), v)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 203)

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is deleted only if the previous value matches.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevValue=XXX
//
func TestV2DeleteKeyCADOnValueSuccess(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)
	resp, _ = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?prevValue=XXX"), v)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "compareAndDelete", "")

	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["modifiedIndex"], 3, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a key is not deleted if the previous value does not match.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevValue=YYY
//
func TestV2DeleteKeyCADOnValueFail(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)
	resp, _ = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?prevValue=YYY"), v)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 101)

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that an error is thrown if an invalid previous value is provided.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X DELETE localhost:4001/v2/keys/foo/bar?prevIndex=
//
func TestV2DeleteKeyCADWithInvalidValue(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)
	resp, _ = DeleteForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?prevValue="), v)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["errorCode"], 201)

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures a unique value is added to the key's children.
//
//   $ curl -X POST localhost:4001/v2/keys/foo/bar
//   $ curl -X POST localhost:4001/v2/keys/foo/bar
//   $ curl -X POST localhost:4001/v2/keys/foo/baz
//
func TestV2CreateUnique(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	// POST should add index to list.
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := PostForm(fullURL, nil)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "create", "")

	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo/bar/2", "")
	assert.Nil(t, node["dir"], "")
	assert.Equal(t, node["modifiedIndex"], 2, "")

	// Second POST should add next index to list.
	resp, _ = PostForm(fullURL, nil)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body = ReadBodyJSON(resp)

	node = body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo/bar/3", "")

	// POST to a different key should add index to that list.
	resp, _ = PostForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/baz"), nil)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	body = ReadBodyJSON(resp)

	node = body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo/baz/4", "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

//
//   $ curl localhost:4001/v2/keys/foo/bar -> fail
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl localhost:4001/v2/keys/foo/bar
//
func TestV2GetKey(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := Get(fullURL)
	resp.Body.Close()

	resp, _ = PutForm(fullURL, v)
	resp.Body.Close()

	resp, _ = Get(fullURL)
	assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBodyJSON(resp)
	resp.Body.Close()
	assert.Equal(t, body["action"], "get", "")
	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo/bar", "")
	assert.Equal(t, node["value"], "XXX", "")
	assert.Equal(t, node["modifiedIndex"], 2, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a directory of values can be recursively retrieved for a given key.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo/x -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/y/z -d value=YYY
//   $ curl localhost:4001/v2/keys/foo -d recursive=true
//
func TestV2GetKeyRecursively(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	v.Set("ttl", "10")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/x"), v)
	ReadBody(resp)

	v.Set("value", "YYY")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/y/z"), v)
	ReadBody(resp)

	resp, _ = Get(fmt.Sprintf("%s%s", u, "/v2/keys/foo?recursive=true"))
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	body := ReadBodyJSON(resp)
	assert.Equal(t, body["action"], "get", "")
	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo", "")
	assert.Equal(t, node["dir"], true, "")
	assert.Equal(t, node["modifiedIndex"], 2, "")
	assert.Equal(t, len(node["nodes"].([]interface{})), 2, "")

	// TODO(xiangli): fix the wrong assumption here.
	// the order of nodes map cannot be determined.
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

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a watcher can wait for a value to be set and return it to the client.
//
//   $ curl localhost:4001/v2/keys/foo/bar?wait=true
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//
func TestV2WatchKey(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	// There exists a little gap between etcd ready to serve and
	// it actually serves the first request, which means the response
	// delay could be a little bigger.
	// This test is time sensitive, so it does one request to ensure
	// that the server is working.
	resp, _ := Get(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"))
	resp.Body.Close()

	var watchResp *http.Response
	c := make(chan bool)
	go func() {
		watchResp, _ = Get(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?wait=true"))
		c <- true
	}()

	// Make sure response didn't fire early.
	time.Sleep(1 * time.Millisecond)

	// Set a value.
	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)

	// A response should follow from the GET above.
	time.Sleep(1 * time.Millisecond)

	select {
	case <-c:
	default:
		t.Fatal("cannot get watch result")
	}

	body := ReadBodyJSON(watchResp)
	watchResp.Body.Close()
	assert.NotNil(t, body, "")
	assert.Equal(t, body["action"], "set", "")

	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/foo/bar", "")
	assert.Equal(t, node["value"], "XXX", "")
	assert.Equal(t, node["modifiedIndex"], 2, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a watcher can wait for a value to be set after a given index.
//
//   $ curl localhost:4001/v2/keys/foo/bar?wait=true&waitIndex=3
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=YYY
//
func TestV2WatchKeyWithIndex(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	var body map[string]interface{}
	c := make(chan bool)
	go func() {
		resp, _ := Get(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar?wait=true&waitIndex=3"))
		body = ReadBodyJSON(resp)
		c <- true
	}()

	// Make sure response didn't fire early.
	time.Sleep(1 * time.Millisecond)
	assert.Nil(t, body, "")

	// Set a value (before given index).
	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)

	// Make sure response didn't fire early.
	time.Sleep(1 * time.Millisecond)
	assert.Nil(t, body, "")

	// Set a value (before given index).
	v.Set("value", "YYY")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar"), v)
	ReadBody(resp)

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
	assert.Equal(t, node["modifiedIndex"], 3, "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that a watcher can wait for a value to be set after a given index.
//
//   $ curl localhost:4001/v2/keys/keyindir/bar?wait=true
//   $ curl -X PUT localhost:4001/v2/keys/keyindir -d dir=true -d ttl=1
//   $ curl -X PUT localhost:4001/v2/keys/keyindir/bar -d value=YYY
//
func TestV2WatchKeyInDir(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	var body map[string]interface{}
	c := make(chan bool)

	// Set a value (before given index).
	v := url.Values{}
	v.Set("dir", "true")
	v.Set("ttl", "1")
	resp, _ := PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/keyindir"), v)
	ReadBody(resp)

	// Set a value (before given index).
	v = url.Values{}
	v.Set("value", "XXX")
	resp, _ = PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/keyindir/bar"), v)
	ReadBody(resp)

	go func() {
		resp, _ := Get(fmt.Sprintf("%s%s", u, "/v2/keys/keyindir/bar?wait=true"))
		body = ReadBodyJSON(resp)
		c <- true
	}()

	// wait for expiration, we do have a up to 500 millisecond delay
	time.Sleep(time.Second + time.Millisecond*500)

	select {
	case <-c:
	default:
		t.Fatal("cannot get watch result")
	}

	assert.NotNil(t, body, "")
	assert.Equal(t, body["action"], "expire", "")

	node := body["node"].(map[string]interface{})
	assert.Equal(t, node["key"], "/keyindir", "")

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}

// Ensures that HEAD could work.
//
//   $ curl -I localhost:4001/v2/keys/foo/bar -> fail
//   $ curl -X PUT localhost:4001/v2/keys/foo/bar -d value=XXX
//   $ curl -I localhost:4001/v2/keys/foo/bar
//
func TestV2HeadKey(t *testing.T) {
	es, hs := buildCluster(1)
	u := hs[0].URL

	v := url.Values{}
	v.Set("value", "XXX")
	fullURL := fmt.Sprintf("%s%s", u, "/v2/keys/foo/bar")
	resp, _ := Head(fullURL)
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	assert.Equal(t, resp.ContentLength, -1)

	resp, _ = PutForm(fullURL, v)
	ReadBody(resp)

	resp, _ = Head(fullURL)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, resp.ContentLength, -1)

	es[0].Stop()
	hs[0].Close()
	afterTest(t)
}
