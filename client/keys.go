// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

const (
	ErrorCodeKeyNotFound = 100
	ErrorCodeTestFailed  = 101
	ErrorCodeNotFile     = 102
	ErrorCodeNotDir      = 104
	ErrorCodeNodeExist   = 105
	ErrorCodeRootROnly   = 107
	ErrorCodeDirNotEmpty = 108

	ErrorCodePrevValueRequired = 201
	ErrorCodeTTLNaN            = 202
	ErrorCodeIndexNaN          = 203
	ErrorCodeInvalidField      = 209
	ErrorCodeInvalidForm       = 210

	ErrorCodeRaftInternal = 300
	ErrorCodeLeaderElect  = 301

	ErrorCodeWatcherCleared    = 400
	ErrorCodeEventIndexCleared = 401
)

type Error struct {
	Code    int    `json:"errorCode"`
	Message string `json:"message"`
	Cause   string `json:"cause"`
	Index   uint64 `json:"index"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%v: %v (%v) [%v]", e.Code, e.Message, e.Cause, e.Index)
}

// PrevExistType is used to define an existence condition when setting
// or deleting Nodes.
type PrevExistType string

const (
	PrevIgnore  = PrevExistType("")
	PrevExist   = PrevExistType("true")
	PrevNoExist = PrevExistType("false")
)

var (
	defaultV2KeysPrefix = "/v2/keys"
)

// NewKeysAPI builds a KeysAPI that interacts with etcd's key-value
// API over HTTP.
func NewKeysAPI(c Client) KeysAPI {
	return NewKeysAPIWithPrefix(c, defaultV2KeysPrefix)
}

// NewKeysAPIWithPrefix acts like NewKeysAPI, but allows the caller
// to provide a custom base URL path. This should only be used in
// very rare cases.
func NewKeysAPIWithPrefix(c Client, p string) KeysAPI {
	return &httpKeysAPI{
		client: c,
		prefix: p,
	}
}

type KeysAPI interface {
	// Get retrieves a set of Nodes from etcd
	Get(ctx context.Context, key string, opts *GetOptions) (*Response, error)

	// Set assigns a new value to a Node identified by a given key. The caller
	// may define a set of conditions in the SetOptions.
	Set(ctx context.Context, key, value string, opts *SetOptions) (*Response, error)

	// Delete removes a Node identified by the given key, optionally destroying
	// all of its children as well. The caller may define a set of required
	// conditions in an DeleteOptions object.
	Delete(ctx context.Context, key string, opts *DeleteOptions) (*Response, error)

	// Create is an alias for Set w/ PrevExist=false
	Create(ctx context.Context, key, value string) (*Response, error)

	// Update is an alias for Set w/ PrevExist=true
	Update(ctx context.Context, key, value string) (*Response, error)

	// Watcher builds a new Watcher targeted at a specific Node identified
	// by the given key. The Watcher may be configured at creation time
	// through a WatcherOptions object. The returned Watcher is designed
	// to emit events that happen to a Node, and optionally to its children.
	Watcher(key string, opts *WatcherOptions) Watcher
}

type WatcherOptions struct {
	// AfterIndex defines the index after-which the Watcher should
	// start emitting events. For example, if a value of 5 is
	// provided, the first event will have an index >= 6.
	//
	// Setting AfterIndex to 0 (default) means that the Watcher
	// should start watching for events starting at the current
	// index, whatever that may be.
	AfterIndex uint64

	// Recursive specifices whether or not the Watcher should emit
	// events that occur in children of the given keyspace. If set
	// to false (default), events will be limited to those that
	// occur for the exact key.
	Recursive bool
}

type SetOptions struct {
	// PrevValue specifies what the current value of the Node must
	// be in order for the Set operation to succeed.
	//
	// Leaving this field empty means that the caller wishes to
	// ignore the current value of the Node. This cannot be used
	// to compare the Node's current value to an empty string.
	PrevValue string

	// PrevIndex indicates what the current ModifiedIndex of the
	// Node must be in order for the Set operation to succeed.
	//
	// If PrevIndex is set to 0 (default), no comparison is made.
	PrevIndex uint64

	// PrevExist specifies whether the Node must currently exist
	// (PrevExist) or not (PrevNoExist). If the caller does not
	// care about existence, set PrevExist to PrevIgnore, or simply
	// leave it unset.
	PrevExist PrevExistType

	// TTL defines a period of time after-which the Node should
	// expire and no longer exist. Values <= 0 are ignored. Given
	// that the zero-value is ignored, TTL cannot be used to set
	// a TTL of 0.
	TTL time.Duration
}

type GetOptions struct {
	// Recursive defines whether or not all children of the Node
	// should be returned.
	Recursive bool

	// Sort instructs the server whether or not to sort the Nodes.
	// If true, the Nodes are sorted alphabetically by key in
	// ascending order (A to z). If false (default), the Nodes will
	// not be sorted and the ordering used should not be considered
	// predictable.
	Sort bool
}

type DeleteOptions struct {
	// PrevValue specifies what the current value of the Node must
	// be in order for the Delete operation to succeed.
	//
	// Leaving this field empty means that the caller wishes to
	// ignore the current value of the Node. This cannot be used
	// to compare the Node's current value to an empty string.
	PrevValue string

	// PrevIndex indicates what the current ModifiedIndex of the
	// Node must be in order for the Delete operation to succeed.
	//
	// If PrevIndex is set to 0 (default), no comparison is made.
	PrevIndex uint64

	// Recursive defines whether or not all children of the Node
	// should be deleted. If set to true, all children of the Node
	// identified by the given key will be deleted. If left unset
	// or explicitly set to false, only a single Node will be
	// deleted.
	Recursive bool
}

type Watcher interface {
	// Next blocks until an etcd event occurs, then returns a Response
	// represeting that event. The behavior of Next depends on the
	// WatcherOptions used to construct the Watcher. Next is designed to
	// be called repeatedly, each time blocking until a subsequent event
	// is available.
	//
	// If the provided context is cancelled, Next will return a non-nil
	// error. Any other failures encountered while waiting for the next
	// event (connection issues, deserialization failures, etc) will
	// also result in a non-nil error.
	Next(context.Context) (*Response, error)
}

type Response struct {
	// Action is the name of the operation that occurred. Possible values
	// include get, set, delete, update, create, compareAndSwap,
	// compareAndDelete and expire.
	Action string `json:"action"`

	// Node represents the state of the relevant etcd Node.
	Node *Node `json:"node"`

	// PrevNode represents the previous state of the Node. PrevNode is non-nil
	// only if the Node existed before the action occured and the action
	// caused a change to the Node.
	PrevNode *Node `json:"prevNode"`

	// Index holds the cluster-level index at the time the Response was generated.
	// This index is not tied to the Node(s) contained in this Response.
	Index uint64 `json:"-"`
}

type Node struct {
	// Key represents the unique location of this Node (e.g. "/foo/bar").
	Key string `json:"key"`

	// Value is the current data stored on this Node. If this Node
	// is a directory, Value will be empty.
	Value string `json:"value"`

	// Nodes holds the children of this Node, only if this Node is a directory.
	// This slice of will be arbitrarily deep (children, grandchildren, great-
	// grandchildren, etc.) if a recursive Get or Watch request were made.
	Nodes []*Node `json:"nodes"`

	// CreatedIndex is the etcd index at-which this Node was created.
	CreatedIndex uint64 `json:"createdIndex"`

	// ModifiedIndex is the etcd index at-which this Node was last modified.
	ModifiedIndex uint64 `json:"modifiedIndex"`
}

func (n *Node) String() string {
	return fmt.Sprintf("{Key: %s, CreatedIndex: %d, ModifiedIndex: %d}", n.Key, n.CreatedIndex, n.ModifiedIndex)
}

type httpKeysAPI struct {
	client httpClient
	prefix string
}

func (k *httpKeysAPI) Set(ctx context.Context, key, val string, opts *SetOptions) (*Response, error) {
	act := &setAction{
		Prefix: k.prefix,
		Key:    key,
		Value:  val,
	}

	if opts != nil {
		act.PrevValue = opts.PrevValue
		act.PrevIndex = opts.PrevIndex
		act.PrevExist = opts.PrevExist
		act.TTL = opts.TTL
	}

	resp, body, err := k.client.Do(ctx, act)
	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(resp.StatusCode, resp.Header, body)
}

func (k *httpKeysAPI) Create(ctx context.Context, key, val string) (*Response, error) {
	return k.Set(ctx, key, val, &SetOptions{PrevExist: PrevNoExist})
}

func (k *httpKeysAPI) Update(ctx context.Context, key, val string) (*Response, error) {
	return k.Set(ctx, key, val, &SetOptions{PrevExist: PrevExist})
}

func (k *httpKeysAPI) Delete(ctx context.Context, key string, opts *DeleteOptions) (*Response, error) {
	act := &deleteAction{
		Prefix: k.prefix,
		Key:    key,
	}

	if opts != nil {
		act.PrevValue = opts.PrevValue
		act.PrevIndex = opts.PrevIndex
		act.Recursive = opts.Recursive
	}

	resp, body, err := k.client.Do(ctx, act)
	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(resp.StatusCode, resp.Header, body)
}

func (k *httpKeysAPI) Get(ctx context.Context, key string, opts *GetOptions) (*Response, error) {
	act := &getAction{
		Prefix: k.prefix,
		Key:    key,
	}

	if opts != nil {
		act.Recursive = opts.Recursive
		act.Sorted = opts.Sort
	}

	resp, body, err := k.client.Do(ctx, act)
	if err != nil {
		return nil, err
	}

	return unmarshalHTTPResponse(resp.StatusCode, resp.Header, body)
}

func (k *httpKeysAPI) Watcher(key string, opts *WatcherOptions) Watcher {
	act := waitAction{
		Prefix: k.prefix,
		Key:    key,
	}

	if opts != nil {
		act.Recursive = opts.Recursive
		if opts.AfterIndex > 0 {
			act.WaitIndex = opts.AfterIndex + 1
		}
	}

	return &httpWatcher{
		client:   k.client,
		nextWait: act,
	}
}

type httpWatcher struct {
	client   httpClient
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
	Sorted    bool
}

func (g *getAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2KeysURL(ep, g.Prefix, g.Key)

	params := u.Query()
	params.Set("recursive", strconv.FormatBool(g.Recursive))
	params.Set("sorted", strconv.FormatBool(g.Sorted))
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

type setAction struct {
	Prefix    string
	Key       string
	Value     string
	PrevValue string
	PrevIndex uint64
	PrevExist PrevExistType
	TTL       time.Duration
}

func (a *setAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2KeysURL(ep, a.Prefix, a.Key)

	params := u.Query()
	if a.PrevValue != "" {
		params.Set("prevValue", a.PrevValue)
	}
	if a.PrevIndex != 0 {
		params.Set("prevIndex", strconv.FormatUint(a.PrevIndex, 10))
	}
	if a.PrevExist != PrevIgnore {
		params.Set("prevExist", string(a.PrevExist))
	}
	u.RawQuery = params.Encode()

	form := url.Values{}
	form.Add("value", a.Value)
	if a.TTL > 0 {
		form.Add("ttl", strconv.FormatUint(uint64(a.TTL.Seconds()), 10))
	}
	body := strings.NewReader(form.Encode())

	req, _ := http.NewRequest("PUT", u.String(), body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

type deleteAction struct {
	Prefix    string
	Key       string
	Value     string
	PrevValue string
	PrevIndex uint64
	Recursive bool
}

func (a *deleteAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2KeysURL(ep, a.Prefix, a.Key)

	params := u.Query()
	if a.PrevValue != "" {
		params.Set("prevValue", a.PrevValue)
	}
	if a.PrevIndex != 0 {
		params.Set("prevIndex", strconv.FormatUint(a.PrevIndex, 10))
	}
	if a.Recursive {
		params.Set("recursive", "true")
	}
	u.RawQuery = params.Encode()

	req, _ := http.NewRequest("DELETE", u.String(), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req
}

func unmarshalHTTPResponse(code int, header http.Header, body []byte) (res *Response, err error) {
	switch code {
	case http.StatusOK, http.StatusCreated:
		res, err = unmarshalSuccessfulKeysResponse(header, body)
	default:
		err = unmarshalFailedKeysResponse(body)
	}

	return
}

func unmarshalSuccessfulKeysResponse(header http.Header, body []byte) (*Response, error) {
	var res Response
	err := json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	if header.Get("X-Etcd-Index") != "" {
		res.Index, err = strconv.ParseUint(header.Get("X-Etcd-Index"), 10, 64)
		if err != nil {
			return nil, err
		}
	}
	return &res, nil
}

func unmarshalFailedKeysResponse(body []byte) error {
	var etcdErr Error
	if err := json.Unmarshal(body, &etcdErr); err != nil {
		return err
	}
	return etcdErr
}
