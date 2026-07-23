// Copyright 2026 The etcd Authors
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

package command

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func TestParseCompareLease(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		expected    *pb.Compare
		expectedErr string
	}{
		{
			name: "zero lease ID parsing",
			line: `lease("foo") > "0"`,
			expected: &pb.Compare{
				Key:         []byte("foo"),
				Target:      pb.Compare_LEASE,
				Result:      pb.Compare_GREATER,
				TargetUnion: &pb.Compare_Lease{Lease: 0},
			},
		},
		{
			name: "valid hex lease id",
			line: `lease("foo") = "2d8257079fa1bc0c"`,
			expected: &pb.Compare{
				Key:         []byte("foo"),
				Target:      pb.Compare_LEASE,
				Result:      pb.Compare_EQUAL,
				TargetUnion: &pb.Compare_Lease{Lease: 0x2d8257079fa1bc0c},
			},
		},
		{
			name:        "overflows int64",
			line:        `lease("foo") > "ffffffffffffffff"`,
			expectedErr: "value out of range",
		},
		{
			name:        "non-hex character",
			line:        `lease("foo") > "g"`,
			expectedErr: "invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, err := ParseCompare(tt.line)

			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			require.True(t, proto.Equal(tt.expected, cmp.GetCompare()))
		})
	}
}
