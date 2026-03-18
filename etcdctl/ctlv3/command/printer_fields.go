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

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	spb "go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	v3 "go.etcd.io/etcd/client/v3"
)

type fieldsPrinter struct {
	printer
	isHex bool
}

func (p *fieldsPrinter) kv(pfx string, kv *spb.KeyValue) {
	fmt.Printf("\"%sKey\" : %q\n", pfx, string(kv.GetKey()))
	fmt.Printf("\"%sCreateRevision\" : %d\n", pfx, kv.GetCreateRevision())
	fmt.Printf("\"%sModRevision\" : %d\n", pfx, kv.GetModRevision())
	fmt.Printf("\"%sVersion\" : %d\n", pfx, kv.GetVersion())
	fmt.Printf("\"%sValue\" : %q\n", pfx, string(kv.GetValue()))
	if p.isHex {
		fmt.Printf("\"%sLease\" : %016x\n", pfx, kv.GetLease())
	} else {
		fmt.Printf("\"%sLease\" : %d\n", pfx, kv.GetLease())
	}
}

func (p *fieldsPrinter) hdr(h *pb.ResponseHeader) {
	if p.isHex {
		fmt.Println(`"ClusterID" :`, types.ID(h.GetClusterId()))
		fmt.Println(`"MemberID" :`, types.ID(h.GetMemberId()))
	} else {
		fmt.Println(`"ClusterID" :`, h.GetClusterId())
		fmt.Println(`"MemberID" :`, h.GetMemberId())
	}
	// Revision only makes sense for k/v responses. For other kinds of
	// responses, i.e. MemberList, usually the revision isn't populated
	// at all; so it would be better to hide this field in these cases.
	if h.GetRevision() > 0 {
		fmt.Println(`"Revision" :`, h.GetRevision())
	}
	fmt.Println(`"RaftTerm" :`, h.GetRaftTerm())
}

func (p *fieldsPrinter) Del(r *v3.DeleteResponse) {
	resp := (*pb.DeleteRangeResponse)(r)
	p.hdr(resp.GetHeader())
	fmt.Println(`"Deleted" :`, resp.GetDeleted())
	for _, kv := range resp.GetPrevKvs() {
		p.kv("Prev", kv)
	}
}

func (p *fieldsPrinter) Get(r *v3.GetResponse) {
	resp := (*pb.RangeResponse)(r)
	p.hdr(resp.GetHeader())
	for _, kv := range resp.GetKvs() {
		p.kv("", kv)
	}
	fmt.Println(`"More" :`, resp.GetMore())
	fmt.Println(`"Count" :`, resp.GetCount())
}

func (p *fieldsPrinter) Put(r *v3.PutResponse) {
	resp := (*pb.PutResponse)(r)
	p.hdr(resp.GetHeader())
	if resp.GetPrevKv() != nil {
		p.kv("Prev", resp.GetPrevKv())
	}
}

func (p *fieldsPrinter) Txn(r *v3.TxnResponse) {
	resp := (*pb.TxnResponse)(r)
	p.hdr(resp.GetHeader())
	fmt.Println(`"Succeeded" :`, resp.GetSucceeded())
	for _, opResp := range resp.GetResponses() {
		switch v := opResp.GetResponse().(type) {
		case *pb.ResponseOp_ResponseDeleteRange:
			p.Del((*v3.DeleteResponse)(v.ResponseDeleteRange))
		case *pb.ResponseOp_ResponsePut:
			p.Put((*v3.PutResponse)(v.ResponsePut))
		case *pb.ResponseOp_ResponseRange:
			p.Get((*v3.GetResponse)(v.ResponseRange))
		default:
			fmt.Printf("\"Unknown\" : %q\n", fmt.Sprintf("%+v", v))
		}
	}
}

func (p *fieldsPrinter) Watch(resp *v3.WatchResponse) {
	if resp == nil {
		return
	}

	p.hdr(resp.Header)
	for _, ev := range resp.Events {
		fmt.Println(`"Type" :`, ev.GetType())
		if ev.GetPrevKv() != nil {
			p.kv("Prev", ev.GetPrevKv())
		}
		p.kv("", ev.GetKv())
	}
}

