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
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"

	v3 "go.etcd.io/etcd/client/v3"
)

type tablePrinter struct{ printer }

func (tp *tablePrinter) MemberList(r v3.MemberListResponse) {
	hdr, rows := makeMemberListTable(r)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}

func (tp *tablePrinter) EndpointHealth(r []epHealth) {
	hdr, rows := makeEndpointHealthTable(r)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}

func (tp *tablePrinter) EndpointStatus(r []epStatus) {
	hdr, rows := makeEndpointStatusTable(r)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}

func (tp *tablePrinter) EndpointHashKV(r []epHashKV) {
	hdr, rows := makeEndpointHashKVTable(r)
	cfgBuilder := tablewriter.NewConfigBuilder().WithRowAlignment(tw.AlignRight)
	table := tablewriter.NewTable(os.Stdout, tablewriter.WithConfig(cfgBuilder.Build()))
	table.Header(hdr)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}
