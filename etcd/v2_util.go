package etcd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type testHttpClient struct {
	*http.Client
}

// Creates a new HTTP client with KeepAlive disabled.
func NewTestClient() *testHttpClient {
	return &testHttpClient{&http.Client{Transport: &http.Transport{DisableKeepAlives: true}}}
}

// Reads the body from the response and closes it.
func (t *testHttpClient) ReadBody(resp *http.Response) []byte {
	if resp == nil {
		return []byte{}
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return body
}

// Reads the body from the response and parses it as JSON.
func (t *testHttpClient) ReadBodyJSON(resp *http.Response) map[string]interface{} {
	m := make(map[string]interface{})
	b := t.ReadBody(resp)
	if err := json.Unmarshal(b, &m); err != nil {
		panic(fmt.Sprintf("HTTP body JSON parse error: %v: %s", err, string(b)))
	}
	return m
}

func (t *testHttpClient) Head(url string) (*http.Response, error) {
	return t.send("HEAD", url, "application/json", nil)
}

func (t *testHttpClient) Get(url string) (*http.Response, error) {
	return t.send("GET", url, "application/json", nil)
}

func (t *testHttpClient) Post(url string, bodyType string, body io.Reader) (*http.Response, error) {
	return t.send("POST", url, bodyType, body)
}

func (t *testHttpClient) PostForm(url string, data url.Values) (*http.Response, error) {
	return t.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func (t *testHttpClient) Put(url string, bodyType string, body io.Reader) (*http.Response, error) {
	return t.send("PUT", url, bodyType, body)
}

func (t *testHttpClient) PutForm(url string, data url.Values) (*http.Response, error) {
	return t.Put(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func (t *testHttpClient) Delete(url string, bodyType string, body io.Reader) (*http.Response, error) {
	return t.send("DELETE", url, bodyType, body)
}

func (t *testHttpClient) DeleteForm(url string, data url.Values) (*http.Response, error) {
	return t.Delete(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func (t *testHttpClient) send(method string, url string, bodyType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", bodyType)
	return t.Do(req)
}