func (p *fieldsPrinter) Grant(r *v3.LeaseGrantResponse) {
	p.hdr(r.ResponseHeader)
	if p.isHex {
		fmt.Printf("\"ID\" : %016x\n", r.ID)
	} else {
		fmt.Println(`"ID" :`, r.ID)
	}
	fmt.Println(`"TTL" :`, r.TTL)
}

func (p *fieldsPrinter) Revoke(id v3.LeaseID, r *v3.LeaseRevokeResponse) {
	p.hdr((*pb.LeaseRevokeResponse)(r).GetHeader())
}

func (p *fieldsPrinter) KeepAlive(r *v3.LeaseKeepAliveResponse) {
	p.hdr(r.ResponseHeader)
	if p.isHex {
		fmt.Printf("\"ID\" : %016x\n", r.ID)
	} else {
		fmt.Println(`"ID" :`, r.ID)
	}
	fmt.Println(`"TTL" :`, r.TTL)
}

func (p *fieldsPrinter) TimeToLive(r *v3.LeaseTimeToLiveResponse, keys bool) {
	p.hdr(r.ResponseHeader)
	if p.isHex {
		fmt.Printf("\"ID\" : %016x\n", r.ID)
	} else {
		fmt.Println(`"ID" :`, r.ID)
	}
	fmt.Println(`"TTL" :`, r.TTL)
	fmt.Println(`"GrantedTTL" :`, r.GrantedTTL)
	for _, k := range r.Keys {
		fmt.Printf("\"Key\" : %q\n", string(k))
	}
}

func (p *fieldsPrinter) Leases(r *v3.LeaseLeasesResponse) {
	p.hdr(r.ResponseHeader)
	for _, item := range r.Leases {
		if p.isHex {
			fmt.Printf("\"ID\" : %016x\n", item.ID)
		} else {
			fmt.Println(`"ID" :`, item.ID)
		}
	}
}

func (p *fieldsPrinter) MemberList(r *v3.MemberListResponse) {
	resp := (*pb.MemberListResponse)(r)
	p.hdr(resp.GetHeader())
	for _, m := range resp.GetMembers() {
		if p.isHex {
			fmt.Println(`"ID" :`, types.ID(m.GetID()))
		} else {
			fmt.Println(`"ID" :`, m.GetID())
		}
		fmt.Printf("\"Name\" : %q\n", m.GetName())
		for _, u := range m.GetPeerURLs() {
			fmt.Printf("\"PeerURL\" : %q\n", u)
		}
		for _, u := range m.GetClientURLs() {
			fmt.Printf("\"ClientURL\" : %q\n", u)
		}
		fmt.Println(`"IsLearner" :`, m.GetIsLearner())
		fmt.Println()
	}
}

func (p *fieldsPrinter) EndpointHealth(hs []epHealth) {
	for _, h := range hs {
		fmt.Printf("\"Endpoint\" : %q\n", h.Ep)
		fmt.Println(`"Health" :`, h.Health)
		fmt.Println(`"Took" :`, h.Took)
		fmt.Println(`"Error" :`, h.Error)
		fmt.Println()
	}
}

func (p *fieldsPrinter) EndpointStatus(eps []epStatus) {
	for _, ep := range eps {
		resp := (*pb.StatusResponse)(ep.Resp)
		p.hdr(resp.GetHeader())
		fmt.Printf("\"Version\" : %q\n", resp.GetVersion())
		fmt.Printf("\"StorageVersion\" : %q\n", resp.GetStorageVersion())
		fmt.Println(`"DBSize" :`, resp.GetDbSize())
		fmt.Println(`"DBSizeInUse" :`, resp.GetDbSizeInUse())
		fmt.Println(`"DBSizeQuota" :`, resp.GetDbSizeQuota())
		fmt.Println(`"Leader" :`, resp.GetLeader())
		fmt.Println(`"IsLearner" :`, resp.GetIsLearner())
		fmt.Println(`"RaftIndex" :`, resp.GetRaftIndex())
		fmt.Println(`"RaftTerm" :`, resp.GetRaftTerm())
		fmt.Println(`"RaftAppliedIndex" :`, resp.GetRaftAppliedIndex())
		fmt.Println(`"Errors" :`, resp.GetErrors())
		fmt.Printf("\"Endpoint\" : %q\n", ep.Ep)
		fmt.Printf("\"DowngradeTargetVersion\" : %q\n", resp.GetDowngradeInfo().GetTargetVersion())
		fmt.Println(`"DowngradeEnabled" :`, resp.GetDowngradeInfo().GetEnabled())
		fmt.Println()
	}
}

