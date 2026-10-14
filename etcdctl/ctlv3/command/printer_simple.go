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

package command

import (
	"fmt"
	"os"
	"strings"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	v3 "go.etcd.io/etcd/client/v3"
)

const rootRole = "root"

type simplePrinter struct {
	isHex     bool
	valueOnly bool
}

func (s *simplePrinter) Del(resp *v3.DeleteResponse) {
	r := (*pb.DeleteRangeResponse)(resp)
	fmt.Println(r.GetDeleted())
	for _, kv := range r.GetPrevKvs() {
		printKV(s.isHex, s.valueOnly, kv)
	}
}

func (s *simplePrinter) Get(resp *v3.GetResponse) {
	r := (*pb.RangeResponse)(resp)
	for _, kv := range r.GetKvs() {
		printKV(s.isHex, s.valueOnly, kv)
	}
}

func (s *simplePrinter) Put(r *v3.PutResponse) {
	resp := (*pb.PutResponse)(r)
	fmt.Println("OK")
	if resp.GetPrevKv() != nil {
		printKV(s.isHex, s.valueOnly, resp.GetPrevKv())
	}
}

func (s *simplePrinter) Txn(resp *v3.TxnResponse) {
	r := (*pb.TxnResponse)(resp)
	if r.GetSucceeded() {
		fmt.Println("SUCCESS")
	} else {
		fmt.Println("FAILURE")
	}

	for _, opResp := range r.GetResponses() {
		fmt.Println("")
		switch v := opResp.GetResponse().(type) {
		case *pb.ResponseOp_ResponseDeleteRange:
			s.Del((*v3.DeleteResponse)(v.ResponseDeleteRange))
		case *pb.ResponseOp_ResponsePut:
			s.Put((*v3.PutResponse)(v.ResponsePut))
		case *pb.ResponseOp_ResponseRange:
			s.Get((*v3.GetResponse)(v.ResponseRange))
		default:
			fmt.Printf("unexpected response %+v\n", opResp)
		}
	}
}

func (s *simplePrinter) Watch(resp *v3.WatchResponse) {
	for _, event := range resp.Events {
		fmt.Println(event.GetType())
		if event.GetPrevKv() != nil {
			printKV(s.isHex, s.valueOnly, event.GetPrevKv())
		}
		printKV(s.isHex, s.valueOnly, event.GetKv())
	}
}

func (s *simplePrinter) Grant(resp *v3.LeaseGrantResponse) {
	fmt.Printf("lease %016x granted with TTL(%ds)\n", resp.ID, resp.TTL)
}

func (s *simplePrinter) Revoke(id v3.LeaseID, r *v3.LeaseRevokeResponse) {
	fmt.Printf("lease %016x revoked\n", id)
}

func (s *simplePrinter) KeepAlive(resp *v3.LeaseKeepAliveResponse) {
	fmt.Printf("lease %016x keepalived with TTL(%d)\n", resp.ID, resp.TTL)
}

func (s *simplePrinter) TimeToLive(resp *v3.LeaseTimeToLiveResponse, keys bool) {
	if resp.GrantedTTL == 0 && resp.TTL == -1 {
		fmt.Printf("lease %016x already expired\n", resp.ID)
		return
	}

	txt := fmt.Sprintf("lease %016x granted with TTL(%ds), remaining(%ds)", resp.ID, resp.GrantedTTL, resp.TTL)
	if keys {
		ks := make([]string, len(resp.Keys))
		for i := range resp.Keys {
			ks[i] = string(resp.Keys[i])
		}
		txt += fmt.Sprintf(", attached keys(%v)", ks)
	}
	fmt.Println(txt)
}

func (s *simplePrinter) Leases(resp *v3.LeaseLeasesResponse) {
	fmt.Printf("found %d leases\n", len(resp.Leases))
	for _, item := range resp.Leases {
		fmt.Printf("%016x\n", item.ID)
	}
}

