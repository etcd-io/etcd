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
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

var OutputFormat string

type printer interface {
	DBStatus(snapshot.Status)
	DBHashKV(HashKV)
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
	p func(any)
}

type printerUnsupported struct{ printerRPC }

func newPrinterUnsupported(n string) printer {
	f := func(any) {
		cobrautl.ExitWithError(cobrautl.ExitBadFeature, errors.New(n+" not supported as output format"))
	}
	return &printerUnsupported{printerRPC{nil, f}}
}

func (p *printerUnsupported) DBStatus(snapshot.Status) { p.p(nil) }
func (p *printerUnsupported) DBHashKV(HashKV)          { p.p(nil) }

func makeDBStatusTable(ds snapshot.Status) (hdr []string, rows [][]string) {
	hdr = []string{"hash", "revision", "total keys", "total size", "version"}
	rows = append(rows, []string{
		strconv.FormatUint(uint64(ds.Hash), 16),
		strconv.FormatInt(ds.Revision, 10),
		strconv.Itoa(ds.TotalKey),
		humanize.Bytes(uint64(ds.TotalSize)),
		ds.Version,
	})
	return hdr, rows
}

func makeDBHashKVTable(ds HashKV) (hdr []string, rows [][]string) {
	hdr = []string{"hash", "hash revision", "compact revision"}
	rows = append(rows, []string{
		strconv.FormatUint(uint64(ds.Hash), 10),
		strconv.FormatInt(ds.HashRevision, 10),
		strconv.FormatInt(ds.CompactRevision, 10),
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
