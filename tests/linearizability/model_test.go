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
	"github.com/anishathalye/porcupine"
	"testing"
)

func TestModel(t *testing.T) {
	tcs := []struct {
		name          string
		okOperations  []porcupine.Operation
		failOperation *porcupine.Operation
	}{
		{
			name: "Etcd must return what was written",
			okOperations: []porcupine.Operation{
				{Input: etcdRequest{op: Put, key: "key", putData: "1"}, Output: etcdResponse{}},
				{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: "1"}},
			},
			failOperation: &porcupine.Operation{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: "2"}},
		},
		{
			name: "Etcd can crash after storing result but before returning success to client",
			okOperations: []porcupine.Operation{
				{Input: etcdRequest{op: Put, key: "key", putData: "1"}, Output: etcdResponse{err: errors.New("failed")}},
				{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: "1"}},
			},
		},
		{
			name: "Etcd can crash before storing result",
			okOperations: []porcupine.Operation{
				{Input: etcdRequest{op: Put, key: "key", putData: "1"}, Output: etcdResponse{err: errors.New("failed")}},
				{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: ""}},
			},
		},
		{
			name: "Etcd can continue errored request after it failed",
			okOperations: []porcupine.Operation{
				{Input: etcdRequest{op: Put, key: "key", putData: "1"}, Output: etcdResponse{err: errors.New("failed")}},
				{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: ""}},
				{Input: etcdRequest{op: Put, key: "key"}, Output: etcdResponse{getData: "2"}},
				{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: "1"}},
			},
			failOperation: &porcupine.Operation{Input: etcdRequest{op: Get, key: "key"}, Output: etcdResponse{getData: ""}},
		},
	}
	for _, tc := range tcs {
		var ok bool
		t.Run(tc.name, func(t *testing.T) {
			state := etcdModel.Init()
			for _, op := range tc.okOperations {
				t.Logf("state: %v", state)
				ok, state = etcdModel.Step(state, op.Input, op.Output)
				if !ok {
					t.Errorf("Unexpected failed operation: %s", etcdModel.DescribeOperation(op.Input, op.Output))
				}
			}
			if tc.failOperation != nil {
				t.Logf("state: %v", state)
				ok, state = etcdModel.Step(state, tc.failOperation.Input, tc.failOperation.Output)
				if ok {
					t.Errorf("Unexpected succesfull operation: %s", etcdModel.DescribeOperation(tc.failOperation.Input, tc.failOperation.Output))
				}

			}
		})
	}
}
