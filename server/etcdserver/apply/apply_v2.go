// Copyright 2016 The etcd Authors
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
	"encoding/json"
	"net/http"
	"path"
	"regexp"

	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

var (
	storeMemberAttributeRegexp = regexp.MustCompile(path.Join(membership.StoreMembersPrefix, "[[:xdigit:]]{1,16}", "attributes"))
)

type RequestV2 pb.Request

func (r *RequestV2) String() string {
	rpb := pb.Request(*r)
	return rpb.String()
}

func v2ToV3Request(lg *zap.Logger, rv2 *RequestV2) pb.InternalRaftRequest {
	r := (*pb.Request)(rv2)
	rMethod := r.GetMethod()
	rPath := r.GetPath()
	rVal := r.GetVal()
	rID := r.GetID()
	if rMethod != http.MethodPut || (!storeMemberAttributeRegexp.MatchString(rPath) && rPath != membership.StoreClusterVersionKey()) {
		lg.Panic("detected disallowed v2 WAL for stage --v2-deprecation=write-only", zap.String("method", rMethod))
	}
	if storeMemberAttributeRegexp.MatchString(rPath) {
		id := membership.MustParseMemberIDFromKey(lg, path.Dir(rPath))
		var attr membership.Attributes
		if err := json.Unmarshal([]byte(rVal), &attr); err != nil {
			lg.Panic("failed to unmarshal", zap.String("value", rVal), zap.Error(err))
		}
		return pb.InternalRaftRequest{
			Header: &pb.RequestHeader{
				ID: rID,
			},
			ClusterMemberAttrSet: &membershippb.ClusterMemberAttrSetRequest{
				Member_ID: uint64(id),
				MemberAttributes: &membershippb.Attributes{
					Name:       attr.Name,
					ClientUrls: attr.ClientURLs,
				},
			},
		}
	}
	if rPath == membership.StoreClusterVersionKey() {
		return pb.InternalRaftRequest{
			Header: &pb.RequestHeader{
				ID: rID,
			},
			ClusterVersionSet: &membershippb.ClusterVersionSetRequest{
				Ver: rVal,
			},
		}
	}
	lg.Panic("detected disallowed v2 WAL for stage --v2-deprecation=write-only", zap.String("method", rMethod))
	return pb.InternalRaftRequest{}
}
