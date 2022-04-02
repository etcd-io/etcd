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
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/coreos/go-semver/semver"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/etcdserver/api"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/version"
	"go.etcd.io/etcd/server/v3/lease"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/storage/mvcc"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

const (
	v3Version = "v3"
)

type applyResult struct {
	resp proto.Message
	err  error
	// physc signals the physical effect of the request has completed in addition
	// to being logically reflected by the node. Currently only used for
	// Compaction requests.
	physc <-chan struct{}
	trace *traceutil.Trace
}

// applierV3Internal is the interface for processing internal V3 raft request
type applierV3Internal interface {
	ClusterVersionSet(r *membershippb.ClusterVersionSetRequest, shouldApplyV3 membership.ShouldApplyV3)
	ClusterMemberAttrSet(r *membershippb.ClusterMemberAttrSetRequest, shouldApplyV3 membership.ShouldApplyV3)
	DowngradeInfoSet(r *membershippb.DowngradeInfoSetRequest, shouldApplyV3 membership.ShouldApplyV3)
}

// applierV3 is the interface for processing V3 raft messages
type applierV3 interface {
	Apply(r *pb.InternalRaftRequest, shouldApplyV3 membership.ShouldApplyV3) *applyResult

	Put(ctx context.Context, txn mvcc.TxnWrite, p *pb.PutRequest) (*pb.PutResponse, *traceutil.Trace, error)
	Range(ctx context.Context, txn mvcc.TxnRead, r *pb.RangeRequest) (*pb.RangeResponse, error)
	DeleteRange(txn mvcc.TxnWrite, dr *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error)
	Txn(ctx context.Context, rt *pb.TxnRequest) (*pb.TxnResponse, *traceutil.Trace, error)
	Compaction(compaction *pb.CompactionRequest) (*pb.CompactionResponse, <-chan struct{}, *traceutil.Trace, error)

	LeaseGrant(lc *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error)
	LeaseRevoke(lc *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error)

	LeaseCheckpoint(lc *pb.LeaseCheckpointRequest) (*pb.LeaseCheckpointResponse, error)

	Alarm(*pb.AlarmRequest) (*pb.AlarmResponse, error)

	Authenticate(r *pb.InternalAuthenticateRequest) (*pb.AuthenticateResponse, error)

	AuthEnable() (*pb.AuthEnableResponse, error)
	AuthDisable() (*pb.AuthDisableResponse, error)
	AuthStatus() (*pb.AuthStatusResponse, error)

	UserAdd(ua *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error)
	UserDelete(ua *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error)
	UserChangePassword(ua *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error)
	UserGrantRole(ua *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error)
	UserGet(ua *pb.AuthUserGetRequest) (*pb.AuthUserGetResponse, error)
	UserRevokeRole(ua *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error)
	RoleAdd(ua *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error)
	RoleGrantPermission(ua *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error)
	RoleGet(ua *pb.AuthRoleGetRequest) (*pb.AuthRoleGetResponse, error)
	RoleRevokePermission(ua *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error)
	RoleDelete(ua *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error)
	UserList(ua *pb.AuthUserListRequest) (*pb.AuthUserListResponse, error)
	RoleList(ua *pb.AuthRoleListRequest) (*pb.AuthRoleListResponse, error)
}

type applierV3backend struct {
	s *EtcdServer
}

func (s *EtcdServer) newApplierV3Backend() applierV3 {
	return &applierV3backend{s: s}
}

func (s *EtcdServer) newApplierV3Internal() applierV3Internal {
	base := &applierV3backend{s: s}
	return base
}

func (s *EtcdServer) newApplierV3() applierV3 {
	return newAuthApplierV3(
		s.AuthStore(),
		newQuotaApplierV3(s, s.newApplierV3Backend()),
		s.lessor,
	)
}

