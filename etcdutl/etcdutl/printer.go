// Copyright 2021 The etcd Authors
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

package etcdutl

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/cobrautl"

	"github.com/dustin/go-humanize"
)

var (
	OutputFormat string
)

type printer interface {
	DBStatus(snapshot.Status)
}

func NewPrinter(printerType string) printer {
	switch printerType {
	case "simple":
		return &simplePrinter{}
	case "fields":
		return &fieldsPrinter{newPrinterUnsupported("fields")}
	case "json":
		return newJSONPrinter()
	case "protobuf":
		return newPBPrinter()
	case "table":
		return &tablePrinter{newPrinterUnsupported("table")}
	}
	return nil
}

type printerRPC struct {
	printer
	p func(interface{})
}

type printerUnsupported struct{ printerRPC }

func newPrinterUnsupported(n string) printer {
	f := func(interface{}) {
		cobrautl.ExitWithError(cobrautl.ExitBadFeature, errors.New(n+" not supported as output format"))
	}
	return &printerUnsupported{printerRPC{nil, f}}
}

func (p *printerUnsupported) DBStatus(snapshot.Status) { p.p(nil) }

func makeDBStatusTable(ds snapshot.Status) (hdr []string, rows [][]string) {
	hdr = []string{"hash", "revision", "total keys", "total size"}
	rows = append(rows, []string{
		fmt.Sprintf("%x", ds.Hash),
		fmt.Sprint(ds.Revision),
		fmt.Sprint(ds.TotalKey),
		humanize.Bytes(uint64(ds.TotalSize)),
	})
	return hdr, rows
}

func initPrinterFromCmd(cmd *cobra.Command) (p printer) {
	outputType, err := cmd.Flags().GetString("write-out")
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	if p = NewPrinter(outputType); p == nil {
		cobrautl.ExitWithError(cobrautl.ExitBadFeature, errors.New("unsupported output format"))
	}
	return p
}
