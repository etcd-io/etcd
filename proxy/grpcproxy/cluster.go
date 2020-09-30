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

package grpcproxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/clientv3/naming"
	"go.etcd.io/etcd/v3/etcdserver/api/v3rpc/rpctypes"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	gnaming "google.golang.org/grpc/naming"
)

// allow maximum 1 retry per second
const resolveRetryRate = 1

type clusterProxy struct {
	lg   *zap.Logger
	clus clientv3.Cluster
	ctx  context.Context
	gr   *naming.GRPCResolver

	// advertise client URL
	advaddr string
	prefix  string

	umu  sync.RWMutex
	umap map[string]gnaming.Update
}

// NewClusterProxy takes optional prefix to fetch grpc-proxy member endpoints.
// The returned channel is closed when there is grpc-proxy endpoint registered
// and the client's context is canceled so the 'register' loop returns.
func NewClusterProxy(lg *zap.Logger, c *clientv3.Client, advaddr string, prefix string) (pb.ClusterServer, <-chan struct{}) {
	if lg == nil {
		lg = zap.NewNop()
	}
	cp := &clusterProxy{
		lg:   lg,
		clus: c.Cluster,
		ctx:  c.Ctx(),
		gr:   &naming.GRPCResolver{Client: c},

		advaddr: advaddr,
		prefix:  prefix,
		umap:    make(map[string]gnaming.Update),
	}

	donec := make(chan struct{})
	if advaddr != "" && prefix != "" {
		go func() {
			defer close(donec)
			cp.resolve(prefix)
		}()
		return cp, donec
	}

	close(donec)
	return cp, donec
}

func (cp *clusterProxy) resolve(prefix string) {
	rm := rate.NewLimiter(rate.Limit(resolveRetryRate), resolveRetryRate)
	for rm.Wait(cp.ctx) == nil {
		wa, err := cp.gr.Resolve(prefix)
		if err != nil {
			cp.lg.Warn("failed to resolve prefix", zap.String("prefix", prefix), zap.Error(err))
			continue
		}
		cp.monitor(wa)
	}
}

func (cp *clusterProxy) monitor(wa gnaming.Watcher) {
	for cp.ctx.Err() == nil {
		ups, err := wa.Next()
		if err != nil {
			cp.lg.Warn("clusterProxy watcher error", zap.Error(err))
			if rpctypes.ErrorDesc(err) == naming.ErrWatcherClosed.Error() {
				return
			}
		}

		cp.umu.Lock()
		for i := range ups {
			switch ups[i].Op {
			case gnaming.Add:
				cp.umap[ups[i].Addr] = *ups[i]
			case gnaming.Delete:
				delete(cp.umap, ups[i].Addr)
			}
		}
		cp.umu.Unlock()
	}
}

func (cp *clusterProxy) MemberAdd(ctx context.Context, r *pb.MemberAddRequest) (*pb.MemberAddResponse, error) {
	if r.IsLearner {
		return cp.memberAddAsLearner(ctx, r.PeerURLs)
	}
	return cp.memberAdd(ctx, r.PeerURLs)
}

func (cp *clusterProxy) memberAdd(ctx context.Context, peerURLs []string) (*pb.MemberAddResponse, error) {
	mresp, err := cp.clus.MemberAdd(ctx, peerURLs)
	if err != nil {
		return nil, err
	}
	resp := (pb.MemberAddResponse)(*mresp)
	return &resp, err
}

func (cp *clusterProxy) memberAddAsLearner(ctx context.Context, peerURLs []string) (*pb.MemberAddResponse, error) {
	mresp, err := cp.clus.MemberAddAsLearner(ctx, peerURLs)
	if err != nil {
		return nil, err
	}
	resp := (pb.MemberAddResponse)(*mresp)
	return &resp, err
}

func (cp *clusterProxy) MemberRemove(ctx context.Context, r *pb.MemberRemoveRequest) (*pb.MemberRemoveResponse, error) {
	mresp, err := cp.clus.MemberRemove(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	resp := (pb.MemberRemoveResponse)(*mresp)
	return &resp, err
}

func (cp *clusterProxy) MemberUpdate(ctx context.Context, r *pb.MemberUpdateRequest) (*pb.MemberUpdateResponse, error) {
	mresp, err := cp.clus.MemberUpdate(ctx, r.ID, r.PeerURLs)
	if err != nil {
		return nil, err
	}
	resp := (pb.MemberUpdateResponse)(*mresp)
	return &resp, err
}

func (cp *clusterProxy) membersFromUpdates() ([]*pb.Member, error) {
	cp.umu.RLock()
	defer cp.umu.RUnlock()
	mbs := make([]*pb.Member, 0, len(cp.umap))
	for addr, upt := range cp.umap {
		m, err := decodeMeta(fmt.Sprint(upt.Metadata))
		if err != nil {
			return nil, err
		}
		mbs = append(mbs, &pb.Member{Name: m.Name, ClientURLs: []string{addr}})
	}
	return mbs, nil
}

// MemberList wraps member list API with following rules:
// - If 'advaddr' is not empty and 'prefix' is not empty, return registered member lists via resolver
// - If 'advaddr' is not empty and 'prefix' is not empty and registered grpc-proxy members haven't been fetched, return the 'advaddr'
// - If 'advaddr' is not empty and 'prefix' is empty, return 'advaddr' without forcing it to 'register'
// - If 'advaddr' is empty, forward to member list API
func (cp *clusterProxy) MemberList(ctx context.Context, r *pb.MemberListRequest) (*pb.MemberListResponse, error) {
	if cp.advaddr != "" {
		if cp.prefix != "" {
			mbs, err := cp.membersFromUpdates()
			if err != nil {
				return nil, err
			}
			if len(mbs) > 0 {
				return &pb.MemberListResponse{Members: mbs}, nil
			}
		}
		// prefix is empty or no grpc-proxy members haven't been registered
		hostname, _ := os.Hostname()
		return &pb.MemberListResponse{Members: []*pb.Member{{Name: hostname, ClientURLs: []string{cp.advaddr}}}}, nil
	}
	mresp, err := cp.clus.MemberList(ctx)
	if err != nil {
		return nil, err
	}
	resp := (pb.MemberListResponse)(*mresp)
	return &resp, err
}

func (cp *clusterProxy) MemberPromote(ctx context.Context, r *pb.MemberPromoteRequest) (*pb.MemberPromoteResponse, error) {
	// TODO: implement
	return nil, errors.New("not implemented")
}
