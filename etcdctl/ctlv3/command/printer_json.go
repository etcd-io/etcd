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
	"encoding/json"
	"fmt"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	v3 "go.etcd.io/etcd/client/v3"
)

type jsonPrinter struct {
	isHex bool
	printer
}

func newJSONPrinter(isHex bool) printer {
	return &jsonPrinter{
		isHex:   isHex,
		printer: &printerRPC{newPrinterUnsupported("json"), printJSON},
	}
}

type (
	HexStatusResponse pb.StatusResponse
	HexHashKVResponse pb.HashKVResponse
	HexResponseHeader pb.ResponseHeader
	HexMember         pb.Member
)

func (sr *HexStatusResponse) MarshalJSON() ([]byte, error) {
	type Alias pb.StatusResponse

	return json.Marshal(&struct {
		Header HexResponseHeader `json:"header"`
		Leader string            `json:"leader"`
		Alias
	}{
		Header: (HexResponseHeader)(*sr.Header),
		Leader: fmt.Sprintf("%x", sr.Leader),
		Alias:  (Alias)(*sr),
	})
}

func (hr *HexHashKVResponse) MarshalJSON() ([]byte, error) {
	type Alias pb.HashKVResponse

	return json.Marshal(&struct {
		Header HexResponseHeader `json:"header"`
		Alias
	}{
		Header: (HexResponseHeader)(*hr.Header),
		Alias:  (Alias)(*hr),
	})
}

func (h *HexResponseHeader) MarshalJSON() ([]byte, error) {
	type Alias pb.ResponseHeader

	return json.Marshal(&struct {
		ClusterID string `json:"cluster_id"`
		MemberID  string `json:"member_id"`
		Alias
	}{
		ClusterID: fmt.Sprintf("%x", h.ClusterId),
		MemberID:  fmt.Sprintf("%x", h.MemberId),
		Alias:     (Alias)(*h),
	})
}

func (m *HexMember) MarshalJSON() ([]byte, error) {
	type Alias pb.Member

	return json.Marshal(&struct {
		ID string `json:"ID"`
		Alias
	}{
		ID:    fmt.Sprintf("%x", m.ID),
		Alias: (Alias)(*m),
	})
}

func (p *jsonPrinter) EndpointHealth(r []epHealth) { p.printJSON(r) }
func (p *jsonPrinter) EndpointStatus(r []epStatus) { p.printJSON(r) }
func (p *jsonPrinter) EndpointHashKV(r []epHashKV) { p.printJSON(r) }

func (p *jsonPrinter) MemberAdd(r v3.MemberAddResponse)                    { p.printJSON(r) }
func (p *jsonPrinter) MemberList(r v3.MemberListResponse)                  { p.printJSON(r) }
func (p *jsonPrinter) MemberPromote(id uint64, r v3.MemberPromoteResponse) { p.printJSON(r) }
func (p *jsonPrinter) MemberRemove(id uint64, r v3.MemberRemoveResponse)   { p.printJSON(r) }
func (p *jsonPrinter) MemberUpdate(id uint64, r v3.MemberUpdateResponse)   { p.printJSON(r) }

func printJSON(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}

func (p *jsonPrinter) printJSON(v any) {
	var data any
	if !p.isHex {
		printJSON(v)
		return
	}

	switch r := v.(type) {
	case []epStatus:
		type Alias epStatus

		data = make([]any, len(r))
		for i, status := range r {
			data.([]any)[i] = &struct {
				Resp *HexStatusResponse `json:"Status"`
				*Alias
			}{
				Resp:  (*HexStatusResponse)(status.Resp),
				Alias: (*Alias)(&status),
			}
		}
	case []epHashKV:
		type Alias epHashKV

		data = make([]any, len(r))
		for i, status := range r {
			data.([]any)[i] = &struct {
				Resp *HexHashKVResponse `json:"HashKV"`
				*Alias
			}{
				Resp:  (*HexHashKVResponse)(status.Resp),
				Alias: (*Alias)(&status),
			}
		}
	case v3.MemberAddResponse:
		type Alias v3.MemberAddResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Member  *HexMember         `json:"member"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Member:  (*HexMember)(r.Member),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case v3.MemberListResponse:
		type Alias v3.MemberListResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case v3.MemberPromoteResponse:
		type Alias v3.MemberPromoteResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case v3.MemberRemoveResponse:
		type Alias v3.MemberRemoveResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case v3.MemberUpdateResponse:
		type Alias v3.MemberUpdateResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	default:
		data = v
	}

	printJSON(data)
}

func toHexMembers(members []*pb.Member) []*HexMember {
	hexMembers := make([]*HexMember, len(members))
	for i, member := range members {
		hexMembers[i] = (*HexMember)(member)
	}
	return hexMembers
}
