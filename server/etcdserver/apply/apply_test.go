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

package apply

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/raft/v3/raftpb"
)

type recordingUberApplier struct {
	request       *InternalRaftRequestWrapper
	shouldApplyV3 membership.ShouldApplyV3
}

func (a *recordingUberApplier) Apply(r *InternalRaftRequestWrapper, shouldApplyV3 membership.ShouldApplyV3) *Result {
	a.request = r
	a.shouldApplyV3 = shouldApplyV3
	return &Result{}
}

func legacyRequestBytes(id uint64, method, requestPath, value string) []byte {
	var data []byte
	data = protowire.AppendTag(data, 1, protowire.VarintType)
	data = protowire.AppendVarint(data, id)
	data = protowire.AppendTag(data, 2, protowire.BytesType)
	data = protowire.AppendString(data, method)
	data = protowire.AppendTag(data, 3, protowire.BytesType)
	data = protowire.AppendString(data, requestPath)
	data = protowire.AppendTag(data, 4, protowire.BytesType)
	data = protowire.AppendString(data, value)
	return data
}

func TestApplyLegacyV2MetadataRequest(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		value       string
		wantAttr    *membershippb.ClusterMemberAttrSetRequest
		wantVersion string
	}{
		{
			name:        "member attributes",
			requestPath: "/0/members/1a2b/attributes",
			value:       `{"name":"member-1","clientURLs":["https://127.0.0.1:2379"]}`,
			wantAttr: &membershippb.ClusterMemberAttrSetRequest{
				Member_ID: 0x1a2b,
				MemberAttributes: &membershippb.Attributes{
					Name:       "member-1",
					ClientUrls: []string{"https://127.0.0.1:2379"},
				},
			},
		},
		{
			name:        "cluster version",
			requestPath: "/0/version",
			value:       "3.6.15",
			wantVersion: "3.6.15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := &recordingUberApplier{}
			entry := &raftpb.Entry{Data: legacyRequestBytes(42, "PUT", tt.requestPath, tt.value)}
			_, id := Apply(zaptest.NewLogger(t), entry, applier, wait.New(), membership.ApplyV2storeOnly)

			require.Equal(t, uint64(42), id)
			require.NotNil(t, applier.request)
			require.Equal(t, membership.ApplyV2storeOnly, applier.shouldApplyV3)
			require.True(t, proto.Equal(tt.wantAttr, applier.request.ClusterMemberAttrSet))
			if tt.wantVersion == "" {
				require.Nil(t, applier.request.ClusterVersionSet)
			} else {
				require.Equal(t, tt.wantVersion, applier.request.ClusterVersionSet.GetVer())
			}
		})
	}
}

func TestRequestV2RejectsMalformedInput(t *testing.T) {
	var request requestV2
	require.Error(t, request.Unmarshal([]byte{0x12, 0x02, 'P'}))
}

var _ UberApplier = (*recordingUberApplier)(nil)