func (a *applierV3backend) Apply(r *pb.InternalRaftRequest, shouldApplyV3 membership.ShouldApplyV3) *applyResult {
	op := "unknown"
	ar := &applyResult{}
	defer func(start time.Time) {
		success := ar.err == nil || ar.err == mvcc.ErrCompacted
		applySec.WithLabelValues(v3Version, op, strconv.FormatBool(success)).Observe(time.Since(start).Seconds())
		warnOfExpensiveRequest(a.s.Logger(), a.s.Cfg.WarningApplyDuration, start, &pb.InternalRaftStringer{Request: r}, ar.resp, ar.err)
		if !success {
			warnOfFailedRequest(a.s.Logger(), start, &pb.InternalRaftStringer{Request: r}, ar.resp, ar.err)
		}
	}(time.Now())

	switch {
	case r.ClusterVersionSet != nil: // Implemented in 3.5.x
		op = "ClusterVersionSet"
		a.s.applyV3Internal.ClusterVersionSet(r.ClusterVersionSet, shouldApplyV3)
		return ar
	case r.ClusterMemberAttrSet != nil:
		op = "ClusterMemberAttrSet" // Implemented in 3.5.x
		a.s.applyV3Internal.ClusterMemberAttrSet(r.ClusterMemberAttrSet, shouldApplyV3)
		return ar
	case r.DowngradeInfoSet != nil:
		op = "DowngradeInfoSet" // Implemented in 3.5.x
		a.s.applyV3Internal.DowngradeInfoSet(r.DowngradeInfoSet, shouldApplyV3)
		return ar
	}

	if !shouldApplyV3 {
		return nil
	}

	// call into a.s.applyV3.F instead of a.F so upper appliers can check individual calls
	switch {
	case r.Range != nil:
		op = "Range"
		ar.resp, ar.err = a.s.applyV3.Range(context.TODO(), nil, r.Range)
	case r.Put != nil:
		op = "Put"
		ar.resp, ar.trace, ar.err = a.s.applyV3.Put(context.TODO(), nil, r.Put)
	case r.DeleteRange != nil:
		op = "DeleteRange"
		ar.resp, ar.err = a.s.applyV3.DeleteRange(nil, r.DeleteRange)
	case r.Txn != nil:
		op = "Txn"
		ar.resp, ar.trace, ar.err = a.s.applyV3.Txn(context.TODO(), r.Txn)
	case r.Compaction != nil:
		op = "Compaction"
		ar.resp, ar.physc, ar.trace, ar.err = a.s.applyV3.Compaction(r.Compaction)
	case r.LeaseGrant != nil:
		op = "LeaseGrant"
		ar.resp, ar.err = a.s.applyV3.LeaseGrant(r.LeaseGrant)
	case r.LeaseRevoke != nil:
		op = "LeaseRevoke"
		ar.resp, ar.err = a.s.applyV3.LeaseRevoke(r.LeaseRevoke)
	case r.LeaseCheckpoint != nil:
		op = "LeaseCheckpoint"
		ar.resp, ar.err = a.s.applyV3.LeaseCheckpoint(r.LeaseCheckpoint)
	case r.Alarm != nil:
		op = "Alarm"
		ar.resp, ar.err = a.s.applyV3.Alarm(r.Alarm)
	case r.Authenticate != nil:
		op = "Authenticate"
		ar.resp, ar.err = a.s.applyV3.Authenticate(r.Authenticate)
	case r.AuthEnable != nil:
		op = "AuthEnable"
		ar.resp, ar.err = a.s.applyV3.AuthEnable()
	case r.AuthDisable != nil:
		op = "AuthDisable"
		ar.resp, ar.err = a.s.applyV3.AuthDisable()
	case r.AuthStatus != nil:
		ar.resp, ar.err = a.s.applyV3.AuthStatus()
	case r.AuthUserAdd != nil:
		op = "AuthUserAdd"
		ar.resp, ar.err = a.s.applyV3.UserAdd(r.AuthUserAdd)
	case r.AuthUserDelete != nil:
		op = "AuthUserDelete"
		ar.resp, ar.err = a.s.applyV3.UserDelete(r.AuthUserDelete)
	case r.AuthUserChangePassword != nil:
		op = "AuthUserChangePassword"
		ar.resp, ar.err = a.s.applyV3.UserChangePassword(r.AuthUserChangePassword)
	case r.AuthUserGrantRole != nil:
		op = "AuthUserGrantRole"
		ar.resp, ar.err = a.s.applyV3.UserGrantRole(r.AuthUserGrantRole)
	case r.AuthUserGet != nil:
		op = "AuthUserGet"
		ar.resp, ar.err = a.s.applyV3.UserGet(r.AuthUserGet)
	case r.AuthUserRevokeRole != nil:
		op = "AuthUserRevokeRole"
		ar.resp, ar.err = a.s.applyV3.UserRevokeRole(r.AuthUserRevokeRole)
	case r.AuthRoleAdd != nil:
		op = "AuthRoleAdd"
		ar.resp, ar.err = a.s.applyV3.RoleAdd(r.AuthRoleAdd)
	case r.AuthRoleGrantPermission != nil:
		op = "AuthRoleGrantPermission"
		ar.resp, ar.err = a.s.applyV3.RoleGrantPermission(r.AuthRoleGrantPermission)
	case r.AuthRoleGet != nil:
		op = "AuthRoleGet"
		ar.resp, ar.err = a.s.applyV3.RoleGet(r.AuthRoleGet)
	case r.AuthRoleRevokePermission != nil:
		op = "AuthRoleRevokePermission"
		ar.resp, ar.err = a.s.applyV3.RoleRevokePermission(r.AuthRoleRevokePermission)
	case r.AuthRoleDelete != nil:
		op = "AuthRoleDelete"
		ar.resp, ar.err = a.s.applyV3.RoleDelete(r.AuthRoleDelete)
	case r.AuthUserList != nil:
		op = "AuthUserList"
		ar.resp, ar.err = a.s.applyV3.UserList(r.AuthUserList)
	case r.AuthRoleList != nil:
		op = "AuthRoleList"
		ar.resp, ar.err = a.s.applyV3.RoleList(r.AuthRoleList)
	default:
		a.s.lg.Panic("not implemented apply", zap.Stringer("raft-request", r))
	}
	return ar
}

