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
	"io"
	"os"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type jsonPrinter struct {
	writer io.Writer
	isHex  bool
	printer
}

type (
	HexResponseHeader pb.ResponseHeader
	HexMember         pb.Member
)

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

func newJSONPrinter(isHex bool) printer {
	return &jsonPrinter{
		writer:  os.Stdout,
		isHex:   isHex,
		printer: &printerRPC{newPrinterUnsupported("json"), printJSON},
	}
}

func (p *jsonPrinter) EndpointHealth(r []epHealth) { printJSON(r) }
func (p *jsonPrinter) EndpointStatus(r []epStatus) { printJSON(r) }
func (p *jsonPrinter) EndpointHashKV(r []epHashKV) { printJSON(r) }

func (p *jsonPrinter) MemberAdd(r clientv3.MemberAddResponse)                   { p.printJSON(r) }
func (p *jsonPrinter) MemberRemove(_ uint64, r clientv3.MemberRemoveResponse)   { p.printJSON(r) }
func (p *jsonPrinter) MemberUpdate(_ uint64, r clientv3.MemberUpdateResponse)   { p.printJSON(r) }
func (p *jsonPrinter) MemberPromote(_ uint64, r clientv3.MemberPromoteResponse) { p.printJSON(r) }
func (p *jsonPrinter) MemberList(r clientv3.MemberListResponse)                 { p.printJSON(r) }

func printJSONTo(w io.Writer, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Fprintln(w, string(b))
}

func printJSON(v any) {
	printJSONTo(os.Stdout, v)
}

func (p *jsonPrinter) printJSON(v any) {
	var data any
	if !p.isHex {
		printJSONTo(p.writer, v)
		return
	}

	switch r := v.(type) {
	case clientv3.MemberAddResponse:
		type Alias clientv3.MemberAddResponse

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
	case clientv3.MemberRemoveResponse:
		type Alias clientv3.MemberRemoveResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case clientv3.MemberUpdateResponse:
		type Alias clientv3.MemberUpdateResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case clientv3.MemberPromoteResponse:
		type Alias clientv3.MemberPromoteResponse

		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
			*Alias
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
			Alias:   (*Alias)(&r),
		}
	case clientv3.MemberListResponse:
		type Alias clientv3.MemberListResponse

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

	printJSONTo(p.writer, data)
}

func toHexMembers(members []*pb.Member) []*HexMember {
	hexMembers := make([]*HexMember, len(members))
	for i, member := range members {
		hexMembers[i] = (*HexMember)(member)
	}
	return hexMembers
}
