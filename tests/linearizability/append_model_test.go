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
	"testing"
)

func TestAppendModel(t *testing.T) {
	tcs := []struct {
		name       string
		operations []testAppendOperation
	}{
		{
			name: "Append appends",
			operations: []testAppendOperation{
				{req: AppendRequest{Key: "key", Op: Append, AppendData: "1"}, resp: AppendResponse{}},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "1"}},
				{req: AppendRequest{Key: "key", Op: Append, AppendData: "2"}, resp: AppendResponse{}},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "1,3"}, failure: true},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "1,2"}},
			},
		},
		{
			name: "Get validates prefix matches",
			operations: []testAppendOperation{
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: ""}},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "1"}},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "2"}, failure: true},
				{req: AppendRequest{Key: "key", Op: Append, AppendData: "2"}, resp: AppendResponse{}},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "1,3"}, failure: true},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "1,2,3"}},
				{req: AppendRequest{Key: "key", Op: Get}, resp: AppendResponse{GetData: "2,3"}, failure: true},
			},
		},
	}
	for _, tc := range tcs {
		var ok bool
		t.Run(tc.name, func(t *testing.T) {
			state := appendModel.Init()
			for _, op := range tc.operations {
				t.Logf("state: %v", state)
				ok, state = appendModel.Step(state, op.req, op.resp)
				if ok != !op.failure {
					t.Errorf("Unexpected operation result, expect: %v, got: %v, operation: %s", !op.failure, ok, appendModel.DescribeOperation(op.req, op.resp))
				}
			}
		})
	}
}

type testAppendOperation struct {
	req     AppendRequest
	resp    AppendResponse
	failure bool
}
