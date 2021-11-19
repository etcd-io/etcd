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
	"fmt"
	"os"

	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

type pbPrinter struct{ printer }

type pbMarshal interface {
	Marshal() ([]byte, error)
}

func newPBPrinter() printer {
	return &pbPrinter{
		&printerRPC{newPrinterUnsupported("protobuf"), printPB},
	}
}

func printPB(v interface{}) {
	m, ok := v.(pbMarshal)
	if !ok {
		cobrautl.ExitWithError(cobrautl.ExitBadFeature, fmt.Errorf("marshal unsupported for type %T (%v)", v, v))
	}
	b, err := m.Marshal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Print(string(b))
}
