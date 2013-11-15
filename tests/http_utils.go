package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// Creates a new HTTP client with KeepAlive disabled.
func NewHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
}

// Reads the body from the response and closes it.
func ReadBody(resp *http.Response) []byte {
	if resp == nil {
		return []byte{}
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return body
}

// Reads the body from the response and parses it as JSON.
func ReadBodyJSON(resp *http.Response) map[string]interface{} {
	m := make(map[string]interface{})
	b := ReadBody(resp)
	if err := json.Unmarshal(b, &m); err != nil {
		panic(fmt.Sprintf("HTTP body JSON parse error: %v", err))
	}
	return m
}

func Get(url string) (*http.Response, error) {
	return send("GET", url, "application/json", nil)
}

func Post(url string, bodyType string, body io.Reader) (*http.Response, error) {
	return send("POST", url, bodyType, body)
}

func PostForm(url string, data url.Values) (*http.Response, error) {
	return Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func Put(url string, bodyType string, body io.Reader) (*http.Response, error) {
	return send("PUT", url, bodyType, body)
}

func PutForm(url string, data url.Values) (*http.Response, error) {
	return Put(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func Delete(url string, bodyType string, body io.Reader) (*http.Response, error) {
	return send("DELETE", url, bodyType, body)
}

func DeleteForm(url string, data url.Values) (*http.Response, error) {
	return Delete(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func send(method string, url string, bodyType string, body io.Reader) (*http.Response, error) {
	c := NewHTTPClient()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", bodyType)
	return c.Do(req)
}
