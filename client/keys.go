/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

var (
	DefaultV2KeysPrefix = "/v2/keys"
)

var (
	ErrUnavailable = errors.New("client: no available etcd endpoints")
	ErrNoLeader    = errors.New("client: no leader")
	ErrKeyNoExist  = errors.New("client: key does not exist")
	ErrKeyExists   = errors.New("client: key already exists")
)

func NewKeysAPI(c HTTPClient) KeysAPI {
	return &httpKeysAPI{
		client: c,
		prefix: DefaultV2KeysPrefix,
	}
}

func NewDiscoveryKeysAPI(c HTTPClient) KeysAPI {
	return &httpKeysAPI{
		client: c,
		prefix: "",
	}
}

type KeysAPI interface {
	Create(ctx context.Context, key, value string, ttl time.Duration) (*Response, error)
	Get(ctx context.Context, key string) (*Response, error)

	Watch(key string, idx uint64) Watcher
	RecursiveWatch(key string, idx uint64) Watcher
}

type Watcher interface {
	Next(context.Context) (*Response, error)
}

type Response struct {
	Action   string `json:"action"`
	Node     *Node  `json:"node"`
	PrevNode *Node  `json:"prevNode"`
	Index    uint64
}

type Nodes []*Node
type Node struct {
	Key           string `json:"key"`
	Value         string `json:"value"`
	Nodes         Nodes  `json:"nodes"`
	ModifiedIndex uint64 `json:"modifiedIndex"`
	CreatedIndex  uint64 `json:"createdIndex"`
}

func (n *Node) String() string {
	return fmt.Sprintf("{Key: %s, CreatedIndex: %d, ModifiedIndex: %d}", n.Key, n.CreatedIndex, n.ModifiedIndex)
}

type httpKeysAPI struct {
	client HTTPClient
	prefix string
}

func (k *httpKeysAPI) Create(ctx context.Context, key, val string, ttl time.Duration) (*Response, error) {
	create := &createAction{
		Prefix: k.prefix,
		Key:    key,
		Value:  val,
	}
	if ttl >= 0 {
		uttl := uint64(ttl.Seconds())
		create.TTL = &uttl
	}

	resp, body, err := k.client.Do(ctx, create)
	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(resp.StatusCode, resp.Header, body)
}

func (k *httpKeysAPI) Get(ctx context.Context, key string) (*Response, error) {
	get := &getAction{
		Prefix:    k.prefix,
		Key:       key,
		Recursive: false,
	}

	resp, body, err := k.client.Do(ctx, get)
	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(resp.StatusCode, resp.Header, body)
}

func (k *httpKeysAPI) Watch(key string, idx uint64) Watcher {
	return &httpWatcher{
		client: k.client,
		nextWait: waitAction{
			Prefix:    k.prefix,
			Key:       key,
			WaitIndex: idx,
			Recursive: false,
		},
	}
}

func (k *httpKeysAPI) RecursiveWatch(key string, idx uint64) Watcher {
	return &httpWatcher{
		client: k.client,
		nextWait: waitAction{
			Prefix:    k.prefix,
			Key:       key,
			WaitIndex: idx,
			Recursive: true,
		},
	}
}

type httpWatcher struct {
	client   HTTPClient
	nextWait waitAction
}

func (hw *httpWatcher) Next(ctx context.Context) (*Response, error) {
	httpresp, body, err := hw.client.Do(ctx, &hw.nextWait)
	if err != nil {
		return nil, err
	}

	resp, err := unmarshalHTTPResponse(httpresp.StatusCode, httpresp.Header, body)
	if err != nil {
		return nil, err
	}

	hw.nextWait.WaitIndex = resp.Node.ModifiedIndex + 1
	return resp, nil
}

// v2KeysURL forms a URL representing the location of a key.
// The endpoint argument represents the base URL of an etcd
// server. The prefix is the path needed to route from the
// provided endpoint's path to the root of the keys API
// (typically "/v2/keys").
func v2KeysURL(ep url.URL, prefix, key string) *url.URL {
	ep.Path = path.Join(ep.Path, prefix, key)
	return &ep
}

type getAction struct {
	Prefix    string
	Key       string
	Recursive bool
}

func (g *getAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2KeysURL(ep, g.Prefix, g.Key)

	params := u.Query()
	params.Set("recursive", strconv.FormatBool(g.Recursive))
	u.RawQuery = params.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

type waitAction struct {
	Prefix    string
	Key       string
	WaitIndex uint64
	Recursive bool
}

func (w *waitAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2KeysURL(ep, w.Prefix, w.Key)

	params := u.Query()
	params.Set("wait", "true")
	params.Set("waitIndex", strconv.FormatUint(w.WaitIndex, 10))
	params.Set("recursive", strconv.FormatBool(w.Recursive))
	u.RawQuery = params.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

type createAction struct {
	Prefix string
	Key    string
	Value  string
	TTL    *uint64
}

func (c *createAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2KeysURL(ep, c.Prefix, c.Key)

	params := u.Query()
	params.Set("prevExist", "false")
	u.RawQuery = params.Encode()

	form := url.Values{}
	form.Add("value", c.Value)
	if c.TTL != nil {
		form.Add("ttl", strconv.FormatUint(*c.TTL, 10))
	}
	body := strings.NewReader(form.Encode())

	req, _ := http.NewRequest("PUT", u.String(), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

func unmarshalHTTPResponse(code int, header http.Header, body []byte) (res *Response, err error) {
	switch code {
	case http.StatusOK, http.StatusCreated:
		res, err = unmarshalSuccessfulResponse(header, body)
	default:
		err = unmarshalErrorResponse(code)
	}

	return
}

func unmarshalSuccessfulResponse(header http.Header, body []byte) (*Response, error) {
	var res Response
	err := json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	if header.Get("X-Etcd-Index") != "" {
		res.Index, err = strconv.ParseUint(header.Get("X-Etcd-Index"), 10, 64)
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func unmarshalErrorResponse(code int) error {
	switch code {
	case http.StatusNotFound:
		return ErrKeyNoExist
	case http.StatusPreconditionFailed:
		return ErrKeyExists
	case http.StatusInternalServerError:
		// this isn't necessarily true
		return ErrNoLeader
	case http.StatusGatewayTimeout:
		return ErrTimeout
	default:
	}

	return fmt.Errorf("unrecognized HTTP status code %d", code)
}
