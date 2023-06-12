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
	"fmt"
	"path"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-semver/semver"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/errors"
	"go.etcd.io/etcd/server/v3/etcdserver/txn"

	"go.uber.org/zap"
)

const v2Version = "v2"

type RequestV2 pb.Request

func (r *RequestV2) String() string {
	rpb := pb.Request(*r)
	return rpb.String()
}

type ApplierV2ToV3 interface {
	Put(r *RequestV2) Response
}

func NewApplierV2ToV3(lg *zap.Logger, c *membership.RaftCluster) ApplierV2ToV3 {
	if lg == nil {
		lg = zap.NewNop()
	}
	return &applierV2ToV3{lg: lg, cluster: c}
}

type applierV2ToV3 struct {
	lg      *zap.Logger
	cluster *membership.RaftCluster
}

func (s *EtcdServer) applyV2RequestToV3(r *RequestV2) (resp Response) {
	stringer := panicAlternativeStringer{
		stringer:    r,
		alternative: func() string { return fmt.Sprintf("id:%d,method:%s,path:%s", r.ID, r.Method, r.Path) },
	}
	defer func(start time.Time) {
		if !utf8.ValidString(r.Method) {
			s.lg.Info("method is not valid utf-8")
			return
		}
		success := resp.Err == nil
		txn.ApplySecObserve(v2Version, r.Method, success, time.Since(start))
		txn.WarnOfExpensiveRequest(s.Logger(), s.Cfg.WarningApplyDuration, start, stringer, nil, nil)
	}(time.Now())

	switch r.Method {
	case "PUT":
		return s.applyV2ToV3.Put(r)
	default:
		// This should never be reached, but just in case:
		return Response{Err: errors.ErrUnknownMethod}
	}
}

func (a *applierV2ToV3) Put(r *RequestV2) Response {
	if storeMemberAttributeRegexp.MatchString(r.Path) {
		id := membership.MustParseMemberIDFromKey(a.lg, path.Dir(r.Path))
		var attr membership.Attributes
		if err := json.Unmarshal([]byte(r.Val), &attr); err != nil {
			a.lg.Panic("failed to unmarshal", zap.String("value", r.Val), zap.Error(err))
		}
		if a.cluster != nil {
			a.cluster.UpdateAttributes(id, attr, true)
		}
		// return an empty response since there is no consumer.
		return Response{}
	}
	// TODO remove v2 version set to avoid the conflict between v2 and v3 in etcd 3.6
	if r.Path == membership.StoreClusterVersionKey() {
		if a.cluster != nil {
			// persist to backend given v2store can be very stale
			a.cluster.SetVersion(semver.Must(semver.NewVersion(r.Val)), api.UpdateCapability, true)
		}
		return Response{}
	}
	a.lg.Panic("unexpected v2 Put request")
	return Response{}
}
