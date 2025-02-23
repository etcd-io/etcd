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

	clientv3 "go.etcd.io/etcd/client/v3"
	"slices"
)

type jsonPrinter struct {
	isHex bool
	printer
}

var hexFields = []string{"cluster_id", "member_id", "ID", "leader"}

func newJSONPrinter(isHex bool) printer {
	return &jsonPrinter{
		isHex:   isHex,
		printer: &printerRPC{newPrinterUnsupported("json"), printJSON},
	}
}

func (p *jsonPrinter) EndpointHealth(r []epHealth) { printJSON(r) }
func (p *jsonPrinter) EndpointStatus(r []epStatus) {
	if p.isHex {
		printWithHexJSON(r)
	} else {
		printJSON(r)
	}
}
func (p *jsonPrinter) EndpointHashKV(r []epHashKV) {
	if p.isHex {
		printWithHexJSON(r)
	} else {
		printJSON(r)
	}
}

func (p *jsonPrinter) MemberAdd(r clientv3.MemberAddResponse) {
	if p.isHex {
		printWithHexJSON(r)
	} else {
		printJSON(r)
	}
}

func (p *jsonPrinter) MemberList(r clientv3.MemberListResponse) {
	if p.isHex {
		printWithHexJSON(r)
	} else {
		printJSON(r)
	}
}

func printJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Println(string(b))
}

func convertFieldsToHex(data interface{}, keysToConvert []string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if contains(keysToConvert, key) {
				if num, ok := val.(float64); ok {
					v[key] = fmt.Sprintf("%x", uint64(num))
				}
			}
			convertFieldsToHex(val, keysToConvert)
		}
	case []interface{}:
		for _, item := range v {
			convertFieldsToHex(item, keysToConvert)
		}
	}
}

func contains(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

func printWithHexJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	var data interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	convertFieldsToHex(data, hexFields)

	b, err = json.Marshal(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	fmt.Println(string(b))
}
