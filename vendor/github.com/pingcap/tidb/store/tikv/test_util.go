// Copyright 2017 PingCAP, Inc.
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

package tikv

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/pd/client"
	"github.com/pingcap/tidb/kv"
	"github.com/twinj/uuid"
)

// NewTestTiKVStore creates a test store with Option
func NewTestTiKVStore(client Client, pdClient pd.Client, clientHijack func(Client) Client, pdClientHijack func(pd.Client) pd.Client, txnLocalLatches uint) (kv.Storage, error) {
	if clientHijack != nil {
		client = clientHijack(client)
	}

	pdCli := pd.Client(&codecPDClient{pdClient})
	if pdClientHijack != nil {
		pdCli = pdClientHijack(pdCli)
	}

	// Make sure the uuid is unique.
	uid := uuid.NewV4().String()
	spkv := NewMockSafePointKV()
	tikvStore, err := newTikvStore(uid, pdCli, spkv, client, false)

	if txnLocalLatches > 0 {
		tikvStore.EnableTxnLocalLatches(txnLocalLatches)
	}

	tikvStore.mock = true
	return tikvStore, errors.Trace(err)
}
