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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/clientv3/snapshot"
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

func (p *jsonPrinter) EndpointHealth(r []epHealth) { printJSON(r) }
func (p *jsonPrinter) EndpointHashKV(r []epHashKV) { printJSON(r) }
func (p *jsonPrinter) EndpointStatus(r []epStatus) {
	if p.isHex {
		printEndpointStatusWithHexJSON(r)
		} else {
		printJSON(r)
	}
}

func (p *jsonPrinter) DBStatus(r snapshot.Status)  { printJSON(r) }

func (p *jsonPrinter) MemberList(r clientv3.MemberListResponse) {
	if p.isHex {
		printMemberListWithHexJSON(r)
	} else {
		printJSON(r)
	}
}

func printJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Println(string(b))
}

func printEndpointStatusWithHexJSON(r []epStatus) {
	var buffer bytes.Buffer
	var b []byte

	buffer.WriteString("[")
	for i := 0; i < len(r); i++ {
		if i != 0 {
			buffer.WriteString(",")
		}

		buffer.WriteString("{\"Endpoint\":\"" + r[i].Ep  + "\",")
		buffer.WriteString("\"Status\":{ \"header\":{\"cluster_id\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.Header.ClusterId, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"member_id\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.Header.MemberId, 16)
		buffer.Write(b)

		buffer.WriteString("\",\"revision\":\"")
		b = strconv.AppendInt(nil, r[i].Resp.Header.Revision, 16)
		buffer.Write(b)

		buffer.WriteString("\",\"raft_term\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.Header.RaftTerm, 16)
		buffer.Write(b)
		buffer.WriteString("\"}")

		buffer.WriteString(",\"version\":\"" + r[i].Resp.Version  + "\"")
		buffer.WriteString(",\"dbSize\":\"")
		b = strconv.AppendInt(nil, r[i].Resp.DbSize, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"leader\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.Leader, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"raftIndex\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.RaftIndex, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"raftTerm\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.RaftTerm, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"raftAppliedIndex\":\"")
		b = strconv.AppendUint(nil, r[i].Resp.RaftAppliedIndex, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"dbSizeInUse\":\"")
		b = strconv.AppendInt(nil, r[i].Resp.DbSizeInUse, 16)
		buffer.Write(b)

		buffer.WriteString("\"}}")
	}

	buffer.WriteString("]")
	fmt.Println(string(buffer.Bytes()))
}

func printMemberListWithHexJSON(r clientv3.MemberListResponse) {
	var buffer bytes.Buffer
	var b []byte
	buffer.WriteString("{\"header\":{\"cluster_id\":\"")
	b = strconv.AppendUint(nil, r.Header.ClusterId, 16)
	buffer.Write(b)
	buffer.WriteString("\",\"member_id\":\"")
	b = strconv.AppendUint(nil, r.Header.MemberId, 16)
	buffer.Write(b)
	buffer.WriteString("\",\"raft_term\":\"")
	b = strconv.AppendUint(nil, r.Header.RaftTerm, 16)
	buffer.Write(b)
	buffer.WriteString("\"}")
	for i := 0; i < len(r.Members); i++ {
		if i == 0 {
			buffer.WriteString(",\"members\":[{\"ID\":\"")
		} else {
			buffer.WriteString(",{\"ID\":\"")
		}
		b = strconv.AppendUint(nil, r.Members[i].ID, 16)
		buffer.Write(b)
		buffer.WriteString("\",\"name\":\"" + r.Members[i].Name + "\"," + "\"peerURLs\":")
		b, err := json.Marshal(r.Members[i].PeerURLs)
		if err != nil {
			return
		}
		buffer.Write(b)
		buffer.WriteString(",\"clientURLS\":")
		b, err = json.Marshal(r.Members[i].ClientURLs)
		if err != nil {
			return
		}
		buffer.Write(b)
		buffer.WriteByte('}')
		if i == len(r.Members)-1 {
			buffer.WriteString("]")
		}
	}
	buffer.WriteString("}")
	fmt.Println(string(buffer.Bytes()))

}