func (a *applierV3backend) Put(ctx context.Context, txn mvcc.TxnWrite, p *pb.PutRequest) (resp *pb.PutResponse, trace *traceutil.Trace, err error) {
	return Put(ctx, a.s.Logger(), a.s.lessor, a.s.KV(), txn, p)
}

func (a *applierV3backend) DeleteRange(txn mvcc.TxnWrite, dr *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	return DeleteRange(a.s.KV(), txn, dr)
}

func (a *applierV3backend) Range(ctx context.Context, txn mvcc.TxnRead, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	return Range(ctx, a.s.Logger(), a.s.KV(), txn, r)
}

func (a *applierV3backend) Txn(ctx context.Context, rt *pb.TxnRequest) (*pb.TxnResponse, *traceutil.Trace, error) {
	return Txn(ctx, a.s.Logger(), rt, a.s.Cfg.ExperimentalTxnModeWriteWithSharedBuffer, a.s.KV(), a.s.lessor)
}

func (a *applierV3backend) Compaction(compaction *pb.CompactionRequest) (*pb.CompactionResponse, <-chan struct{}, *traceutil.Trace, error) {
	resp := &pb.CompactionResponse{}
	resp.Header = &pb.ResponseHeader{}
	trace := traceutil.New("compact",
		a.s.Logger(),
		traceutil.Field{Key: "revision", Value: compaction.Revision},
	)

	ch, err := a.s.KV().Compact(trace, compaction.Revision)
	if err != nil {
		return nil, ch, nil, err
	}
	// get the current revision. which key to get is not important.
	rr, _ := a.s.KV().Range(context.TODO(), []byte("compaction"), nil, mvcc.RangeOptions{})
	resp.Header.Revision = rr.Rev
	return resp, ch, trace, err
}

