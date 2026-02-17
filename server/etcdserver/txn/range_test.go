// Copyright 2025 The etcd Authors
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

package txn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func TestRangeLimit(t *testing.T) {
	tests := []struct {
		name     string
		request  *pb.RangeRequest
		expected int64
	}{
		{
			name: "No sort, no limit",
			request: &pb.RangeRequest{
				Limit: 0,
			},
			expected: 0,
		},
		{
			name: "No sort, with limit",
			request: &pb.RangeRequest{
				Limit: 10,
			},
			expected: 11,
		},
		{
			name: "Sort KEY ASCEND, with limit (Now optimized to return limit)",
			request: &pb.RangeRequest{
				Limit:      10,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
			expected: 11,
		},
		{
			name: "Sort KEY DESCEND, with limit",
			request: &pb.RangeRequest{
				Limit:      10,
				SortOrder:  pb.RangeRequest_DESCEND,
				SortTarget: pb.RangeRequest_KEY,
			},
			expected: 0,
		},
		{
			name: "Sort VERSION ASCEND, with limit",
			request: &pb.RangeRequest{
				Limit:      10,
				SortOrder:  pb.RangeRequest_ASCEND,
				SortTarget: pb.RangeRequest_VERSION,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := rangeLimit(tt.request)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
