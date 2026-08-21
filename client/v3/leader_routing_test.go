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

package clientv3

import (
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

// TestIsMutationRequest verifies the leader-aware request classification.
//
// Eligible requests are consensus writes, which any member submits to Raft, and
// the leader-only MoveLeader. Reads, streams, and member-local operations stay
// on round_robin.
func TestIsMutationRequest(t *testing.T) {
	putOp := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: &pb.PutRequest{}}}
	deleteOp := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{RequestDeleteRange: &pb.DeleteRangeRequest{}}}
	rangeOp := &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: &pb.RangeRequest{}}}
	nestedTxnWithPutOp := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{RequestTxn: &pb.TxnRequest{
		Failure: []*pb.RequestOp{putOp},
	}}}
	nestedReadOnlyTxnOp := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{RequestTxn: &pb.TxnRequest{
		Success: []*pb.RequestOp{rangeOp},
		Failure: []*pb.RequestOp{rangeOp},
	}}}

	tests := []struct {
		name string
		req  any
		want bool
	}{
		// KV consensus writes.
		{"put", &pb.PutRequest{}, true},
		{"delete range", &pb.DeleteRangeRequest{}, true},
		{"compaction", &pb.CompactionRequest{}, true},
		{"txn with put in success branch", &pb.TxnRequest{Success: []*pb.RequestOp{putOp}, Failure: []*pb.RequestOp{rangeOp}}, true},
		{"txn with delete in failure branch", &pb.TxnRequest{Success: []*pb.RequestOp{rangeOp}, Failure: []*pb.RequestOp{deleteOp}}, true},
		{"txn with mutation only in nested txn", &pb.TxnRequest{Success: []*pb.RequestOp{rangeOp, nestedTxnWithPutOp}}, true},
		{"read-only txn", &pb.TxnRequest{Success: []*pb.RequestOp{rangeOp}, Failure: []*pb.RequestOp{rangeOp}}, false},
		// The server's IsTxnReadonly requires every operation in both branches
		// to be a Range.
		//
		// A nested Txn, even a read-only one, takes the Raft proposal path and
		// uses leader-aware routing here.
		{"read-only nested txn", &pb.TxnRequest{Success: []*pb.RequestOp{nestedReadOnlyTxnOp}}, true},
		// A nil op is not a Range, so the server treats the transaction as a
		// write; an empty transaction is read-only on both sides.
		{"txn with nil op", &pb.TxnRequest{Success: []*pb.RequestOp{nil}}, true},
		{"empty txn", &pb.TxnRequest{}, false},
		{"range", &pb.RangeRequest{}, false},

		// Lease grant and revoke are Raft proposals; keep-alive is a stream
		// the unary interceptor never sees, and the lease reads are reads.
		{"lease grant", &pb.LeaseGrantRequest{}, true},
		{"lease revoke", &pb.LeaseRevokeRequest{}, true},
		{"lease keep alive request", &pb.LeaseKeepAliveRequest{}, false},
		{"lease time to live", &pb.LeaseTimeToLiveRequest{}, false},
		{"lease leases", &pb.LeaseLeasesRequest{}, false},

		// Auth administration proposes through Raft, including Authenticate,
		// which registers the issued token. Auth reads are reads.
		{"authenticate", &pb.AuthenticateRequest{}, true},
		{"auth enable", &pb.AuthEnableRequest{}, true},
		{"auth disable", &pb.AuthDisableRequest{}, true},
		{"auth user add", &pb.AuthUserAddRequest{}, true},
		{"auth user delete", &pb.AuthUserDeleteRequest{}, true},
		{"auth user change password", &pb.AuthUserChangePasswordRequest{}, true},
		{"auth user grant role", &pb.AuthUserGrantRoleRequest{}, true},
		{"auth user revoke role", &pb.AuthUserRevokeRoleRequest{}, true},
		{"auth role add", &pb.AuthRoleAddRequest{}, true},
		{"auth role delete", &pb.AuthRoleDeleteRequest{}, true},
		{"auth role grant permission", &pb.AuthRoleGrantPermissionRequest{}, true},
		{"auth role revoke permission", &pb.AuthRoleRevokePermissionRequest{}, true},
		{"auth status", &pb.AuthStatusRequest{}, false},
		{"auth user get", &pb.AuthUserGetRequest{}, false},
		{"auth user list", &pb.AuthUserListRequest{}, false},
		{"auth role get", &pb.AuthRoleGetRequest{}, false},
		{"auth role list", &pb.AuthRoleListRequest{}, false},

		// MoveLeader is leader-only. A follower rejects it instead of forwarding,
		// so leader-aware routing fixes first-attempt routing.
		{"move leader", &pb.MoveLeaderRequest{}, true},

		// The server submits every Alarm action to Raft, including GET.
		{"alarm get", &pb.AlarmRequest{Action: pb.AlarmRequest_GET}, true},
		{"alarm deactivate", &pb.AlarmRequest{Action: pb.AlarmRequest_DEACTIVATE}, true},

		// Member-local maintenance and rare or action-mixed cluster-admin
		// operations stay on round_robin.
		{"defragment", &pb.DefragmentRequest{}, false},
		{"status", &pb.StatusRequest{}, false},
		{"hash kv", &pb.HashKVRequest{}, false},
		{"downgrade", &pb.DowngradeRequest{Action: pb.DowngradeRequest_ENABLE}, false},
		{"member list", &pb.MemberListRequest{}, false},
		{"member add", &pb.MemberAddRequest{}, false},
		{"member remove", &pb.MemberRemoveRequest{}, false},
		{"member update", &pb.MemberUpdateRequest{}, false},
		{"member promote", &pb.MemberPromoteRequest{}, false},

		{"unknown request", struct{}{}, false},
		{"nil request", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMutationRequest(tt.req); got != tt.want {
				t.Errorf("isMutationRequest(%T) = %v, want %v", tt.req, got, tt.want)
			}
		})
	}
}
