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

	v3 "go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/clientv3/snapshot"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	spb "go.etcd.io/etcd/v3/mvcc/mvccpb"
)

type fieldsPrinter struct{ printer }

func (p *fieldsPrinter) kv(pfx string, kv *spb.KeyValue) {
	fmt.Printf("\"%sKey\" : %q\n", pfx, string(kv.Key))
	fmt.Printf("\"%sCreateRevision\" : %d\n", pfx, kv.CreateRevision)
	fmt.Printf("\"%sModRevision\" : %d\n", pfx, kv.ModRevision)
	fmt.Printf("\"%sVersion\" : %d\n", pfx, kv.Version)
	fmt.Printf("\"%sValue\" : %q\n", pfx, string(kv.Value))
	fmt.Printf("\"%sLease\" : %d\n", pfx, kv.Lease)
}

func (p *fieldsPrinter) hdr(h *pb.ResponseHeader) {
	fmt.Println(`"ClusterID" :`, h.ClusterId)
	fmt.Println(`"MemberID" :`, h.MemberId)
	fmt.Println(`"Revision" :`, h.Revision)
	fmt.Println(`"RaftTerm" :`, h.RaftTerm)
}

func (p *fieldsPrinter) Del(r v3.DeleteResponse) {
	p.hdr(r.Header)
	fmt.Println(`"Deleted" :`, r.Deleted)
	for _, kv := range r.PrevKvs {
		p.kv("Prev", kv)
	}
}

func (p *fieldsPrinter) Get(r v3.GetResponse) {
	p.hdr(r.Header)
	for _, kv := range r.Kvs {
		p.kv("", kv)
	}
	fmt.Println(`"More" :`, r.More)
	fmt.Println(`"Count" :`, r.Count)
}

func (p *fieldsPrinter) Put(r v3.PutResponse) {
	p.hdr(r.Header)
	if r.PrevKv != nil {
		p.kv("Prev", r.PrevKv)
	}
}

func (p *fieldsPrinter) Txn(r v3.TxnResponse) {
	p.hdr(r.Header)
	fmt.Println(`"Succeeded" :`, r.Succeeded)
	for _, resp := range r.Responses {
		switch v := resp.Response.(type) {
		case *pb.ResponseOp_ResponseDeleteRange:
			p.Del((v3.DeleteResponse)(*v.ResponseDeleteRange))
		case *pb.ResponseOp_ResponsePut:
			p.Put((v3.PutResponse)(*v.ResponsePut))
		case *pb.ResponseOp_ResponseRange:
			p.Get((v3.GetResponse)(*v.ResponseRange))
		default:
			fmt.Printf("\"Unknown\" : %q\n", fmt.Sprintf("%+v", v))
		}
	}
}

func (p *fieldsPrinter) Watch(resp v3.WatchResponse) {
	p.hdr(&resp.Header)
	for _, e := range resp.Events {
		fmt.Println(`"Type" :`, e.Type)
		if e.PrevKv != nil {
			p.kv("Prev", e.PrevKv)
		}
		p.kv("", e.Kv)
	}
}

func (p *fieldsPrinter) Grant(r v3.LeaseGrantResponse) {
	p.hdr(r.ResponseHeader)
	fmt.Println(`"ID" :`, r.ID)
	fmt.Println(`"TTL" :`, r.TTL)
}

func (p *fieldsPrinter) Revoke(id v3.LeaseID, r v3.LeaseRevokeResponse) {
	p.hdr(r.Header)
}

func (p *fieldsPrinter) KeepAlive(r v3.LeaseKeepAliveResponse) {
	p.hdr(r.ResponseHeader)
	fmt.Println(`"ID" :`, r.ID)
	fmt.Println(`"TTL" :`, r.TTL)
}

func (p *fieldsPrinter) TimeToLive(r v3.LeaseTimeToLiveResponse, keys bool) {
	p.hdr(r.ResponseHeader)
	fmt.Println(`"ID" :`, r.ID)
	fmt.Println(`"TTL" :`, r.TTL)
	fmt.Println(`"GrantedTTL" :`, r.GrantedTTL)
	for _, k := range r.Keys {
		fmt.Printf("\"Key\" : %q\n", string(k))
	}
}

func (p *fieldsPrinter) Leases(r v3.LeaseLeasesResponse) {
	p.hdr(r.ResponseHeader)
	for _, item := range r.Leases {
		fmt.Println(`"ID" :`, item.ID)
	}
}

