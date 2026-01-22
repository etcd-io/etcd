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

package v3rpc

import (
	"context"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
)

type ClusterServer struct {
	cluster api.Cluster
	server  *etcdserver.EtcdServer
}

func NewClusterServer(s *etcdserver.EtcdServer) *ClusterServer {
	return &ClusterServer{
		cluster: s.Cluster(),
		server:  s,
	}
}

func (cs *ClusterServer) MemberAdd(ctx context.Context, r *pb.MemberAddRequest) (*pb.MemberAddResponse, error) {
	urls, err := types.NewURLs(r.PeerURLs)
	if err != nil {
		return nil, rpctypes.ErrGRPCMemberBadURLs
	}

	now := time.Now()
	var m *membership.Member
	if r.IsLearner {
		m = membership.NewMemberAsLearner("", urls, "", &now)
	} else {
		m = membership.NewMember("", urls, "", &now)
	}
	membs, merr := cs.server.AddMember(ctx, *m)
	if merr != nil {
		return nil, togRPCError(merr)
	}

	return &pb.MemberAddResponse{
		Header: cs.header(),
		Member: &pb.Member{
			ID:        uint64(m.ID),
			PeerURLs:  m.PeerURLs,
			IsLearner: m.IsLearner,
		},
		Members: membersToProtoMembers(membs),
	}, nil
}

func (cs *ClusterServer) MemberRemove(ctx context.Context, r *pb.MemberRemoveRequest) (*pb.MemberRemoveResponse, error) {
	membs, err := cs.server.RemoveMember(ctx, r.ID)
	if err != nil {
		return nil, togRPCError(err)
	}
	return &pb.MemberRemoveResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

func (cs *ClusterServer) MemberUpdate(ctx context.Context, r *pb.MemberUpdateRequest) (*pb.MemberUpdateResponse, error) {
	m := membership.Member{
		ID:             types.ID(r.ID),
		RaftAttributes: membership.RaftAttributes{PeerURLs: r.PeerURLs},
	}
	membs, err := cs.server.UpdateMember(ctx, m)
	if err != nil {
		return nil, togRPCError(err)
	}
	return &pb.MemberUpdateResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

func (cs *ClusterServer) MemberList(ctx context.Context, r *pb.MemberListRequest) (*pb.MemberListResponse, error) {
	if r.Linearizable {
		if err := cs.server.LinearizableReadNotify(ctx); err != nil {
			return nil, togRPCError(err)
		}
	}
	membs := membersToProtoMembers(cs.cluster.Members())
	return &pb.MemberListResponse{Header: cs.header(), Members: membs}, nil
}

func (cs *ClusterServer) MemberPromote(ctx context.Context, r *pb.MemberPromoteRequest) (*pb.MemberPromoteResponse, error) {
	membs, err := cs.server.PromoteMember(ctx, r.ID)
	if err != nil {
		return nil, togRPCError(err)
	}
	return &pb.MemberPromoteResponse{Header: cs.header(), Members: membersToProtoMembers(membs)}, nil
}

// MemberWatch watches for membership changes in the cluster.
func (cs *ClusterServer) MemberWatch(r *pb.MemberWatchRequest, stream pb.Cluster_MemberWatchServer) error {
	// Get the membership event broadcaster from the cluster
	broadcaster := cs.cluster.MemberEventBroadcaster()
	if broadcaster == nil {
		return rpctypes.ErrGRPCMemberWatchNotSupported
	}

	// Subscribe to membership events
	subID, eventCh := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(subID)

	// If requested, send current members as the first response
	if r.IncludeCurrentMembers {
		membs := membersToProtoMembers(cs.cluster.Members())
		resp := &pb.MemberWatchResponse{
			Header:  cs.header(),
			Members: membs,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	// Stream membership events
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed, subscription ended
				return nil
			}

			// Convert membership event to proto event
			protoEvent := memberEventToProto(event)
			resp := &pb.MemberWatchResponse{
				Header: cs.header(),
				Events: []*pb.MemberEvent{protoEvent},
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

// memberEventToProto converts a membership.MemberEvent to a pb.MemberEvent.
func memberEventToProto(event membership.MemberEvent) *pb.MemberEvent {
	var eventType pb.MemberEventType
	switch event.Type {
	case membership.MemberAdded:
		eventType = pb.MemberEventType_MEMBER_ADDED
	case membership.MemberRemoved:
		eventType = pb.MemberEventType_MEMBER_REMOVED
	case membership.MemberUpdated:
		eventType = pb.MemberEventType_MEMBER_UPDATED
	case membership.MemberPromoted:
		eventType = pb.MemberEventType_MEMBER_PROMOTED
	}

	return &pb.MemberEvent{
		Type: eventType,
		Member: &pb.Member{
			Name:       event.Member.Name,
			ID:         uint64(event.Member.ID),
			PeerURLs:   event.Member.PeerURLs,
			ClientURLs: event.Member.ClientURLs,
			IsLearner:  event.Member.IsLearner,
		},
	}
}

func (cs *ClusterServer) header() *pb.ResponseHeader {
	return &pb.ResponseHeader{ClusterId: uint64(cs.cluster.ID()), MemberId: uint64(cs.server.MemberID()), RaftTerm: cs.server.Term()}
}

func membersToProtoMembers(membs []*membership.Member) []*pb.Member {
	protoMembs := make([]*pb.Member, len(membs))
	for i := range membs {
		protoMembs[i] = &pb.Member{
			Name:       membs[i].Name,
			ID:         uint64(membs[i].ID),
			PeerURLs:   membs[i].PeerURLs,
			ClientURLs: membs[i].ClientURLs,
			IsLearner:  membs[i].IsLearner,
		}
	}
	return protoMembs
}
