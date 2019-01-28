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

package mockstore

import (
	"net/url"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/pd/client"
	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/mockstore/mocktikv"
	"github.com/pingcap/tidb/store/tikv"
)

// MockDriver is in memory mock TiKV driver.
type MockDriver struct {
}

// Open creates a MockTiKV storage.
func (d MockDriver) Open(path string) (kv.Storage, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !strings.EqualFold(u.Scheme, "mocktikv") {
		return nil, errors.Errorf("Uri scheme expected(mocktikv) but found (%s)", u.Scheme)
	}

	opts := []MockTiKVStoreOption{WithPath(u.Path)}
	txnLocalLatches := config.GetGlobalConfig().TxnLocalLatches
	if txnLocalLatches.Enabled {
		opts = append(opts, WithTxnLocalLatches(txnLocalLatches.Capacity))
	}
	return NewMockTikvStore(opts...)
}

type mockOptions struct {
	cluster         *mocktikv.Cluster
	mvccStore       mocktikv.MVCCStore
	clientHijack    func(tikv.Client) tikv.Client
	pdClientHijack  func(pd.Client) pd.Client
	path            string
	txnLocalLatches uint
}

// MockTiKVStoreOption is used to control some behavior of mock tikv.
type MockTiKVStoreOption func(*mockOptions)

// WithHijackClient hijacks KV client's behavior, makes it easy to simulate the network
// problem between TiDB and TiKV.
func WithHijackClient(wrap func(tikv.Client) tikv.Client) MockTiKVStoreOption {
	return func(c *mockOptions) {
		c.clientHijack = wrap
	}
}

// WithCluster provides the customized cluster.
func WithCluster(cluster *mocktikv.Cluster) MockTiKVStoreOption {
	return func(c *mockOptions) {
		c.cluster = cluster
	}
}

// WithMVCCStore provides the customized mvcc store.
func WithMVCCStore(store mocktikv.MVCCStore) MockTiKVStoreOption {
	return func(c *mockOptions) {
		c.mvccStore = store
	}
}

// WithPath specifies the mocktikv path.
func WithPath(path string) MockTiKVStoreOption {
	return func(c *mockOptions) {
		c.path = path
	}
}

// WithTxnLocalLatches enable txnLocalLatches, when capacity > 0.
func WithTxnLocalLatches(capacity uint) MockTiKVStoreOption {
	return func(c *mockOptions) {
		c.txnLocalLatches = capacity
	}
}

// NewMockTikvStore creates a mocked tikv store, the path is the file path to store the data.
// If path is an empty string, a memory storage will be created.
func NewMockTikvStore(options ...MockTiKVStoreOption) (kv.Storage, error) {
	var opt mockOptions
	for _, f := range options {
		f(&opt)
	}

	client, pdClient, err := mocktikv.NewTiKVAndPDClient(opt.cluster, opt.mvccStore, opt.path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return tikv.NewTestTiKVStore(client, pdClient, opt.clientHijack, opt.pdClientHijack, opt.txnLocalLatches)
}
