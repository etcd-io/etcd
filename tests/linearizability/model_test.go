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
			name: "Get response data should match PUT",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "1", Revision: 1}},
			},
		},
		{
			name: "Get response revision should be equal or greater then put",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key"}, resp: EtcdResponse{Revision: 2}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 2}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{Revision: 4}},
			},
		},
		{
			name: "Put bumps revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Put can fail and be lost",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 2}},
			},
		},
		{
			name: "Put can fail but bump revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "3"}, resp: EtcdResponse{Revision: 3}},
			},
		},
		{
			name: "Put can fail but be persisted and bump revision",
			operations: []testOperation{
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "1"}, resp: EtcdResponse{Revision: 1}},
				{req: EtcdRequest{Op: Put, Key: "key", PutData: "2"}, resp: EtcdResponse{Err: errors.New("failed")}},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 1}, failure: true},
				{req: EtcdRequest{Op: Get, Key: "key"}, resp: EtcdResponse{GetData: "2", Revision: 2}},
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
	}
	for _, tc := range tcs {
		var ok bool
		t.Run(tc.name, func(t *testing.T) {
			state := etcdModel.Init()
			for _, op := range tc.operations {
				t.Logf("state: %v", state)
				ok, state = etcdModel.Step(state, op.req, op.resp)
				if ok != !op.failure {
					t.Errorf("Unexpected operation result, expect: %v, got: %v, operation: %s", !op.failure, ok, etcdModel.DescribeOperation(op.req, op.resp))
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
