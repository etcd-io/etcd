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
	"os"
	"slices"

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

func (p *jsonPrinter) EndpointHealth(r []epHealth) { p.printJSON(r) }
func (p *jsonPrinter) EndpointStatus(r []epStatus) { p.printJSON(r) }
func (p *jsonPrinter) EndpointHashKV(r []epHashKV) { p.printJSON(r) }

func (p *jsonPrinter) MemberAdd(r v3.MemberAddResponse)   { p.printJSON(r) }
func (p *jsonPrinter) MemberList(r v3.MemberListResponse) { p.printJSON(r) }

func printJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Println(string(b))
}

func (p *jsonPrinter) printJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	if !p.isHex {
		fmt.Println(string(b))
		return
	}

	var data any
	if err = json.Unmarshal(b, &data); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	convertFieldsToHex(data)
	b, err = json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Println(string(b))
}

func convertFieldsToHex(data any) {
	keysToConvert := []string{"cluster_id", "member_id", "ID", "leader"}

	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			if slices.Contains(keysToConvert, key) {
				if num, ok := val.(float64); ok {
					v[key] = fmt.Sprintf("%x", uint64(num))
				}
			}
			convertFieldsToHex(val)
		}
	case []any:
		for _, item := range v {
			convertFieldsToHex(item)
		}
	}
}
