// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"errors"
	"testing"
)

func TestModel(t *testing.T) {
	tcs := []struct {
		name       string
		operations []testOperation
	}{
		{
			name: "First Get can start from non-empty value and non-zero revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 42}},
			},
		},
		{
			name: "First Put can start from non-zero revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Revision: 42}},
			},
		},
		{
			name: "First delete can start from non-zero revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Revision: 42}},
			},
		},
		{
			name: "First Txn can start from non-zero revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "", TxnNewData: "42"}, resp: EtcdResponse{Revision: 42}},
			},
		},
		{
			name: "Get response data should match put",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
			},
		},
		{
			name: "Get revision should be equal to put",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key"}, resp: EtcdResponse{Revision: 2}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 3}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Put must increase revision by 1",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 3}, failure: true},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Put can fail and be lost before get",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 2}, failure: true},
			},
		},
		{
			name: "Put can fail and be lost before put",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Put can fail and be lost before delete",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Revision: 1}},
			},
		},
		{
			name: "Put can fail and be lost before txn failed",
			operations: []testOperation{
				// Txn failure
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{Revision: 1}},
				// Txn success
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Revision: 2}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{TxnSucceeded: true, Revision: 3}},
			},
		},
		{
			name:       "Put can fail and be lost before txn success",
			operations: []testOperation{},
		},
		{
			name: "Put can fail but be persisted and increase revision before get",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "3", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 2}},
				// Two failed request, two persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "4", Revision: 4}},
			},
		},
		{
			name: "Put can fail but be persisted and increase revision before delete",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 3}},
				// Two failed request, two persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "5"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "6"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 7}},
				// Two failed request, one persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "8"}, resp: EtcdResponse{Revision: 8}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "9"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "10"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 10}},
			},
		},
		{
			name: "Put can fail but be persisted before txn",
			operations: []testOperation{
				// Txn success
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2"}, resp: EtcdResponse{TxnSucceeded: true, Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2"}, resp: EtcdResponse{TxnSucceeded: true, Revision: 3}},
				// Txn failure
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "5"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "5"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 5, GetData: "5"}},
			},
		},
		{
			name: "Delete only increases revision on success",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 2}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 0, Revision: 3}, failure: true},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 0, Revision: 2}},
			},
		},
		{
			name: "Delete clears value",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 2}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Delete can fail and be lost before get",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2}, failure: true},
			},
		},
		{
			name: "Delete can fail and be lost before delete",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 2}},
			},
		},
		{
			name: "Delete can fail and be lost before put",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Delete can fail but be persisted before get",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2}},
				// Two failed request, one persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 3}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 4}},
			},
		},
		{
			name: "Delete can fail but be persisted before put",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 3}},
				// Two failed request, one persisted.
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "5"}, resp: EtcdResponse{Revision: 5}},
			},
		},
		{
			name: "Delete can fail but be persisted before delete",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Revision: 2}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 3}},
				// Two failed request, one persisted.
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Revision: 4}},
			},
		},
		{
			name: "Delete can fail but be persisted before txn",
			operations: []testOperation{
				// Txn success
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "", TxnNewData: "1"}, resp: EtcdResponse{TxnSucceeded: true, Revision: 3}},
				// Txn failure
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "4", TxnNewData: "5"}, resp: EtcdResponse{TxnSucceeded: false, Revision: 5}},
			},
		},
		{
			name: "Txn sets new value if value matches expected",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Revision: 1, TxnSucceeded: true}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Revision: 2, TxnSucceeded: false}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Revision: 1, TxnSucceeded: false}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Revision: 2, TxnSucceeded: true}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 2}},
			},
		},
		{
			name: "Txn can expect on empty key",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "", TxnNewData: "2"}, resp: EtcdResponse{Revision: 2, TxnSucceeded: true}},
			},
		},
		{
			name: "Txn doesn't do anything if value doesn't match expected",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{Revision: 2, TxnSucceeded: true}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{Revision: 1, TxnSucceeded: true}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{Revision: 2, TxnSucceeded: false}, failure: true},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{Revision: 1, TxnSucceeded: false}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "3", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "3", Revision: 2}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
			},
		},
		{
			name: "Txn can fail and be lost before get",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2, GetData: "2"}, failure: true},
			},
		},
		{
			name: "Txn can fail and be lost before delete",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 2}},
			},
		},
		{
			name: "Txn can fail and be lost before put",
			operations: []testOperation{
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Txn can fail but be persisted before get",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1, GetData: "2"}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2, GetData: "2"}},
				// Two failed request, two persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 3}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "3", TxnNewData: "4"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "4", TxnNewData: "5"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 5, GetData: "5"}},
			},
		},
		{
			name: "Txn can fail but be persisted before put",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 3}},
				// Two failed request, two persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "4", TxnNewData: "5"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "5", TxnNewData: "6"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "7"}, resp: EtcdResponse{Revision: 7}},
			},
		},
		{
			name: "Txn can fail but be persisted before delete",
			operations: []testOperation{
				// One failed request, one persisted.
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 3}},
				// Two failed request, two persisted.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "4", TxnNewData: "5"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "5", TxnNewData: "6"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Delete, Key: "key"}, resp: EtcdResponse{Deleted: 1, Revision: 7}},
			},
		},
		{
			name: "Txn can fail but be persisted before txn",
			operations: []testOperation{
				// One failed request, one persisted with success.
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "1", TxnNewData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "2", TxnNewData: "3"}, resp: EtcdResponse{Revision: 3, TxnSucceeded: true}},
				// Two failed request, two persisted with success.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "4"}, resp: EtcdResponse{Revision: 4}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "4", TxnNewData: "5"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "5", TxnNewData: "6"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "6", TxnNewData: "7"}, resp: EtcdResponse{Revision: 7, TxnSucceeded: true}},
				// One failed request, one persisted with failure.
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "8"}, resp: EtcdResponse{Revision: 8}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "8", TxnNewData: "9"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Txn, Key: "key", TxnExpectData: "8", TxnNewData: "10"}, resp: EtcdResponse{Revision: 9}},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			state := etcdModel.Init()
			for _, op := range tc.operations {
				ok, newState := etcdModel.Step(state, op.req, op.resp)
				if ok != !op.failure {
					t.Logf("state: %v", state)
					t.Errorf("Unexpected operation result, expect: %v, got: %v, operation: %s", !op.failure, ok, etcdModel.DescribeOperation(op.req, op.resp))
				}
				if ok {
					state = newState
					t.Logf("state: %v", state)
				}
			}
		})
	}
}

type testOperation struct {
	req     EtcdRequest
	resp    EtcdResponse
	failure bool
}
