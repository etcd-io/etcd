// Package goplay provides The Go Playground (https://play.golang.org/) client
package goplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://play.golang.org"

// DefaultClient is default Go Playground client.
var DefaultClient = &Client{}

var delay = time.Sleep

// Client represensts The Go Playground client.
type Client struct {
	// The base URL of The Go Playground. Default is `https://play.golang.org/`.
	BaseURL string

	// The HTTP client to use when sending requests. Defaults to
	// `http.DefaultClient`.
	HTTPClient *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}

func (c *Client) shareEndpoint() string {
	return c.baseURL() + "/share"
}

func (c *Client) compileEndpoint() string {
	return c.baseURL() + "/compile"
}

// Run runs code which compiled in The Go Playground.
func (c *Client) Run(code io.Reader, stdout io.Writer, stderr io.Writer) error {
	resp, err := c.Compile(code)
	if err != nil {
		return err
	}
	if resp.Errors != "" {
		return errors.New(resp.Errors)
	}
	for _, event := range resp.Events {
		delay(event.Delay)
		w := stderr
		if event.Kind == "stdout" {
			w = stdout
		}
		fmt.Fprint(w, event.Message)
	}
	return nil
}

// Compile compiles code on The Go Playground.
func (c *Client) Compile(code io.Reader) (*Response, error) {
	b, err := ioutil.ReadAll(code)
	if err != nil {
		return nil, err
	}
	v := url.Values{}
	v.Set("version", "2")
	v.Set("body", string(b))
	resp, err := c.httpClient().PostForm(c.compileEndpoint(), v)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Share creates go playground share link.
func (c *Client) Share(code io.Reader) (string, error) {
	req, err := http.NewRequest("POST", c.shareEndpoint(), code)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/p/%s", c.baseURL(), string(b)), nil
}

// Response represensts response type of /compile.
type Response struct {
	Errors string
	Events []*Event
	// Licence: Copyright (c) 2014 The Go Authors. All rights reserved.
	// https://github.com/golang/playground/blob/816964eae74f7612221c13ab73f2a8021c581010/sandbox/sandbox.go#L35-L38
}

// Event represensts event of /compile result.
type Event struct {
	Message string
	Kind    string        // "stdout" or "stderr"
	Delay   time.Duration // time to wait before printing Message
	// Licence: Copyright (c) 2014 The Go Authors. All rights reserved.
	// https://github.com/golang/playground/blob/816964eae74f7612221c13ab73f2a8021c581010/sandbox/play.go#L76-L80
}
