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
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "2", revision: 42}},
			},
		},
		{
			name: "First Put can start from non-zero revision",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{revision: 42}},
			},
		},
		{
			name: "Get response data should match PUT",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{revision: 1}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "2", revision: 1}, failure: true},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "1", revision: 1}},
			},
		},
		{
			name: "Get response revision should be equal or greater then put",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key"}, resp: etcdResponse{revision: 2}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{revision: 1}, failure: true},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{revision: 2}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{revision: 4}},
			},
		},
		{
			name: "Put bumps revision",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{revision: 1}},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{revision: 1}, failure: true},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{revision: 2}},
			},
		},
		{
			name: "Put can fail and be lost",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{revision: 1}},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{err: errors.New("failed")}},
				{req: etcdRequest{op: Put, key: "key", putData: "3"}, resp: etcdResponse{revision: 2}},
			},
		},
		{
			name: "Put can fail but bump revision",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{revision: 1}},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{err: errors.New("failed")}},
				{req: etcdRequest{op: Put, key: "key", putData: "3"}, resp: etcdResponse{revision: 3}},
			},
		},
		{
			name: "Put can fail but be persisted and bump revision",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{revision: 1}},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{err: errors.New("failed")}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "2", revision: 1}, failure: true},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "2", revision: 2}},
			},
		},
		{
			name: "Put can fail but be persisted later",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{err: errors.New("failed")}},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{revision: 2}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "2", revision: 2}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "1", revision: 3}},
			},
		},
		{
			name: "Put can fail but bump revision later",
			operations: []testOperation{
				{req: etcdRequest{op: Put, key: "key", putData: "1"}, resp: etcdResponse{err: errors.New("failed")}},
				{req: etcdRequest{op: Put, key: "key", putData: "2"}, resp: etcdResponse{revision: 2}},
				{req: etcdRequest{op: Get, key: "key"}, resp: etcdResponse{getData: "2", revision: 2}},
				{req: etcdRequest{op: Put, key: "key", putData: "3"}, resp: etcdResponse{revision: 4}},
			},
		},
		{
			name: "Deleting non existent key does not change revision",
			operations: []testOperation{
				{req: etcdRequest{op: Delete, key: "NotThere"}, resp: etcdResponse{deleted: 0, revision: 4}},
			},
		},
		{
			name: "Deleting  existent key bumps up revision",
			operations: []testOperation{
				{req: etcdRequest{op: Delete, key: "key"}, resp: etcdResponse{deleted: 1, revision: 5}},
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
	req     etcdRequest
	resp    etcdResponse
	failure bool
}