func (a *applierV3backend) LeaseGrant(lc *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	l, err := a.s.lessor.Grant(lease.LeaseID(lc.ID), lc.TTL)
	resp := &pb.LeaseGrantResponse{}
	if err == nil {
		resp.ID = int64(l.ID)
		resp.TTL = l.TTL()
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) LeaseRevoke(lc *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error) {
	err := a.s.lessor.Revoke(lease.LeaseID(lc.ID))
	return &pb.LeaseRevokeResponse{Header: newHeader(a.s)}, err
}

func (a *applierV3backend) LeaseCheckpoint(lc *pb.LeaseCheckpointRequest) (*pb.LeaseCheckpointResponse, error) {
	for _, c := range lc.Checkpoints {
		err := a.s.lessor.Checkpoint(lease.LeaseID(c.ID), c.Remaining_TTL)
		if err != nil {
			return &pb.LeaseCheckpointResponse{Header: newHeader(a.s)}, err
		}
	}
	return &pb.LeaseCheckpointResponse{Header: newHeader(a.s)}, nil
}

func (a *applierV3backend) Alarm(ar *pb.AlarmRequest) (*pb.AlarmResponse, error) {
	resp := &pb.AlarmResponse{}
	oldCount := len(a.s.alarmStore.Get(ar.Alarm))

	lg := a.s.Logger()
	switch ar.Action {
	case pb.AlarmRequest_GET:
		resp.Alarms = a.s.alarmStore.Get(ar.Alarm)
	case pb.AlarmRequest_ACTIVATE:
		if ar.Alarm == pb.AlarmType_NONE {
			break
		}
		m := a.s.alarmStore.Activate(types.ID(ar.MemberID), ar.Alarm)
		if m == nil {
			break
		}
		resp.Alarms = append(resp.Alarms, m)
		activated := oldCount == 0 && len(a.s.alarmStore.Get(m.Alarm)) == 1
		if !activated {
			break
		}

		lg.Warn("alarm raised", zap.String("alarm", m.Alarm.String()), zap.String("from", types.ID(m.MemberID).String()))
		switch m.Alarm {
		case pb.AlarmType_CORRUPT:
			a.s.applyV3 = newApplierV3Corrupt(a)
		case pb.AlarmType_NOSPACE:
			a.s.applyV3 = newApplierV3Capped(a)
		default:
			lg.Panic("unimplemented alarm activation", zap.String("alarm", fmt.Sprintf("%+v", m)))
		}
	case pb.AlarmRequest_DEACTIVATE:
		m := a.s.alarmStore.Deactivate(types.ID(ar.MemberID), ar.Alarm)
		if m == nil {
			break
		}
		resp.Alarms = append(resp.Alarms, m)
		deactivated := oldCount > 0 && len(a.s.alarmStore.Get(ar.Alarm)) == 0
		if !deactivated {
			break
		}

		switch m.Alarm {
		case pb.AlarmType_NOSPACE, pb.AlarmType_CORRUPT:
			// TODO: check kv hash before deactivating CORRUPT?
			lg.Warn("alarm disarmed", zap.String("alarm", m.Alarm.String()), zap.String("from", types.ID(m.MemberID).String()))
			a.s.applyV3 = a.s.newApplierV3()
		default:
			lg.Warn("unimplemented alarm deactivation", zap.String("alarm", fmt.Sprintf("%+v", m)))
		}
	default:
		return nil, nil
	}
	return resp, nil
}

type applierV3Capped struct {
	applierV3
	q serverstorage.BackendQuota
}

// newApplierV3Capped creates an applyV3 that will reject Puts and transactions
// with Puts so that the number of keys in the store is capped.
func newApplierV3Capped(base applierV3) applierV3 { return &applierV3Capped{applierV3: base} }

func (a *applierV3Capped) Put(ctx context.Context, txn mvcc.TxnWrite, p *pb.PutRequest) (*pb.PutResponse, *traceutil.Trace, error) {
	return nil, nil, ErrNoSpace
}

func (a *applierV3Capped) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, *traceutil.Trace, error) {
	if a.q.Cost(r) > 0 {
		return nil, nil, ErrNoSpace
	}
	return a.applierV3.Txn(ctx, r)
}

