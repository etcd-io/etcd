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
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protowire"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

var memberAttributePathRegexp = regexp.MustCompile(path.Join(membership.StoreMembersPrefix, "[[:xdigit:]]{1,16}", "attributes"))

// requestV2 contains the part of the removed v2 WAL request needed to replay
// the metadata entries written by etcd 3.5. Keeping this type local to the
// apply package avoids bringing the removed public v2 request API back.
type requestV2 struct {
	ID     uint64
	Method string
	Path   string
	Val    string
}

func (r *requestV2) Unmarshal(data []byte) error {
	*r = requestV2{}
	for len(data) > 0 {
		number, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return protowire.ParseError(n)
		}
		data = data[n:]

		switch number {
		case 1:
			if typ != protowire.VarintType {
				return fmt.Errorf("request v2: field ID has wire type %d", typ)
			}
			value, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			r.ID = value
			data = data[n:]
		case 2, 3, 4:
			if typ != protowire.BytesType {
				return fmt.Errorf("request v2: field %d has wire type %d", number, typ)
			}
			value, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			switch number {
			case 2:
				r.Method = string(value)
			case 3:
				r.Path = string(value)
			case 4:
				r.Val = string(value)
			}
			data = data[n:]
		default:
			n := protowire.ConsumeFieldValue(number, typ, data)
			if n < 0 {
				return protowire.ParseError(n)
			}
			data = data[n:]
		}
	}
	return nil
}

func (r *requestV2) String() string {
	return fmt.Sprintf("RequestV2{ID:%d Method:%q Path:%q}", r.ID, r.Method, r.Path)
}

func (r *requestV2) toInternalRaftRequest(lg *zap.Logger) pb.InternalRaftRequest {
	if r.Method != http.MethodPut || (!memberAttributePathRegexp.MatchString(r.Path) && r.Path != membership.StoreClusterVersionKey()) {
		lg.Panic("detected disallowed v2 WAL for stage --v2-deprecation=write-only", zap.String("method", r.Method), zap.String("path", r.Path))
	}

	if memberAttributePathRegexp.MatchString(r.Path) {
		id := membership.MustParseMemberIDFromKey(lg, path.Dir(r.Path))
		var attributes membership.Attributes
		if err := json.Unmarshal([]byte(r.Val), &attributes); err != nil {
			lg.Panic("failed to unmarshal v2 member attributes", zap.Error(err))
		}
		return pb.InternalRaftRequest{
			Header: &pb.RequestHeader{ID: r.ID},
			ClusterMemberAttrSet: &membershippb.ClusterMemberAttrSetRequest{
				Member_ID: uint64(id),
				MemberAttributes: &membershippb.Attributes{
					Name:       attributes.Name,
					ClientUrls: attributes.ClientURLs,
				},
			},
		}
	}

	return pb.InternalRaftRequest{
		Header:            &pb.RequestHeader{ID: r.ID},
		ClusterVersionSet: &membershippb.ClusterVersionSetRequest{Ver: r.Val},
	}
}