func (s *simplePrinter) Alarm(resp *v3.AlarmResponse) {
	r := (*pb.AlarmResponse)(resp)
	for _, e := range r.GetAlarms() {
		fmt.Printf("%+v\n", e)
	}
}

func (s *simplePrinter) MemberAdd(r *v3.MemberAddResponse) {
	resp := (*pb.MemberAddResponse)(r)
	asLearner := " "
	if resp.GetMember().GetIsLearner() {
		asLearner = " as learner "
	}
	fmt.Printf("Member %16x added%sto cluster %16x\n", resp.GetMember().GetID(), asLearner, resp.GetHeader().GetClusterId())
}

func (s *simplePrinter) MemberRemove(id uint64, r *v3.MemberRemoveResponse) {
	fmt.Printf("Member %16x removed from cluster %16x\n", id, (*pb.MemberRemoveResponse)(r).GetHeader().GetClusterId())
}

func (s *simplePrinter) MemberUpdate(id uint64, r *v3.MemberUpdateResponse) {
	fmt.Printf("Member %16x updated in cluster %16x\n", id, (*pb.MemberUpdateResponse)(r).GetHeader().GetClusterId())
}

func (s *simplePrinter) MemberPromote(id uint64, r *v3.MemberPromoteResponse) {
	fmt.Printf("Member %16x promoted in cluster %16x\n", id, (*pb.MemberPromoteResponse)(r).GetHeader().GetClusterId())
}

func (s *simplePrinter) MemberList(resp *v3.MemberListResponse) {
	_, rows := makeMemberListTable(resp)
	for _, row := range rows {
		fmt.Println(strings.Join(row, ", "))
	}
}

func (s *simplePrinter) EndpointHealth(hs []epHealth) {
	for _, h := range hs {
		if h.Error == "" {
			fmt.Printf("%s is healthy: successfully committed proposal: took = %v\n", h.Ep, h.Took)
		} else {
			fmt.Fprintf(os.Stderr, "%s is unhealthy: failed to commit proposal: %v\n", h.Ep, h.Error)
		}
	}
}

func (s *simplePrinter) EndpointStatus(statusList []epStatus) {
	_, rows := makeEndpointStatusTable(statusList)
	for _, row := range rows {
		fmt.Println(strings.Join(row, ", "))
	}
}

func (s *simplePrinter) EndpointHashKV(hashList []epHashKV) {
	_, rows := makeEndpointHashKVTable(hashList)
	for _, row := range rows {
		fmt.Println(strings.Join(row, ", "))
	}
}

func (s *simplePrinter) MoveLeader(leader, target uint64, r *v3.MoveLeaderResponse) {
	fmt.Printf("Leadership transferred from %s to %s\n", types.ID(leader), types.ID(target))
}

func (s *simplePrinter) DowngradeValidate(r *v3.DowngradeResponse) {
	fmt.Printf("Downgrade validate success, cluster version %s\n", (*pb.DowngradeResponse)(r).GetVersion())
}

func (s *simplePrinter) DowngradeEnable(r *v3.DowngradeResponse) {
	fmt.Printf("Downgrade enable success, cluster version %s\n", (*pb.DowngradeResponse)(r).GetVersion())
}

func (s *simplePrinter) DowngradeCancel(r *v3.DowngradeResponse) {
	fmt.Printf("Downgrade cancel success, cluster version %s\n", (*pb.DowngradeResponse)(r).GetVersion())
}

func (s *simplePrinter) RoleAdd(role string, r *v3.AuthRoleAddResponse) {
	fmt.Printf("Role %s created\n", role)
}

