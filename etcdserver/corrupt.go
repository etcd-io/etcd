// Copyright 2017 The etcd Authors
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

	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc"
	"github.com/coreos/etcd/pkg/types"

	"golang.org/x/net/context"
)

func (s *EtcdServer) monitorKVHash() {
	t := s.Cfg.CorruptCheckTime
	if t == 0 {
		return
	}
	plog.Infof("enabled corruption checking with %s interval", t)
	for {
		select {
		case <-s.stopping:
			return
		case <-time.After(t):
		}
		if !s.isLeader() {
			continue
		}
		if err := s.checkHashKV(); err != nil {
			plog.Debugf("check hash kv failed %v", err)
		}
	}
}

func (s *EtcdServer) checkHashKV() error {
	h, rev, crev, err := s.kv.HashByRev(0)
	if err != nil {
		plog.Fatalf("failed to hash kv store (%v)", err)
	}
	resps := []*clientv3.HashKVResponse{}
	for _, m := range s.cluster.Members() {
		if m.ID == s.ID() {
			continue
		}

		cli, cerr := clientv3.New(clientv3.Config{Endpoints: m.PeerURLs})
		if cerr != nil {
			continue
		}

		respsLen := len(resps)
		for _, c := range cli.Endpoints() {
			ctx, cancel := context.WithTimeout(context.Background(), s.Cfg.ReqTimeout())
			resp, herr := cli.HashKV(ctx, c, rev)
			cancel()
			if herr == nil {
				cerr = herr
				resps = append(resps, resp)
				break
			}
		}
		cli.Close()

		if respsLen == len(resps) {
			plog.Warningf("failed to hash kv for peer %s (%v)", types.ID(m.ID), cerr)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Cfg.ReqTimeout())
	err = s.linearizableReadNotify(ctx)
	cancel()
	if err != nil {
		return err
	}

	h2, rev2, crev2, err := s.kv.HashByRev(0)
	if err != nil {
		plog.Warningf("failed to hash kv store (%v)", err)
		return err
	}

	alarmed := false
	mismatch := func(id uint64) {
		if alarmed {
			return
		}
		alarmed = true
		a := &pb.AlarmRequest{
			MemberID: uint64(id),
			Action:   pb.AlarmRequest_ACTIVATE,
			Alarm:    pb.AlarmType_CORRUPT,
		}
		s.goAttach(func() {
			s.raftRequest(s.ctx, pb.InternalRaftRequest{Alarm: a})
		})
	}

	if h2 != h && rev2 == rev && crev == crev2 {
		plog.Warningf("mismatched hashes %d and %d for revision %d", h, h2, rev)
		mismatch(uint64(s.ID()))
	}

	for _, resp := range resps {
		id := resp.Header.MemberId
		if resp.Header.Revision > rev2 {
			plog.Warningf(
				"revision %d from member %v, expected at most %d",
				resp.Header.Revision,
				types.ID(id),
				rev2)
			mismatch(id)
		}
		if resp.CompactRevision > crev2 {
			plog.Warningf(
				"compact revision %d from member %v, expected at most %d",
				resp.CompactRevision,
				types.ID(id),
				crev2,
			)
			mismatch(id)
		}
		if resp.CompactRevision == crev && resp.Hash != h {
			plog.Warningf(
				"hash %d at revision %d from member %v, expected hash %d",
				resp.Hash,
				rev,
				types.ID(id),
				h,
			)
			mismatch(id)
		}
	}
	return nil
}

type applierV3Corrupt struct {
	applierV3
}

func newApplierV3Corrupt(a applierV3) *applierV3Corrupt { return &applierV3Corrupt{a} }

func (a *applierV3Corrupt) Put(txn mvcc.TxnWrite, p *pb.PutRequest) (*pb.PutResponse, error) {
	return nil, ErrCorrupt
}

func (a *applierV3Corrupt) Range(txn mvcc.TxnRead, p *pb.RangeRequest) (*pb.RangeResponse, error) {
	return nil, ErrCorrupt
}

func (a *applierV3Corrupt) DeleteRange(txn mvcc.TxnWrite, p *pb.DeleteRangeRequest) (*pb.DeleteRangeResponse, error) {
	return nil, ErrCorrupt
}

func (a *applierV3Corrupt) Txn(rt *pb.TxnRequest) (*pb.TxnResponse, error) {
	return nil, ErrCorrupt
}

func (a *applierV3Corrupt) Compaction(compaction *pb.CompactionRequest) (*pb.CompactionResponse, <-chan struct{}, error) {
	return nil, nil, ErrCorrupt
}

func (a *applierV3Corrupt) LeaseGrant(lc *pb.LeaseGrantRequest) (*pb.LeaseGrantResponse, error) {
	return nil, ErrCorrupt
}

func (a *applierV3Corrupt) LeaseRevoke(lc *pb.LeaseRevokeRequest) (*pb.LeaseRevokeResponse, error) {
	return nil, ErrCorrupt
}