func (p *fieldsPrinter) MemberList(r v3.MemberListResponse) {
	p.hdr(r.Header)
	for _, m := range r.Members {
		fmt.Println(`"ID" :`, m.ID)
		fmt.Printf("\"Name\" : %q\n", m.Name)
		for _, u := range m.PeerURLs {
			fmt.Printf("\"PeerURL\" : %q\n", u)
		}
		for _, u := range m.ClientURLs {
			fmt.Printf("\"ClientURL\" : %q\n", u)
		}
		fmt.Println(`"IsLearner" :`, m.IsLearner)
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
		p.hdr(ep.Resp.Header)
		fmt.Printf("\"Version\" : %q\n", ep.Resp.Version)
		fmt.Println(`"DBSize" :`, ep.Resp.DbSize)
		fmt.Println(`"Leader" :`, ep.Resp.Leader)
		fmt.Println(`"IsLearner" :`, ep.Resp.IsLearner)
		fmt.Println(`"RaftIndex" :`, ep.Resp.RaftIndex)
		fmt.Println(`"RaftTerm" :`, ep.Resp.RaftTerm)
		fmt.Println(`"RaftAppliedIndex" :`, ep.Resp.RaftAppliedIndex)
		fmt.Println(`"Errors" :`, ep.Resp.Errors)
		fmt.Printf("\"Endpoint\" : %q\n", ep.Ep)
		fmt.Println()
	}
}

func (p *fieldsPrinter) EndpointHashKV(hs []epHashKV) {
	for _, h := range hs {
		p.hdr(h.Resp.Header)
		fmt.Printf("\"Endpoint\" : %q\n", h.Ep)
		fmt.Println(`"Hash" :`, h.Resp.Hash)
		fmt.Println()
	}
}

func (p *fieldsPrinter) Alarm(r v3.AlarmResponse) {
	p.hdr(r.Header)
	for _, a := range r.Alarms {
		fmt.Println(`"MemberID" :`, a.MemberID)
		fmt.Println(`"AlarmType" :`, a.Alarm)
		fmt.Println()
	}
}

func (p *fieldsPrinter) DBStatus(r snapshot.Status) {
	fmt.Println(`"Hash" :`, r.Hash)
	fmt.Println(`"Revision" :`, r.Revision)
	fmt.Println(`"Keys" :`, r.TotalKey)
	fmt.Println(`"Size" :`, r.TotalSize)
}

func (p *fieldsPrinter) RoleAdd(role string, r v3.AuthRoleAddResponse) { p.hdr(r.Header) }
func (p *fieldsPrinter) RoleGet(role string, r v3.AuthRoleGetResponse) {
	p.hdr(r.Header)
	for _, p := range r.Perm {
		fmt.Println(`"PermType" : `, p.PermType.String())
		fmt.Printf("\"Key\" : %q\n", string(p.Key))
		fmt.Printf("\"RangeEnd\" : %q\n", string(p.RangeEnd))
	}
}
func (p *fieldsPrinter) RoleDelete(role string, r v3.AuthRoleDeleteResponse) { p.hdr(r.Header) }
func (p *fieldsPrinter) RoleList(r v3.AuthRoleListResponse) {
	p.hdr(r.Header)
	fmt.Printf(`"Roles" :`)
	for _, r := range r.Roles {
		fmt.Printf(" %q", r)
	}
	fmt.Println()
}
func (p *fieldsPrinter) RoleGrantPermission(role string, r v3.AuthRoleGrantPermissionResponse) {
	p.hdr(r.Header)
}
func (p *fieldsPrinter) RoleRevokePermission(role string, key string, end string, r v3.AuthRoleRevokePermissionResponse) {
	p.hdr(r.Header)
}
func (p *fieldsPrinter) UserAdd(user string, r v3.AuthUserAddResponse)          { p.hdr(r.Header) }
func (p *fieldsPrinter) UserChangePassword(r v3.AuthUserChangePasswordResponse) { p.hdr(r.Header) }
func (p *fieldsPrinter) UserGrantRole(user string, role string, r v3.AuthUserGrantRoleResponse) {
	p.hdr(r.Header)
}
func (p *fieldsPrinter) UserRevokeRole(user string, role string, r v3.AuthUserRevokeRoleResponse) {
	p.hdr(r.Header)
}
func (p *fieldsPrinter) UserDelete(user string, r v3.AuthUserDeleteResponse) { p.hdr(r.Header) }
