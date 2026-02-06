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
	return json.Marshal(&struct {
		ClusterID string `json:"cluster_id"`
		MemberID  string `json:"member_id"`
		Revision  int64  `json:"revision,omitempty"`
		RaftTerm  uint64 `json:"raft_term,omitempty"`
	}{
		ClusterID: fmt.Sprintf("%x", h.ClusterId),
		MemberID:  fmt.Sprintf("%x", h.MemberId),
		Revision:  h.Revision,
		RaftTerm:  h.RaftTerm,
	})
}

func (m *HexMember) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID         string   `json:"ID"`
		Name       string   `json:"name,omitempty"`
		PeerURLs   []string `json:"peerURLs,omitempty"`
		ClientURLs []string `json:"clientURLs,omitempty"`
		IsLearner  bool     `json:"isLearner,omitempty"`
	}{
		ID:         fmt.Sprintf("%x", m.ID),
		Name:       m.Name,
		PeerURLs:   m.PeerURLs,
		ClientURLs: m.ClientURLs,
		IsLearner:  m.IsLearner,
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

func (p *jsonPrinter) MemberAdd(r *clientv3.MemberAddResponse)                   { p.printJSON(r) }
func (p *jsonPrinter) MemberRemove(_ uint64, r *clientv3.MemberRemoveResponse)   { p.printJSON(r) }
func (p *jsonPrinter) MemberUpdate(_ uint64, r *clientv3.MemberUpdateResponse)   { p.printJSON(r) }
func (p *jsonPrinter) MemberPromote(_ uint64, r *clientv3.MemberPromoteResponse) { p.printJSON(r) }
func (p *jsonPrinter) MemberList(r *clientv3.MemberListResponse)                 { p.printJSON(r) }

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

type TxnResponseJSON struct {
	Header    *pb.ResponseHeader `json:"header,omitempty"`
	Succeeded bool               `json:"succeeded"`
	Responses []ResponseOpJSON   `json:"responses,omitempty"`
}
type ResponseOpJSON struct {
	Response ResponseOpResponseJSON `json:"Response"`
}
type ResponseOpResponseJSON struct {
	ResponseRange       *pb.RangeResponse       `json:"response_range,omitempty"`
	ResponsePut         *pb.PutResponse         `json:"response_put,omitempty"`
	ResponseDeleteRange *pb.DeleteRangeResponse `json:"response_delete_range,omitempty"`
	ResponseTxn         *pb.TxnResponse         `json:"response_txn,omitempty"`
}

func (t *TxnResponseJSON) ToProto() *pb.TxnResponse {
	r := &pb.TxnResponse{
		Header:    t.Header,
		Succeeded: t.Succeeded,
	}
	for _, jsonResponse := range t.Responses {
		switch {
		case jsonResponse.Response.ResponseDeleteRange != nil:
			r.Responses = append(r.Responses, &pb.ResponseOp{Response: &pb.ResponseOp_ResponseDeleteRange{ResponseDeleteRange: jsonResponse.Response.ResponseDeleteRange}})
		case jsonResponse.Response.ResponseRange != nil:
			r.Responses = append(r.Responses, &pb.ResponseOp{Response: &pb.ResponseOp_ResponseRange{ResponseRange: jsonResponse.Response.ResponseRange}})
		case jsonResponse.Response.ResponseTxn != nil:
			r.Responses = append(r.Responses, &pb.ResponseOp{Response: &pb.ResponseOp_ResponseTxn{ResponseTxn: jsonResponse.Response.ResponseTxn}})
		case jsonResponse.Response.ResponsePut != nil:
			r.Responses = append(r.Responses, &pb.ResponseOp{Response: &pb.ResponseOp_ResponsePut{ResponsePut: jsonResponse.Response.ResponsePut}})
		default:
			// unknown
			r.Responses = append(r.Responses, &pb.ResponseOp{})
		}
	}
	return r
}
func TxnResponseJSONFromProto(protoResponse *pb.TxnResponse) TxnResponseJSON {
	r := TxnResponseJSON{
		Header:    protoResponse.GetHeader(),
		Succeeded: protoResponse.GetSucceeded(),
	}
	for _, response := range protoResponse.GetResponses() {
		switch response := response.Response.(type) {
		case *pb.ResponseOp_ResponseRange:
			r.Responses = append(r.Responses, ResponseOpJSON{ResponseOpResponseJSON{ResponseRange: response.ResponseRange}})
		case *pb.ResponseOp_ResponsePut:
			r.Responses = append(r.Responses, ResponseOpJSON{ResponseOpResponseJSON{ResponsePut: response.ResponsePut}})
		case *pb.ResponseOp_ResponseDeleteRange:
			r.Responses = append(r.Responses, ResponseOpJSON{ResponseOpResponseJSON{ResponseDeleteRange: response.ResponseDeleteRange}})
		case *pb.ResponseOp_ResponseTxn:
			r.Responses = append(r.Responses, ResponseOpJSON{ResponseOpResponseJSON{ResponseTxn: response.ResponseTxn}})
		default:
			r.Responses = append(r.Responses, ResponseOpJSON{ResponseOpResponseJSON{}})
		}
	}
	return r
}

func (p *jsonPrinter) printJSON(v any) {
	var data any
	if !p.isHex {
		printJSONTo(p.writer, v)
		return
	}

	switch r := v.(type) {
	case *clientv3.TxnResponse:
		data = TxnResponseJSONFromProto((*pb.TxnResponse)(r))
	case *clientv3.MemberAddResponse:
		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Member  *HexMember         `json:"member"`
			Members []*HexMember       `json:"members"`
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Member:  (*HexMember)(r.Member),
			Members: toHexMembers(r.Members),
		}
	case *clientv3.MemberRemoveResponse:
		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
		}
	case *clientv3.MemberUpdateResponse:
		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
		}
	case *clientv3.MemberPromoteResponse:
		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
		}
	case *clientv3.MemberListResponse:
		data = &struct {
			Header  *HexResponseHeader `json:"header"`
			Members []*HexMember       `json:"members"`
		}{
			Header:  (*HexResponseHeader)(r.Header),
			Members: toHexMembers(r.Members),
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
