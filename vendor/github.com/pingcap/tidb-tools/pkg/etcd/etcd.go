// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package etcd

import (
	"context"
	"crypto/tls"
	"path"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/pingcap/errors"
)

// Node organizes the ectd query result as a Trie tree
type Node struct {
	Value  []byte
	Childs map[string]*Node
}

// Client is a wrapped etcd client that support some simple method
type Client struct {
	client   *clientv3.Client
	rootPath string
}

// NewClient returns a wrapped etcd client
func NewClient(cli *clientv3.Client, root string) *Client {
	return &Client{
		client:   cli,
		rootPath: root,
	}
}

// NewClientFromCfg returns a wrapped etcd client
func NewClientFromCfg(endpoints []string, dialTimeout time.Duration, root string, security *tls.Config) (*Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
		TLS:         security,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &Client{
		client:   cli,
		rootPath: root,
	}, nil
}

// Close shutdowns the connection to etcd
func (e *Client) Close() error {
	if err := e.client.Close(); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// Create guarantees to set a key = value with some options(like ttl)
func (e *Client) Create(ctx context.Context, key string, val string, opts []clientv3.OpOption) error {
	key = keyWithPrefix(e.rootPath, key)
	txnResp, err := e.client.KV.Txn(ctx).If(
		clientv3.Compare(clientv3.ModRevision(key), "=", 0),
	).Then(
		clientv3.OpPut(key, val, opts...),
	).Commit()
	if err != nil {
		return errors.Trace(err)
	}

	if !txnResp.Succeeded {
		return errors.AlreadyExistsf("key %s in etcd", key)
	}

	return nil
}

// Get returns a key/value matchs the given key
func (e *Client) Get(ctx context.Context, key string) ([]byte, error) {
	key = keyWithPrefix(e.rootPath, key)
	resp, err := e.client.KV.Get(ctx, key)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.NotFoundf("key %s in etcd", key)
	}

	return resp.Kvs[0].Value, nil
}

// Update updates a key/value.
// set ttl 0 to disable the Lease ttl feature
func (e *Client) Update(ctx context.Context, key string, val string, ttl int64) error {
	key = keyWithPrefix(e.rootPath, key)

	var opts []clientv3.OpOption
	if ttl > 0 {
		lcr, err := e.client.Lease.Grant(ctx, ttl)
		if err != nil {
			return errors.Trace(err)
		}

		opts = []clientv3.OpOption{clientv3.WithLease(lcr.ID)}
	}

	txnResp, err := e.client.KV.Txn(ctx).If(
		clientv3.Compare(clientv3.ModRevision(key), ">", 0),
	).Then(
		clientv3.OpPut(key, val, opts...),
	).Commit()
	if err != nil {
		return errors.Trace(err)
	}

	if !txnResp.Succeeded {
		return errors.NotFoundf("key %s in etcd", key)
	}

	return nil
}

// UpdateOrCreate updates a key/value, if the key does not exist then create, or update
func (e *Client) UpdateOrCreate(ctx context.Context, key string, val string, ttl int64) error {
	key = keyWithPrefix(e.rootPath, key)

	var opts []clientv3.OpOption
	if ttl > 0 {
		lcr, err := e.client.Lease.Grant(ctx, ttl)
		if err != nil {
			return errors.Trace(err)
		}

		opts = []clientv3.OpOption{clientv3.WithLease(lcr.ID)}
	}

	_, err := e.client.KV.Do(ctx, clientv3.OpPut(key, val, opts...))
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// List returns the trie struct that constructed by the key/value with same prefix
func (e *Client) List(ctx context.Context, key string) (*Node, error) {
	key = keyWithPrefix(e.rootPath, key)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	resp, err := e.client.KV.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Trace(err)
	}

	root := new(Node)
	length := len(key)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if len(key) <= length {
			continue
		}

		keyTail := key[length:]
		tailNode := parseToDirTree(root, keyTail)
		tailNode.Value = kv.Value
	}

	return root, nil
}

// Delete deletes the key/values with matching prefix or key
func (e *Client) Delete(ctx context.Context, key string, withPrefix bool) error {
	key = keyWithPrefix(e.rootPath, key)
	var opts []clientv3.OpOption
	if withPrefix {
		opts = []clientv3.OpOption{clientv3.WithPrefix()}
	}

	_, err := e.client.KV.Delete(ctx, key, opts...)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Watch watchs the events of key with prefix.
func (e *Client) Watch(ctx context.Context, prefix string) clientv3.WatchChan {
	return e.client.Watch(ctx, prefix, clientv3.WithPrefix())
}

func parseToDirTree(root *Node, path string) *Node {
	pathDirs := strings.Split(path, "/")
	current := root
	var next *Node
	var ok bool

	for _, dir := range pathDirs {
		if current.Childs == nil {
			current.Childs = make(map[string]*Node)
		}

		next, ok = current.Childs[dir]
		if !ok {
			current.Childs[dir] = new(Node)
			next = current.Childs[dir]
		}

		current = next
	}

	return current
}

func keyWithPrefix(prefix, key string) string {
	if strings.HasPrefix(key, prefix) {
		return key
	}

	return path.Join(prefix, key)
}