func (a *applierV3Capped) LeaseGrant(lc *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	return nil, ErrNoSpace
}

func (a *applierV3backend) AuthEnable() (*pb.AuthEnableResponse, error) {
	err := a.s.AuthStore().AuthEnable()
	if err != nil {
		return nil, err
	}
	return &pb.AuthEnableResponse{Header: newHeader(a.s)}, nil
}

func (a *applierV3backend) AuthDisable() (*pb.AuthDisableResponse, error) {
	a.s.AuthStore().AuthDisable()
	return &pb.AuthDisableResponse{Header: newHeader(a.s)}, nil
}

func (a *applierV3backend) AuthStatus() (*pb.AuthStatusResponse, error) {
	enabled := a.s.AuthStore().IsAuthEnabled()
	authRevision := a.s.AuthStore().Revision()
	return &pb.AuthStatusResponse{Header: newHeader(a.s), Enabled: enabled, AuthRevision: authRevision}, nil
}

func (a *applierV3backend) Authenticate(r *pb.InternalAuthenticateRequest) (*pb.AuthenticateResponse, error) {
	ctx := context.WithValue(context.WithValue(a.s.ctx, auth.AuthenticateParamIndex{}, a.s.consistIndex.ConsistentIndex()), auth.AuthenticateParamSimpleTokenPrefix{}, r.SimpleToken)
	resp, err := a.s.AuthStore().Authenticate(ctx, r.Name, r.Password)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserAdd(r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error) {
	resp, err := a.s.AuthStore().UserAdd(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserDelete(r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error) {
	resp, err := a.s.AuthStore().UserDelete(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserChangePassword(r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error) {
	resp, err := a.s.AuthStore().UserChangePassword(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserGrantRole(r *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error) {
	resp, err := a.s.AuthStore().UserGrantRole(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserGet(r *pb.AuthUserGetRequest) (*pb.AuthUserGetResponse, error) {
	resp, err := a.s.AuthStore().UserGet(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserRevokeRole(r *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error) {
	resp, err := a.s.AuthStore().UserRevokeRole(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) RoleAdd(r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error) {
	resp, err := a.s.AuthStore().RoleAdd(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) RoleGrantPermission(r *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error) {
	resp, err := a.s.AuthStore().RoleGrantPermission(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) RoleGet(r *pb.AuthRoleGetRequest) (*pb.AuthRoleGetResponse, error) {
	resp, err := a.s.AuthStore().RoleGet(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) RoleRevokePermission(r *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error) {
	resp, err := a.s.AuthStore().RoleRevokePermission(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) RoleDelete(r *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error) {
	resp, err := a.s.AuthStore().RoleDelete(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) UserList(r *pb.AuthUserListRequest) (*pb.AuthUserListResponse, error) {
	resp, err := a.s.AuthStore().UserList(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) RoleList(r *pb.AuthRoleListRequest) (*pb.AuthRoleListResponse, error) {
	resp, err := a.s.AuthStore().RoleList(r)
	if resp != nil {
		resp.Header = newHeader(a.s)
	}
	return resp, err
}

func (a *applierV3backend) ClusterVersionSet(r *membershippb.ClusterVersionSetRequest, shouldApplyV3 membership.ShouldApplyV3) {
	prevVersion := a.s.Cluster().Version()
	newVersion := semver.Must(semver.NewVersion(r.Ver))
	a.s.cluster.SetVersion(newVersion, api.UpdateCapability, shouldApplyV3)
	// Force snapshot after cluster version downgrade.
	if prevVersion != nil && newVersion.LessThan(*prevVersion) {
		lg := a.s.Logger()
		if lg != nil {
			lg.Info("Cluster version downgrade detected, forcing snapshot",
				zap.String("prev-cluster-version", prevVersion.String()),
				zap.String("new-cluster-version", newVersion.String()),
			)
		}
		a.s.forceSnapshot = true
	}
}

func (a *applierV3backend) ClusterMemberAttrSet(r *membershippb.ClusterMemberAttrSetRequest, shouldApplyV3 membership.ShouldApplyV3) {
	a.s.cluster.UpdateAttributes(
		types.ID(r.Member_ID),
		membership.Attributes{
			Name:       r.MemberAttributes.Name,
			ClientURLs: r.MemberAttributes.ClientUrls,
		},
		shouldApplyV3,
	)
}

func (a *applierV3backend) DowngradeInfoSet(r *membershippb.DowngradeInfoSetRequest, shouldApplyV3 membership.ShouldApplyV3) {
	d := version.DowngradeInfo{Enabled: false}
	if r.Enabled {
		d = version.DowngradeInfo{Enabled: true, TargetVersion: r.Ver}
	}
	a.s.cluster.SetDowngradeInfo(&d, shouldApplyV3)
}

type quotaApplierV3 struct {
	applierV3
	q serverstorage.Quota
}

func newQuotaApplierV3(s *EtcdServer, app applierV3) applierV3 {
	return &quotaApplierV3{app, serverstorage.NewBackendQuota(s.Cfg, s.Backend(), "v3-applier")}
}

func (a *quotaApplierV3) Put(ctx context.Context, txn mvcc.TxnWrite, p *pb.PutRequest) (*pb.PutResponse, *traceutil.Trace, error) {
	ok := a.q.Available(p)
	resp, trace, err := a.applierV3.Put(ctx, txn, p)
	if err == nil && !ok {
		err = ErrNoSpace
	}
	return resp, trace, err
}

func (a *quotaApplierV3) Txn(ctx context.Context, rt *pb.TxnRequest) (*pb.TxnResponse, *traceutil.Trace, error) {
	ok := a.q.Available(rt)
	resp, trace, err := a.applierV3.Txn(ctx, rt)
	if err == nil && !ok {
		err = ErrNoSpace
	}
	return resp, trace, err
}

func (a *quotaApplierV3) LeaseGrant(lc *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	ok := a.q.Available(lc)
	resp, err := a.applierV3.LeaseGrant(lc)
	if err == nil && !ok {
		err = ErrNoSpace
	}
	return resp, err
}

func noSideEffect(r *pb.InternalRaftRequest) bool {
	return r.Range != nil || r.AuthUserGet != nil || r.AuthRoleGet != nil || r.AuthStatus != nil
}

func removeNeedlessRangeReqs(txn *pb.TxnRequest) {
	f := func(ops []*pb.RequestOp) []*pb.RequestOp {
		j := 0
		for i := 0; i < len(ops); i++ {
			if _, ok := ops[i].Request.(*pb.RequestOp_RequestRange); ok {
				continue
			}
			ops[j] = ops[i]
			j++
		}

		return ops[:j]
	}

	txn.Success = f(txn.Success)
	txn.Failure = f(txn.Failure)
}

func newHeader(s *EtcdServer) *pb.ResponseHeader {
	return &pb.ResponseHeader{
		ClusterId: uint64(s.Cluster().ID()),
		MemberId:  uint64(s.ID()),
		Revision:  s.KV().Rev(),
		RaftTerm:  s.Term(),
	}
}
