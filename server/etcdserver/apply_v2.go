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

package etcdserver

import (
	"encoding/json"
	"path"
	"time"

	"github.com/coreos/go-semver/semver"

	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/etcdserver/errors"
)

// ApplierV2 is the interface for processing V2 raft messages
type ApplierV2 interface {
	Put(r *RequestV2, shouldApplyV3 membership.ShouldApplyV3) Response
}

func NewApplierV2(lg *zap.Logger, s v2store.Store, c *membership.RaftCluster) ApplierV2 {
	if lg == nil {
		lg = zap.NewNop()
	}
	return &applierV2store{lg: lg, store: s, cluster: c}
}

type applierV2store struct {
	lg      *zap.Logger
	store   v2store.Store
	cluster *membership.RaftCluster
}

func (a *applierV2store) Put(r *RequestV2, shouldApplyV3 membership.ShouldApplyV3) Response {
	if storeMemberAttributeRegexp.MatchString(r.Path) {
		id := membership.MustParseMemberIDFromKey(a.lg, path.Dir(r.Path))
		var attr membership.Attributes
		if err := json.Unmarshal([]byte(r.Val), &attr); err != nil {
			a.lg.Panic("failed to unmarshal", zap.String("value", r.Val), zap.Error(err))
		}
		if a.cluster != nil {
			a.cluster.UpdateAttributes(id, attr, shouldApplyV3)
		}
	}
	// TODO remove v2 version set to avoid the conflict between v2 and v3 in etcd 3.6
	if r.Path == membership.StoreClusterVersionKey() {
		if a.cluster != nil {
			// persist to backend given v2store can be very stale
			a.cluster.SetVersion(semver.Must(semver.NewVersion(r.Val)), api.UpdateCapability, shouldApplyV3)
		}
	}
	// return an empty response since there is no consumer.
	return Response{}
}

// applyV2Request interprets r as a call to v2store.X
// and returns a Response interpreted from v2store.Event
func (s *EtcdServer) applyV2Request(r *RequestV2, shouldApplyV3 membership.ShouldApplyV3) (resp Response) {
	switch r.Method {
	case "PUT":
		return s.applyV2.Put(r, shouldApplyV3)
	default:
		// This should never be reached, but just in case:
		return Response{Err: errors.ErrUnknownMethod}
	}
}

func (r *RequestV2) TTLOptions() v2store.TTLOptionSet {
	refresh, _ := pbutil.GetBool(r.Refresh)
	ttlOptions := v2store.TTLOptionSet{Refresh: refresh}
	if r.Expiration != 0 {
		ttlOptions.ExpireTime = time.Unix(0, r.Expiration)
	}
	return ttlOptions
}
