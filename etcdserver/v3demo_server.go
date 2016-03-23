// Copyright 2015 CoreOS, Inc.
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
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/lease/leasehttp"
	dstorage "github.com/coreos/etcd/storage"
	"golang.org/x/net/context"
)

const (
	// the max request size that raft accepts.
	// TODO: make this a flag? But we probably do not want to
	// accept large request which might block raft stream. User
	// specify a large value might end up with shooting in the foot.
	maxRequestBytes = 1.5 * 1024 * 1024
)

type RaftKV interface {
	Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error)
	Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error)
	DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error)
	Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error)
	Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error)
	Hash(ctx context.Context, r *pb.HashRequest) (*pb.HashResponse, error)
}

type Lessor interface {
	// LeaseCreate sends LeaseCreate request to raft and apply it after committed.
	LeaseCreate(ctx context.Context, r *pb.LeaseCreateRequest) (*pb.LeaseCreateResponse, error)
	// LeaseRevoke sends LeaseRevoke request to raft and apply it after committed.
	LeaseRevoke(ctx context.Context, r *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error)

	// LeaseRenew renews the lease with given ID. The renewed TTL is returned. Or an error
	// is returned.
	LeaseRenew(id lease.LeaseID) (int64, error)
}

type Authenticator interface {
	AuthEnable(ctx context.Context, r *pb.AuthEnableRequest) (*pb.AuthEnableResponse, error)
	UserAdd(ctx context.Context, r *pb.UserAddRequest) (*pb.UserAddResponse, error)
}

func (s *EtcdServer) Range(ctx context.Context, r *pb.RangeRequest) (*pb.RangeResponse, error) {
	if r.Serializable {
		return s.applyV3.Range(noTxn, r)
	}

	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Range: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.RangeResponse), result.err
}

func (s *EtcdServer) Put(ctx context.Context, r *pb.PutRequest) (*pb.PutResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Put: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.PutResponse), result.err
}

func (s *EtcdServer) DeleteRange(ctx context.Context, r *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{DeleteRange: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.DeleteRangeResponse), result.err
}

func (s *EtcdServer) Txn(ctx context.Context, r *pb.TxnRequest) (*pb.TxnResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Txn: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.TxnResponse), result.err
}

func (s *EtcdServer) Compact(ctx context.Context, r *pb.CompactionRequest) (*pb.CompactionResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{Compaction: r})
	if err != nil {
		return nil, err
	}
	resp := result.resp.(*pb.CompactionResponse)
	if resp == nil {
		resp = &pb.CompactionResponse{}
	}
	if resp.Header == nil {
		resp.Header = &pb.ResponseHeader{}
	}
	resp.Header.Revision = s.kv.Rev()
	return resp, result.err
}

func (s *EtcdServer) Hash(ctx context.Context, r *pb.HashRequest) (*pb.HashResponse, error) {
	h, err := s.be.Hash()
	if err != nil {
		return nil, err
	}
	return &pb.HashResponse{Header: &pb.ResponseHeader{Revision: s.kv.Rev()}, Hash: h}, nil
}

func (s *EtcdServer) LeaseCreate(ctx context.Context, r *pb.LeaseCreateRequest) (*pb.LeaseCreateResponse, error) {
	// no id given? choose one
	for r.ID == int64(lease.NoLease) {
		// only use positive int64 id's
		r.ID = int64(s.reqIDGen.Next() & ((1 << 63) - 1))
	}
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{LeaseCreate: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.LeaseCreateResponse), result.err
}

func (s *EtcdServer) LeaseRevoke(ctx context.Context, r *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{LeaseRevoke: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.LeaseRevokeResponse), result.err
}

func (s *EtcdServer) LeaseRenew(id lease.LeaseID) (int64, error) {
	ttl, err := s.lessor.Renew(id)
	if err == nil {
		return ttl, nil
	}
	if err != lease.ErrNotPrimary {
		return -1, err
	}

	// renewals don't go through raft; forward to leader manually
	leader := s.cluster.Member(s.Leader())
	for i := 0; i < 5 && leader == nil; i++ {
		// wait an election
		dur := time.Duration(s.cfg.ElectionTicks) * time.Duration(s.cfg.TickMs) * time.Millisecond
		select {
		case <-time.After(dur):
			leader = s.cluster.Member(s.Leader())
		case <-s.done:
			return -1, ErrStopped
		}
	}
	if leader == nil || len(leader.PeerURLs) == 0 {
		return -1, ErrNoLeader
	}

	for _, url := range leader.PeerURLs {
		lurl := url + "/leases"
		ttl, err = leasehttp.RenewHTTP(id, lurl, s.peerRt, s.cfg.peerDialTimeout())
		if err == nil {
			break
		}
	}
	return ttl, err
}

func (s *EtcdServer) AuthEnable(ctx context.Context, r *pb.AuthEnableRequest) (*pb.AuthEnableResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{AuthEnable: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.AuthEnableResponse), result.err
}

func (s *EtcdServer) UserAdd(ctx context.Context, r *pb.UserAddRequest) (*pb.UserAddResponse, error) {
	result, err := s.processInternalRaftRequest(ctx, pb.InternalRaftRequest{UserAdd: r})
	if err != nil {
		return nil, err
	}
	return result.resp.(*pb.UserAddResponse), result.err
}

func (s *EtcdServer) processInternalRaftRequest(ctx context.Context, r pb.InternalRaftRequest) (*applyResult, error) {
	r.ID = s.reqIDGen.Next()

	data, err := r.Marshal()
	if err != nil {
		return nil, err
	}

	if len(data) > maxRequestBytes {
		return nil, ErrRequestTooLarge
	}

	ch := s.w.Register(r.ID)

	s.r.Propose(ctx, data)

	select {
	case x := <-ch:
		return x.(*applyResult), nil
	case <-ctx.Done():
		s.w.Trigger(r.ID, nil) // GC wait
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrStopped
	}
}

// Watchable returns a watchable interface attached to the etcdserver.
func (s *EtcdServer) Watchable() dstorage.Watchable {
	return s.getKV()
}