func (p *fieldsPrinter) EndpointHashKV(hs []epHashKV) {
	for _, h := range hs {
		resp := (*pb.HashKVResponse)(h.Resp)
		p.hdr(resp.GetHeader())
		fmt.Printf("\"Endpoint\" : %q\n", h.Ep)
		fmt.Println(`"Hash" :`, resp.GetHash())
		fmt.Println(`"HashRevision" :`, resp.GetHashRevision())
		fmt.Println()
	}
}

func (p *fieldsPrinter) Alarm(r *v3.AlarmResponse) {
	resp := (*pb.AlarmResponse)(r)
	p.hdr(resp.GetHeader())
	for _, a := range resp.GetAlarms() {
		if p.isHex {
			fmt.Println(`"MemberID" :`, types.ID(a.GetMemberID()))
		} else {
			fmt.Println(`"MemberID" :`, a.GetMemberID())
		}
		fmt.Println(`"AlarmType" :`, a.GetAlarm())
		fmt.Println()
	}
}

func (p *fieldsPrinter) RoleAdd(role string, r *v3.AuthRoleAddResponse) {
	p.hdr((*pb.AuthRoleAddResponse)(r).GetHeader())
}

func (p *fieldsPrinter) RoleGet(role string, r *v3.AuthRoleGetResponse) {
	resp := (*pb.AuthRoleGetResponse)(r)
	p.hdr(resp.GetHeader())
	for _, perm := range resp.GetPerm() {
		fmt.Println(`"PermType" : `, perm.GetPermType().String())
		fmt.Printf("\"Key\" : %q\n", string(perm.GetKey()))
		fmt.Printf("\"RangeEnd\" : %q\n", string(perm.GetRangeEnd()))
	}
}

func (p *fieldsPrinter) RoleDelete(role string, r *v3.AuthRoleDeleteResponse) {
	p.hdr((*pb.AuthRoleDeleteResponse)(r).GetHeader())
}

func (p *fieldsPrinter) RoleList(r *v3.AuthRoleListResponse) {
	resp := (*pb.AuthRoleListResponse)(r)
	p.hdr(resp.GetHeader())
	fmt.Print(`"Roles" :`)
	for _, role := range resp.GetRoles() {
		fmt.Printf(" %q", role)
	}
	fmt.Println()
}

func (p *fieldsPrinter) RoleGrantPermission(role string, r *v3.AuthRoleGrantPermissionResponse) {
	p.hdr((*pb.AuthRoleGrantPermissionResponse)(r).GetHeader())
}

func (p *fieldsPrinter) RoleRevokePermission(role string, key string, end string, r *v3.AuthRoleRevokePermissionResponse) {
	p.hdr((*pb.AuthRoleRevokePermissionResponse)(r).GetHeader())
}

func (p *fieldsPrinter) UserAdd(user string, r *v3.AuthUserAddResponse) {
	p.hdr((*pb.AuthUserAddResponse)(r).GetHeader())
}

func (p *fieldsPrinter) UserChangePassword(r *v3.AuthUserChangePasswordResponse) {
	p.hdr((*pb.AuthUserChangePasswordResponse)(r).GetHeader())
}

func (p *fieldsPrinter) UserGrantRole(user string, role string, r *v3.AuthUserGrantRoleResponse) {
	p.hdr((*pb.AuthUserGrantRoleResponse)(r).GetHeader())
}

func (p *fieldsPrinter) UserRevokeRole(user string, role string, r *v3.AuthUserRevokeRoleResponse) {
	p.hdr((*pb.AuthUserRevokeRoleResponse)(r).GetHeader())
}

func (p *fieldsPrinter) UserDelete(user string, r *v3.AuthUserDeleteResponse) {
	p.hdr((*pb.AuthUserDeleteResponse)(r).GetHeader())
}