func (s *simplePrinter) RoleGet(role string, r *v3.AuthRoleGetResponse) {
	resp := (*pb.AuthRoleGetResponse)(r)
	fmt.Printf("Role %s\n", role)
	if rootRole == role && resp.GetPerm() == nil {
		fmt.Println("KV Read:")
		fmt.Println("\t[, <open ended>")
		fmt.Println("KV Write:")
		fmt.Println("\t[, <open ended>")
		return
	}

	fmt.Println("KV Read:")

	printRange := func(perm *authpb.Permission) {
		sKey := string(perm.GetKey())
		sRangeEnd := string(perm.GetRangeEnd())
		if sRangeEnd != "\x00" {
			fmt.Printf("\t[%s, %s)", sKey, sRangeEnd)
		} else {
			fmt.Printf("\t[%s, <open ended>", sKey)
		}
		if v3.GetPrefixRangeEnd(sKey) == sRangeEnd && len(sKey) > 0 {
			fmt.Printf(" (prefix %s)", sKey)
		}
		fmt.Print("\n")
	}

	for _, perm := range resp.GetPerm() {
		if perm.GetPermType() == v3.PermRead || perm.GetPermType() == v3.PermReadWrite {
			if len(perm.GetRangeEnd()) == 0 {
				fmt.Printf("\t%s\n", perm.GetKey())
			} else {
				printRange(perm)
			}
		}
	}
	fmt.Println("KV Write:")
	for _, perm := range resp.GetPerm() {
		if perm.GetPermType() == v3.PermWrite || perm.GetPermType() == v3.PermReadWrite {
			if len(perm.GetRangeEnd()) == 0 {
				fmt.Printf("\t%s\n", perm.GetKey())
			} else {
				printRange(perm)
			}
		}
	}
}

func (s *simplePrinter) RoleList(r *v3.AuthRoleListResponse) {
	for _, role := range (*pb.AuthRoleListResponse)(r).GetRoles() {
		fmt.Printf("%s\n", role)
	}
}

func (s *simplePrinter) RoleDelete(role string, r *v3.AuthRoleDeleteResponse) {
	fmt.Printf("Role %s deleted\n", role)
}

func (s *simplePrinter) RoleGrantPermission(role string, r *v3.AuthRoleGrantPermissionResponse) {
	fmt.Printf("Role %s updated\n", role)
}

func (s *simplePrinter) RoleRevokePermission(role string, key string, end string, r *v3.AuthRoleRevokePermissionResponse) {
	if len(end) == 0 {
		fmt.Printf("Permission of key %s is revoked from role %s\n", key, role)
		return
	}
	if end != "\x00" {
		fmt.Printf("Permission of range [%s, %s) is revoked from role %s\n", key, end, role)
	} else {
		fmt.Printf("Permission of range [%s, <open ended> is revoked from role %s\n", key, role)
	}
}

func (s *simplePrinter) UserAdd(name string, r *v3.AuthUserAddResponse) {
	fmt.Printf("User %s created\n", name)
}

func (s *simplePrinter) UserGet(name string, r *v3.AuthUserGetResponse) {
	fmt.Printf("User: %s\n", name)
	fmt.Print("Roles:")
	for _, role := range (*pb.AuthUserGetResponse)(r).GetRoles() {
		fmt.Printf(" %s", role)
	}
	fmt.Print("\n")
}

func (s *simplePrinter) UserChangePassword(*v3.AuthUserChangePasswordResponse) {
	fmt.Println("Password updated")
}

func (s *simplePrinter) UserGrantRole(user string, role string, r *v3.AuthUserGrantRoleResponse) {
	fmt.Printf("Role %s is granted to user %s\n", role, user)
}

func (s *simplePrinter) UserRevokeRole(user string, role string, r *v3.AuthUserRevokeRoleResponse) {
	fmt.Printf("Role %s is revoked from user %s\n", role, user)
}

func (s *simplePrinter) UserDelete(user string, r *v3.AuthUserDeleteResponse) {
	fmt.Printf("User %s deleted\n", user)
}

func (s *simplePrinter) UserList(r *v3.AuthUserListResponse) {
	for _, user := range (*pb.AuthUserListResponse)(r).GetUsers() {
		fmt.Printf("%s\n", user)
	}
}

func (s *simplePrinter) AuthStatus(r *v3.AuthStatusResponse) {
	resp := (*pb.AuthStatusResponse)(r)
	fmt.Println("Authentication Status:", resp.GetEnabled())
	fmt.Println("AuthRevision:", resp.GetAuthRevision())
}
